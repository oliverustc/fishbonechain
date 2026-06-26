package imt

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"gnarkabc/gnarkwrapper"
	"gnarkabc/hash/mimchash"
)

// PreparedProof contains all values needed to deterministically assign
// a RootObfuscationProof circuit.
type PreparedProof struct {
	Leaf   []byte
	Path   [][]byte
	Root0  []byte
	Root1  []byte
	Root2  []byte
	Root3  []byte
	Index0 int
	Index1 int
}

const (
	domainPad      = "FISHBONE:DATA_TRADE:IMT:PAD:v1"
	domainDecoy    = "FISHBONE:DATA_TRADE:IMT:DECOY_ROOT:v1"
	separator byte = 0x00
)

func padString(prefix string, idx int) string {
	return fmt.Sprintf("%s%c%d", prefix, separator, idx)
}

func decoyString(prefix, datasetID, fieldName string, idx int) string {
	return fmt.Sprintf("%s%c%s%c%s%c%d", prefix, separator, datasetID, separator, fieldName, separator, idx)
}

func deterministicMiMC(curveName string, data []byte) ([]byte, error) {
	curve, ok := gnarkwrapper.CurveMap[curveName]
	if !ok {
		return nil, fmt.Errorf("unknown curve %q", curveName)
	}
	mod := curve.ScalarField()
	hashEntry, ok := mimchash.MiMCCaseMap[curveName]
	if !ok {
		return nil, fmt.Errorf("no MiMC case for curve %q", curveName)
	}
	hashFunc := hashEntry.Hash
	// Convert data to MiMC-friendly field element bytes via Convert2Byte.
	b := mimchash.Convert2Byte(string(data), mod)
	hash := mimchash.MiMCHash(hashFunc, [][]byte{b})
	return hash, nil
}

func deterministicMiMCPair(curveName string, left, right []byte) ([]byte, error) {
	curve, ok := gnarkwrapper.CurveMap[curveName]
	if !ok {
		return nil, fmt.Errorf("unknown curve %q", curveName)
	}
	mod := curve.ScalarField()
	hashEntry, ok := mimchash.MiMCCaseMap[curveName]
	if !ok {
		return nil, fmt.Errorf("no MiMC case for curve %q", curveName)
	}
	hashFunc := hashEntry.Hash
	lb := mimchash.Convert2Byte(string(left), mod)
	rb := mimchash.Convert2Byte(string(right), mod)
	hash := mimchash.MiMCHash(hashFunc, [][]byte{lb, rb})
	return hash, nil
}

// PrepareProof builds a deterministic depth-10 Merkle proof where leaf 0
// is masked_value_hash and all other leaves are deterministic padding
// leaves derived from fixture metadata.
func PrepareProof(curveName string, maskedValueHash []byte, f Fixture) (PreparedProof, error) {
	if err := f.Validate(); err != nil {
		return PreparedProof{}, err
	}
	if f.Depth < 1 {
		return PreparedProof{}, fmt.Errorf("depth must be >= 1, got %d", f.Depth)
	}

	// Build leaves: leaf 0 = masked_value_hash; others = deterministic padding.
	leaves := make([][]byte, 1<<f.Depth)
	leaves[0] = maskedValueHash
	for i := 1; i < len(leaves); i++ {
		padLabel := []byte(padString(domainPad, i))
		padLeaf, err := deterministicMiMC(curveName, padLabel)
		if err != nil {
			return PreparedProof{}, fmt.Errorf("padding leaf %d: %w", i, err)
		}
		leaves[i] = padLeaf
	}

	// Build deterministic Merkle tree bottom-up.
	nodes := make([][]byte, len(leaves))
	copy(nodes, leaves)
	proofPath := make([][]byte, f.Depth)
	for level := 0; level < f.Depth; level++ {
		step := 1 << level
		for i := 0; i < len(nodes); i += 2 * step {
			left := nodes[i]
			right := nodes[i+step]
			parent, err := deterministicMiMCPair(curveName, left, right)
			if err != nil {
				return PreparedProof{}, fmt.Errorf("merkle level %d index %d: %w", level, i, err)
			}
			nodes[i] = parent
		}
		// Capture the sibling for the proof path of leaf 0.
		// Leaf 0 is in the first pair (i=0, left at index 0, right at index step).
		sibling := nodes[step]
		proofPath[level] = make([]byte, len(sibling))
		copy(proofPath[level], sibling)
	}
	root := nodes[0]

	// Build deterministic decoy roots derived from fixture metadata.
	decoyRoots := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		label := []byte(decoyString(domainDecoy, f.DatasetID, f.FieldName, i))
		dr, err := deterministicMiMC(curveName, label)
		if err != nil {
			return PreparedProof{}, fmt.Errorf("decoy root %d: %w", i, err)
		}
		decoyRoots[i] = dr
	}

	return PreparedProof{
		Leaf:   maskedValueHash,
		Path:   proofPath,
		Root0:  root,
		Root1:  decoyRoots[0],
		Root2:  decoyRoots[1],
		Root3:  decoyRoots[2],
		Index0: 0,
		Index1: 0,
	}, nil
}

// u64le encodes v as 8-byte little-endian.
func u64le(v uint64) []byte {
	var out [8]byte
	binary.LittleEndian.PutUint64(out[:], v)
	return out[:]
}

// u64FromBytes converts 8-byte little-endian to uint64.
func u64FromBytes(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

// bigFromBytes converts a byte slice to *big.Int (big-endian unsigned).
func bigFromBytes(b []byte) *big.Int {
	return new(big.Int).SetBytes(b)
}
