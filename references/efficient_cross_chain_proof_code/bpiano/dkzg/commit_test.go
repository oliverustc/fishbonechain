package dkzg

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// TestCommitLocalBasic 验证 CommitLocal 能生成正确的 G1 点。
//
// 对于求值全为 1 的常数多项式 f_i（即 evals[j] = 1），承诺为：
//
//	com_i = Σ_j 1 · Ux[i][j] = Vy[i]
//
// 这由单位划分性质 Σ_j L_j(τ_X) = 1 保证：
//
//	Σ_j Ux[i][j] = g^{R_i(τ_Y) · Σ L_j(τ_X)} = g^{R_i(τ_Y)} = Vy[i]。
func TestCommitLocalBasic(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 常数多项式 f_i = 1：所有求值为 1。
	evals := make([]fr.Element, T)
	for j := range evals {
		evals[j].SetOne()
	}

	for i := uint64(0); i < M; i++ {
		com, err := CommitLocal(i, evals, srs)
		if err != nil {
			t.Fatalf("CommitLocal(%d): %v", i, err)
		}
		// f_i = 1 时，com_i 应等于 Vy[i]。
		if !com.Equal(&srs.Vy[i]) {
			t.Errorf("CommitLocal(%d) 常数 1 多项式：期望 Vy[%d]，得到不同点", i, i)
		}
	}
}

// TestCommitLocalSingleBasis 验证对第 k 个 Lagrange 基多项式的承诺，
// 该多项式在 index k 处为 1，其余为 0。
//
// com_i = 1 · Ux[i][k] + Σ_{j≠k} 0 · Ux[i][j] = Ux[i][k]。
func TestCommitLocalSingleBasis(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	for k := uint64(0); k < T; k++ {
		evals := make([]fr.Element, T)
		evals[k].SetOne() // f(ω^k) = 1，f(ω^j) = 0（j ≠ k）

		for i := uint64(0); i < M; i++ {
			com, err := CommitLocal(i, evals, srs)
			if err != nil {
				t.Fatalf("CommitLocal(%d, k=%d): %v", i, k, err)
			}
			if !com.Equal(&srs.Ux[i][k]) {
				t.Errorf("CommitLocal(%d) 基多项式 k=%d：期望 Ux[%d][%d]", i, k, i, k)
			}
		}
	}
}

// TestCommitGlobalVsAggregate 验证 CommitGlobal 与各子节点 CommitLocal 聚合结果相等。
func TestCommitGlobalVsAggregate(t *testing.T) {
	tauX := big.NewInt(3)
	tauY := big.NewInt(11)
	M := uint64(4)
	T := uint64(8)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 构造测试用求值数据。
	allEvals := make([][]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		allEvals[i] = make([]fr.Element, T)
		for j := uint64(0); j < T; j++ {
			allEvals[i][j].SetUint64(i*T + j + 1)
		}
	}

	// 通过便捷函数直接计算全局承诺。
	comGlobal, err := CommitGlobal(allEvals, srs)
	if err != nil {
		t.Fatalf("CommitGlobal: %v", err)
	}

	// 通过 CommitLocal + AggregateDigests 计算全局承诺。
	localDigests := make([]Digest, M)
	for i := uint64(0); i < M; i++ {
		localDigests[i], err = CommitLocal(i, allEvals[i], srs)
		if err != nil {
			t.Fatalf("CommitLocal(%d): %v", i, err)
		}
	}
	comAgg, err := AggregateDigests(localDigests)
	if err != nil {
		t.Fatalf("AggregateDigests: %v", err)
	}

	if !comGlobal.Equal(&comAgg) {
		t.Error("CommitGlobal != AggregateDigests(CommitLocal(...))")
	}
}

// TestCommitGlobalLinear 验证同态性：commit(f+g) = commit(f) + commit(g)。
func TestCommitGlobalLinear(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(13)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 两组求值数据。
	evalsA := make([][]fr.Element, M)
	evalsB := make([][]fr.Element, M)
	evalsSum := make([][]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		evalsA[i] = make([]fr.Element, T)
		evalsB[i] = make([]fr.Element, T)
		evalsSum[i] = make([]fr.Element, T)
		for j := uint64(0); j < T; j++ {
			evalsA[i][j].SetUint64(i + j + 1)
			evalsB[i][j].SetUint64((i + 1) * (j + 2))
			evalsSum[i][j].Add(&evalsA[i][j], &evalsB[i][j])
		}
	}

	comA, err := CommitGlobal(evalsA, srs)
	if err != nil {
		t.Fatalf("CommitGlobal A: %v", err)
	}
	comB, err := CommitGlobal(evalsB, srs)
	if err != nil {
		t.Fatalf("CommitGlobal B: %v", err)
	}
	comSum, err := CommitGlobal(evalsSum, srs)
	if err != nil {
		t.Fatalf("CommitGlobal Sum: %v", err)
	}

	// 在 G1 中计算 comA + comB。
	var comAPlusB bn254.G1Jac
	var comAJac, comBJac bn254.G1Jac
	comAJac.FromAffine(&comA)
	comBJac.FromAffine(&comB)
	comAPlusB.Set(&comAJac)
	comAPlusB.AddAssign(&comBJac)
	var comAPlusBaff bn254.G1Affine
	comAPlusBaff.FromJacobian(&comAPlusB)

	if !comSum.Equal(&comAPlusBaff) {
		t.Error("承诺不满足加法同态：commit(A+B) != commit(A) + commit(B)")
	}
}

// TestCommitErrors 验证非法输入能返回正确的错误。
func TestCommitErrors(t *testing.T) {
	tauX := big.NewInt(2)
	tauY := big.NewInt(3)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	evals := make([]fr.Element, T)

	// 节点索引越界。
	if _, err := CommitLocal(M, evals, srs); err != ErrInvalidNodeIndex {
		t.Errorf("期望 ErrInvalidNodeIndex，得到 %v", err)
	}

	// 多项式长度错误。
	shortEvals := make([]fr.Element, T-1)
	if _, err := CommitLocal(0, shortEvals, srs); err != ErrInvalidPolySize {
		t.Errorf("期望 ErrInvalidPolySize，得到 %v", err)
	}

	// AggregateDigests 传入空切片。
	if _, err := AggregateDigests(nil); err != ErrMismatchedInputs {
		t.Errorf("期望 ErrMismatchedInputs（nil 切片），得到 %v", err)
	}
}

// TestCommitKnownValue 用手工计算值验证承诺结果。
//
// 使用 τ_X=2，τ_Y=3，M=2，T=4：
// 多项式 f_0(X) 在 X=ω^0 处取值 v，其余为 0。
// 则 com_0 = v · Ux[0][0]。
func TestCommitKnownValue(t *testing.T) {
	tauX := big.NewInt(2)
	tauY := big.NewInt(3)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// f_0 在 index 0 处取值 v=5，其余为 0。
	var v fr.Element
	v.SetUint64(5)
	evals := make([]fr.Element, T)
	evals[0].Set(&v)

	com, err := CommitLocal(0, evals, srs)
	if err != nil {
		t.Fatalf("CommitLocal: %v", err)
	}

	// 期望：5 · Ux[0][0]。
	var vBig big.Int
	v.BigInt(&vBig)
	var expected bn254.G1Affine
	expected.ScalarMultiplication(&srs.Ux[0][0], &vBig)

	if !com.Equal(&expected) {
		t.Error("单基多项式的承诺与 5·Ux[0][0] 不匹配")
	}
}
