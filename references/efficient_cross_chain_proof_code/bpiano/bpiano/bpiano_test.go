package bpiano_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/piano"
)

// ────────────────────────────────────────────────────────────────────────────
// 辅助函数（与 piano_test 中的辅助函数对应）
// ────────────────────────────────────────────────────────────────────────────

func buildTrivialCircuit(T, M int) (piano.CircuitInfo, []piano.WitnessInstance) {
	zero := make([]fr.Element, T)
	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	perm := piano.BuildPermutation(lro, 3*T, T)
	ci := piano.CircuitInfo{
		Ql: cloneSlice(zero), Qr: cloneSlice(zero),
		Qm: cloneSlice(zero), Qo: cloneSlice(zero),
		Qk: cloneSlice(zero), Permutation: perm,
	}
	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		witnesses[i] = piano.WitnessInstance{
			L: cloneSlice(zero), R: cloneSlice(zero), O: cloneSlice(zero),
		}
	}
	return ci, witnesses
}

func buildCopyConstraintCircuit(T, M int) (piano.CircuitInfo, []piano.WitnessInstance) {
	ql := make([]fr.Element, T)
	qr := make([]fr.Element, T)
	ql[0].SetInt64(-1); qr[0].SetOne()

	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	lro[0] = 0; lro[T] = 0
	perm := piano.BuildPermutation(lro, 3*T, T)

	zero := make([]fr.Element, T)
	ci := piano.CircuitInfo{
		Ql: ql, Qr: qr,
		Qm: cloneSlice(zero), Qo: cloneSlice(zero),
		Qk: cloneSlice(zero), Permutation: perm,
	}
	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		L := make([]fr.Element, T)
		R := make([]fr.Element, T)
		O := make([]fr.Element, T)
		L[0].SetInt64(5); R[0].SetInt64(5)
		witnesses[i] = piano.WitnessInstance{L: L, R: R, O: O}
	}
	return ci, witnesses
}

func cloneSlice(s []fr.Element) []fr.Element {
	out := make([]fr.Element, len(s))
	copy(out, s)
	return out
}

func setup(t *testing.T, T, M int, ci piano.CircuitInfo) (*piano.ProvingKey, *piano.VerifyingKey) {
	t.Helper()
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)
	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}
	return pk, vk
}

// ────────────────────────────────────────────────────────────────────────────
// 测试 1 — Compress + VerifyCompressed：平凡电路
// ────────────────────────────────────────────────────────────────────────────

func TestCompressVerifyTrivial(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress 失败：%v", err)
	}
	if err := bpiano.VerifyCompressed(proof, vk, nil); err != nil {
		t.Fatalf("VerifyCompressed 失败：%v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 测试 2 — 配对验证应拒绝篡改的 ComQX
// ────────────────────────────────────────────────────────────────────────────

func TestVerifyRejectsTamperedProof(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress 失败：%v", err)
	}

	// 篡改 ComQX — 配对验证必须失败。
	tampered := *proof
	tampered.ComQX = tampered.LRO[0] // 错误的 G1 点
	if err := bpiano.VerifyCompressed(&tampered, vk, nil); err == nil {
		t.Fatal("期望篡改 ComQX 后验证失败，得到 nil")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 测试 3 — 含复制约束的电路可以压缩并验证
// ────────────────────────────────────────────────────────────────────────────

func TestCompressVerifyCopyConstraint(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildCopyConstraintCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress 失败：%v", err)
	}
	if err := bpiano.VerifyCompressed(proof, vk, nil); err != nil {
		t.Fatalf("VerifyCompressed 失败：%v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 测试 4 — K=2 批量：CoordinateChallenges + Aggregate + VerifyBatch
// ────────────────────────────────────────────────────────────────────────────

func TestBatchK2(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	// K=2 个证明：均使用相同的电路和 witness。
	K := 2
	pks := make([]*piano.ProvingKey, K)
	wits := make([][]piano.WitnessInstance, K)
	pubs := make([][][]fr.Element, K)
	for k := 0; k < K; k++ {
		pks[k] = pk
		wits[k] = witnesses
	}

	proofs, err := bpiano.CoordinateChallenges(pks, wits, pubs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}
	batch, err := bpiano.Aggregate(proofs)
	if err != nil {
		t.Fatalf("Aggregate 失败：%v", err)
	}
	if err := bpiano.VerifyBatch(batch, vk, pubs); err != nil {
		t.Fatalf("VerifyBatch 失败：%v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 测试 5 — K=3 批量中含一个无效证明，应被拒绝
// ────────────────────────────────────────────────────────────────────────────

func TestBatchRejectsInvalidProof(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	K := 3
	pks := make([]*piano.ProvingKey, K)
	wits := make([][]piano.WitnessInstance, K)
	pubs := make([][][]fr.Element, K)
	for k := 0; k < K; k++ {
		pks[k] = pk
		wits[k] = witnesses
	}

	proofs, err := bpiano.CoordinateChallenges(pks, wits, pubs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}

	// 篡改 proof[1]。
	proofs[1].EvalA.SetInt64(999)

	batch, err := bpiano.Aggregate(proofs)
	if err != nil {
		t.Fatalf("Aggregate 失败：%v", err)
	}
	if err := bpiano.VerifyBatch(batch, vk, pubs); err == nil {
		t.Fatal("期望 VerifyBatch 拒绝含篡改证明的批次，得到 nil")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Phase 5 测试：聚合验证
// ────────────────────────────────────────────────────────────────────────────

// TestCoordinateChallengesSharedChallenges 验证 K=2 时两个证明使用协调挑战：
// 单独调用 VerifyCompressed 应失败（证明使用 sharedAlpha，非 FS 派生的 alpha），
// 而 VerifyBatch 应成功。
func TestCoordinateChallengesSharedChallenges(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	K := 2
	pks := make([]*piano.ProvingKey, K)
	wits := make([][]piano.WitnessInstance, K)
	pubs := make([][][]fr.Element, K)
	for k := 0; k < K; k++ {
		pks[k] = pk
		wits[k] = witnesses
	}

	proofs, err := bpiano.CoordinateChallenges(pks, wits, pubs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}

	// 单独用 VerifyCompressed 验证每个证明：应失败（使用了非 FS 的共享 alpha）
	for k, proof := range proofs {
		if err := bpiano.VerifyCompressed(proof, vk, pubs[k]); err == nil {
			t.Errorf("proof[%d]：期望 VerifyCompressed 失败（协调挑战格式），得到 nil", k)
		}
	}

	// VerifyBatch 应成功
	agg, err := bpiano.AggregateProofs(proofs)
	if err != nil {
		t.Fatalf("AggregateProofs 失败：%v", err)
	}
	if err := bpiano.VerifyBatch(agg, vk, pubs); err != nil {
		t.Fatalf("VerifyBatch 失败：%v", err)
	}
}

// TestVerifyBatch_K1 验证 K=1 的聚合证明可以通过 VerifyBatch。
func TestVerifyBatch_K1(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	pks := []*piano.ProvingKey{pk}
	wits := [][]piano.WitnessInstance{witnesses}
	pubs := [][][]fr.Element{{nil}}

	proofs, err := bpiano.CoordinateChallenges(pks, wits, pubs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}
	agg, err := bpiano.AggregateProofs(proofs)
	if err != nil {
		t.Fatalf("AggregateProofs 失败：%v", err)
	}
	if err := bpiano.VerifyBatch(agg, vk, pubs); err != nil {
		t.Fatalf("VerifyBatch K=1 失败：%v", err)
	}
}

// TestVerifyBatch_K2DifferentWitness 验证 K=2 且两个证明 witness 不同时聚合验证通过。
func TestVerifyBatch_K2DifferentWitness(t *testing.T) {
	T, M := 4, 2
	// 使用平凡电路（所有选择子为零，任意 witness 满足约束）
	ci, _ := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	zero := make([]fr.Element, T)
	makeWitness := func(val int64) []piano.WitnessInstance {
		ws := make([]piano.WitnessInstance, M)
		for i := range ws {
			L := make([]fr.Element, T)
			L[0].SetInt64(val)
			ws[i] = piano.WitnessInstance{
				L: L, R: append([]fr.Element{}, zero...), O: append([]fr.Element{}, zero...),
			}
		}
		return ws
	}

	pks := []*piano.ProvingKey{pk, pk}
	wits := [][]piano.WitnessInstance{makeWitness(3), makeWitness(7)}
	pubs := [][][]fr.Element{nil, nil}

	proofs, err := bpiano.CoordinateChallenges(pks, wits, pubs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}
	agg, err := bpiano.AggregateProofs(proofs)
	if err != nil {
		t.Fatalf("AggregateProofs 失败：%v", err)
	}
	if err := bpiano.VerifyBatch(agg, vk, pubs); err != nil {
		t.Fatalf("VerifyBatch K=2（不同 witness）失败：%v", err)
	}
}

// TestVerifyBatch_K4 验证 K=4 的聚合证明可以通过 VerifyBatch。
func TestVerifyBatch_K4(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	K := 4
	pks := make([]*piano.ProvingKey, K)
	wits := make([][]piano.WitnessInstance, K)
	pubs := make([][][]fr.Element, K)
	for k := 0; k < K; k++ {
		pks[k] = pk
		wits[k] = witnesses
	}

	proofs, err := bpiano.CoordinateChallenges(pks, wits, pubs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}
	agg, err := bpiano.AggregateProofs(proofs)
	if err != nil {
		t.Fatalf("AggregateProofs 失败：%v", err)
	}
	if err := bpiano.VerifyBatch(agg, vk, pubs); err != nil {
		t.Fatalf("VerifyBatch K=4 失败：%v", err)
	}
}

// TestVerifyBatch_Tamper 验证篡改聚合证明中某个 eval 后 VerifyBatch 返回错误。
func TestVerifyBatch_Tamper(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildTrivialCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	pks := []*piano.ProvingKey{pk, pk}
	wits := [][]piano.WitnessInstance{witnesses, witnesses}
	pubs := [][][]fr.Element{nil, nil}

	proofs, err := bpiano.CoordinateChallenges(pks, wits, pubs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}

	// 篡改 proof[1] 的某个求值
	proofs[1].EvalA.SetInt64(42)

	agg, err := bpiano.AggregateProofs(proofs)
	if err != nil {
		t.Fatalf("AggregateProofs 失败：%v", err)
	}
	if err := bpiano.VerifyBatch(agg, vk, pubs); err == nil {
		t.Fatal("期望 VerifyBatch 拒绝篡改后的聚合证明，得到 nil")
	}
}
