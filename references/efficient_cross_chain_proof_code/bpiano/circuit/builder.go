// Package circuit provides a PLONK circuit builder that accumulates gates
// and builds a piano.CircuitInfo for use with the Piano protocol.
package circuit

import (
	"math/bits"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/piano"
)

// Builder accumulates PLONK gates and builds CircuitInfo.
type Builder struct {
	ql, qr, qm, qo, qk []fr.Element
	wL, wR, wO          []int
	nVars               int
	nRows               int
	capacity            int
}

// NewBuilder creates a new Builder with the given initial capacity.
func NewBuilder(capacity int) *Builder {
	return &Builder{
		capacity: capacity,
		ql:       make([]fr.Element, 0, capacity),
		qr:       make([]fr.Element, 0, capacity),
		qm:       make([]fr.Element, 0, capacity),
		qo:       make([]fr.Element, 0, capacity),
		qk:       make([]fr.Element, 0, capacity),
		wL:       make([]int, 0, capacity),
		wR:       make([]int, 0, capacity),
		wO:       make([]int, 0, capacity),
	}
}

// Init allocates variable 0 as the constant-zero variable.
// Must be called before any other method.
func (b *Builder) Init() {
	b.nVars = 1 // var 0 is the zero constant
}

// NewVar allocates a new variable and returns its index.
func (b *Builder) NewVar() int {
	idx := b.nVars
	b.nVars++
	return idx
}

// ZeroVar returns 0, the index of the constant-zero variable.
func (b *Builder) ZeroVar() int {
	return 0
}

// NbRows returns the current number of rows (gates).
func (b *Builder) NbRows() int {
	return b.nRows
}

// NbVars returns the current number of variables.
func (b *Builder) NbVars() int {
	return b.nVars
}

// AddGate appends a gate:  ql*L + qr*R + qm*L*R + qo*O + qk = 0
// l, r, o are variable indices. Returns the row index.
func (b *Builder) AddGate(l, r, o int, ql, qr, qm, qo, qk int64) int {
	row := b.nRows
	b.nRows++

	var vql, vqr, vqm, vqo, vqk fr.Element
	vql.SetInt64(ql)
	vqr.SetInt64(qr)
	vqm.SetInt64(qm)
	vqo.SetInt64(qo)
	vqk.SetInt64(qk)

	b.ql = append(b.ql, vql)
	b.qr = append(b.qr, vqr)
	b.qm = append(b.qm, vqm)
	b.qo = append(b.qo, vqo)
	b.qk = append(b.qk, vqk)
	b.wL = append(b.wL, l)
	b.wR = append(b.wR, r)
	b.wO = append(b.wO, o)

	return row
}

// BoolCheck enforces v*(1-v) = 0, i.e., v is boolean.
// Gate: L=v, R=v, O=zero_var; Ql=1, Qr=0, Qm=-1, Qo=0, Qk=0
// Expands: v - v*v = 0  ↔  v*(1-v)=0
func (b *Builder) BoolCheck(v int) {
	b.AddGate(v, v, b.ZeroVar(), 1, 0, -1, 0, 0)
}

// XOR computes out = in1 XOR in2 = in1 + in2 - 2*in1*in2.
// Gate: Ql=1, Qr=1, Qm=-2, Qo=-1, Qk=0
func (b *Builder) XOR(in1, in2 int) int {
	out := b.NewVar()
	b.AddGate(in1, in2, out, 1, 1, -2, -1, 0)
	return out
}

// AND computes out = in1 AND in2 = in1*in2.
// Gate: Qm=1, Qo=-1 (and other selectors zero)
func (b *Builder) AND(in1, in2 int) int {
	out := b.NewVar()
	b.AddGate(in1, in2, out, 0, 0, 1, -1, 0)
	return out
}

// NOT computes out = 1 - in.
// Gate: L=in, R=zero_var, O=out; Ql=1, Qr=0, Qm=0, Qo=1, Qk=-1
// Expands: in + out - 1 = 0  ↔  out = 1 - in
func (b *Builder) NOT(in int) int {
	out := b.NewVar()
	b.AddGate(in, b.ZeroVar(), out, 1, 0, 0, 1, -1)
	return out
}

// nextPow2 returns the smallest power of 2 >= n.
func nextPow2(n int) int {
	if n <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(n-1))
}

// Build pads to the next power of 2, builds the CircuitInfo, and returns
// the WitnessHelper that maps variable values to wire vectors.
func (b *Builder) Build() (*piano.CircuitInfo, *WitnessHelper) {
	T := nextPow2(b.nRows)
	if T < b.nRows {
		T = b.nRows
	}
	// Pad with zero rows (identity rows: all zero selectors, var 0 in all slots)
	for b.nRows < T {
		b.AddGate(0, 0, 0, 0, 0, 0, 0, 0)
	}

	// Build the flat lro array of length 3*T for BuildPermutation.
	lro := make([]int, 3*T)
	for j := 0; j < T; j++ {
		lro[j] = b.wL[j]
		lro[T+j] = b.wR[j]
		lro[2*T+j] = b.wO[j]
	}

	perm := piano.BuildPermutation(lro, b.nVars, T)

	ci := &piano.CircuitInfo{
		Ql:             make([]fr.Element, T),
		Qr:             make([]fr.Element, T),
		Qm:             make([]fr.Element, T),
		Qo:             make([]fr.Element, T),
		Qk:             make([]fr.Element, T),
		Permutation:    perm,
		NbPublicInputs: 0,
	}
	copy(ci.Ql, b.ql)
	copy(ci.Qr, b.qr)
	copy(ci.Qm, b.qm)
	copy(ci.Qo, b.qo)
	copy(ci.Qk, b.qk)

	wh := &WitnessHelper{
		wL: make([]int, T),
		wR: make([]int, T),
		wO: make([]int, T),
		t:  T,
	}
	copy(wh.wL, b.wL)
	copy(wh.wR, b.wR)
	copy(wh.wO, b.wO)

	return ci, wh
}

// WitnessHelper maps variable values to wire values per row.
type WitnessHelper struct {
	wL, wR, wO []int
	t          int
}

// Size returns the number of rows T.
func (wh *WitnessHelper) Size() int {
	return wh.t
}

// Make builds a WitnessInstance from a slice of variable values.
// varVals[i] is the field element value of variable i.
func (wh *WitnessHelper) Make(varVals []fr.Element) piano.WitnessInstance {
	T := wh.t
	wi := piano.WitnessInstance{
		L: make([]fr.Element, T),
		R: make([]fr.Element, T),
		O: make([]fr.Element, T),
	}
	for j := 0; j < T; j++ {
		wi.L[j].Set(&varVals[wh.wL[j]])
		wi.R[j].Set(&varVals[wh.wR[j]])
		wi.O[j].Set(&varVals[wh.wO[j]])
	}
	return wi
}
