package piano_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/piano"
)

// ────────────────────────────────────────────────────────────────────────────
// 测试辅助函数
// ────────────────────────────────────────────────────────────────────────────

// buildTrivialCircuit 为拥有 T 行、M 个子节点的平凡电路
// 返回 CircuitInfo 和 WitnessInstances。
// 门约束为 a·(-1) = 0，即 Ql=-1，其余选择子为 0。
// 连线置换为恒等（无共享变量）。
// Witness：所有 L[j]=0（满足 0·(-1)=0），R=O=0。
func buildTrivialCircuit(T, M int) (piano.CircuitInfo, []piano.WitnessInstance) {
	// 选择子：Ql[j] = -1（L 线输出），其余为 0。
	// 为简单起见，使用全零选择子 → 0·L+0·R+0·L·R+0·O+0 = 0 恒成立。
	zero := make([]fr.Element, T)

	// 恒等置换：槽 i 映射到自身。
	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i // 每条线各自独立变量
	}
	perm := piano.BuildPermutation(lro, 3*T, T)

	ci := piano.CircuitInfo{
		Ql:             cloneSlice(zero),
		Qr:             cloneSlice(zero),
		Qm:             cloneSlice(zero),
		Qo:             cloneSlice(zero),
		Qk:             cloneSlice(zero),
		Permutation:    perm,
		NbPublicInputs: 0,
	}

	// Witness：全为零（平凡满足）。
	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		witnesses[i] = piano.WitnessInstance{
			L: cloneSlice(zero),
			R: cloneSlice(zero),
			O: cloneSlice(zero),
		}
	}
	return ci, witnesses
}

// buildCopyConstraintCircuit 创建一个含复制约束的电路：
// 每一行的 L[0]、R[0]、O[0] 共享同一个变量。
// 门约束：Ql=-1，Qr=1，其余为 0，即 Ql·L + Qr·R = 0 → L = R。
// 其余行：全零选择子。
//
// 实际测试重点在于置换的正确性，而非门约束本身。
// 连线：行 0..T-1 中 L[0]=R[0] 通过置换关联。
func buildCopyConstraintCircuit(T, M int) (piano.CircuitInfo, []piano.WitnessInstance) {
	ql := make([]fr.Element, T)
	qr := make([]fr.Element, T)
	qm := make([]fr.Element, T)
	qo := make([]fr.Element, T)
	qk := make([]fr.Element, T)

	// 第 0 行：ql=-1，qr=1 → L=R 约束。
	ql[0].SetInt64(-1)
	qr[0].SetOne()

	// 置换：L[0] 和 R[0] 是同一变量（变量 0）。
	// 其余线各自独立。
	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	// 线 L[0] = 槽 0，R[0] = 槽 T → 同为变量 0。
	lro[0] = 0
	lro[T] = 0
	// 变量总数说明：槽 1..T-1 对应变量 1..T-1，槽 T+1..2T-1 对应 T..2T-2，
	// 槽 2T..3T-1 对应 2T-1..3T-2，槽 0 和 T 共享变量 0。
	// 为简单起见，使用 nbVariables = 3*T（部分变量未使用）。
	perm := piano.BuildPermutation(lro, 3*T, T)

	ci := piano.CircuitInfo{
		Ql:             ql,
		Qr:             qr,
		Qm:             qm,
		Qo:             qo,
		Qk:             qk,
		Permutation:    perm,
		NbPublicInputs: 0,
	}

	// Witness：L[0]=R[0]=5，其余为 0。
	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		L := make([]fr.Element, T)
		R := make([]fr.Element, T)
		O := make([]fr.Element, T)
		L[0].SetInt64(5)
		R[0].SetInt64(5)
		witnesses[i] = piano.WitnessInstance{L: L, R: R, O: O}
	}
	return ci, witnesses
}

func cloneSlice(s []fr.Element) []fr.Element {
	out := make([]fr.Element, len(s))
	copy(out, s)
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// 测试用例
// ────────────────────────────────────────────────────────────────────────────

// TestSetup 验证对小参数执行 Setup 不报错。
func TestSetup(t *testing.T) {
	T := uint64(4)
	M := uint64(2)
	ci, _ := buildTrivialCircuit(int(T), int(M))

	tauX := big.NewInt(7)
	tauY := big.NewInt(11)
	_, _, err := piano.SetupWithTrapdoors(ci, M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}
}

// TestProveTrivial 对全零平凡电路执行完整的 prove+verify 流程。
func TestProveTrivial(t *testing.T) {
	T := uint64(4)
	M := uint64(2)
	ci, witnesses := buildTrivialCircuit(int(T), int(M))

	tauX := big.NewInt(7)
	tauY := big.NewInt(11)
	pk, vk, err := piano.SetupWithTrapdoors(ci, M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}

	proof, err := piano.Prove(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Prove 失败：%v", err)
	}

	if err := piano.Verify(vk, proof, nil); err != nil {
		t.Fatalf("Verify 失败：%v", err)
	}
	t.Log("TestProveTrivial：证明验证成功")
}

// TestProveCopyConstraint 对含复制约束的电路执行 prove+verify 流程。
func TestProveCopyConstraint(t *testing.T) {
	T := uint64(4)
	M := uint64(2)
	ci, witnesses := buildCopyConstraintCircuit(int(T), int(M))

	tauX := big.NewInt(13)
	tauY := big.NewInt(17)
	pk, vk, err := piano.SetupWithTrapdoors(ci, M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}

	proof, err := piano.Prove(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Prove 失败：%v", err)
	}

	if err := piano.Verify(vk, proof, nil); err != nil {
		t.Fatalf("Verify 失败：%v", err)
	}
	t.Log("TestProveCopyConstraint：证明验证成功")
}

// TestProveM4 测试 M=4 个子节点的场景。
func TestProveM4(t *testing.T) {
	T := uint64(4)
	M := uint64(4)
	ci, witnesses := buildTrivialCircuit(int(T), int(M))

	tauX := big.NewInt(3)
	tauY := big.NewInt(5)
	pk, vk, err := piano.SetupWithTrapdoors(ci, M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}

	proof, err := piano.Prove(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Prove 失败：%v", err)
	}

	if err := piano.Verify(vk, proof, nil); err != nil {
		t.Fatalf("Verify 失败：%v", err)
	}
	t.Log("TestProveM4：证明验证成功")
}

// TestBuildPermutation 验证 BuildPermutation 能正确生成恒等置换和环置换。
func TestBuildPermutation(t *testing.T) {
	T := 4
	// 所有线独立（期望恒等置换）。
	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	perm := piano.BuildPermutation(lro, 3*T, T)
	for i, p := range perm {
		if p != int64(i) {
			t.Errorf("期望恒等置换，位置 %d 得到 %d", i, p)
		}
	}

	// 两个槽共享同一变量：期望 2-环置换。
	lro2 := make([]int, 3*T)
	for i := range lro2 {
		lro2[i] = i
	}
	lro2[0] = 0
	lro2[T] = 0 // 槽 0 和槽 T 是同一变量
	perm2 := piano.BuildPermutation(lro2, 3*T, T)
	// 期望 perm2[0] = T，perm2[T] = 0（2-环）。
	if perm2[0] != int64(T) || perm2[T] != 0 {
		t.Errorf("期望 2-环：perm2[0]=%d（期望 %d），perm2[T]=%d（期望 0）", perm2[0], T, perm2[T])
	}
}
