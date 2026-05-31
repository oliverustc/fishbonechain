package keccak

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/piano"
)

// TestKeccakCircuitBuild 验证：
//  1. 电路构建不会 panic。
//  2. 输出变量与原生 Keccak-256 哈希一致。
//  3. witness 满足所有 PLONK 约束。
func TestKeccakCircuitBuild(t *testing.T) {
	t.Log("正在构建 Keccak-256 电路...")
	kc := Build()
	ci := kc.CircuitInfo()
	wh := kc.WitnessHelper()
	T := wh.Size()
	t.Logf("电路规模 T = %d", T)

	// 使用固定的 64 字节消息。
	var msgBytes [64]byte
	for i := range msgBytes {
		msgBytes[i] = byte(i)
	}

	t.Log("正在计算 witness...")
	varVals := kc.WitnessFor(msgBytes)

	// 验证原生哈希与电路输出变量一致。
	t.Log("验证原生哈希与电路输出是否一致...")
	expectedHash := Hash256(msgBytes[:])
	gotHash := extractOutputHashTest(kc, varVals)
	if gotHash != expectedHash {
		t.Errorf("哈希不一致：\n  得到  %x\n  期望 %x", gotHash, expectedHash)
	} else {
		t.Logf("哈希一致：%x", expectedHash[:8])
	}

	// 验证所有 PLONK 约束均被满足。
	t.Log("验证电路约束...")
	wi := wh.Make(varVals)
	checkConstraints(t, ci, &wi, T)
}

func extractOutputHashTest(kc *KeccakCircuit, varVals []fr.Element) [32]byte {
	var bits [256]bool
	for i, v := range kc.OutputVars {
		bits[i] = !varVals[v].IsZero()
	}
	var h [32]byte
	for i := 0; i < 32; i++ {
		for z := 0; z < 8; z++ {
			if bits[i*8+z] {
				h[i] |= 1 << uint(z)
			}
		}
	}
	return h
}

func checkConstraints(t *testing.T, ci *piano.CircuitInfo, wi *piano.WitnessInstance, T int) {
	t.Helper()
	violations := 0
	for j := 0; j < T; j++ {
		l := wi.L[j]
		r := wi.R[j]
		o := wi.O[j]

		// ql·l + qr·r + qm·l·r + qo·o + qk = 0
		var v, tmp fr.Element
		v.Mul(&ci.Ql[j], &l)

		tmp.Mul(&ci.Qr[j], &r)
		v.Add(&v, &tmp)

		tmp.Mul(&l, &r)
		tmp.Mul(&ci.Qm[j], &tmp)
		v.Add(&v, &tmp)

		tmp.Mul(&ci.Qo[j], &o)
		v.Add(&v, &tmp)

		v.Add(&v, &ci.Qk[j])

		if !v.IsZero() {
			violations++
			if violations <= 5 {
				t.Errorf("第 %d 行约束不满足：残差=%v", j, v)
				t.Logf("  L=%v R=%v O=%v", l, r, o)
				t.Logf("  Ql=%v Qr=%v Qm=%v Qo=%v Qk=%v",
					ci.Ql[j], ci.Qr[j], ci.Qm[j], ci.Qo[j], ci.Qk[j])
			}
		}
	}
	if violations > 0 {
		t.Errorf("约束违例总数：%d / %d", violations, T)
	} else {
		t.Logf("全部 %d 个约束均已满足！", T)
	}
}
