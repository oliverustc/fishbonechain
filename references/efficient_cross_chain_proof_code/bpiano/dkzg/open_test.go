package dkzg

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// TestLocalOpenXClaimedValue 验证 LocalOpenX 能正确计算 f_i(alpha)。
//
// 对于以求值形式给出的多项式，f_i(alpha) 可由 Σ_j evals[j] · L_j(alpha) 计算，
// 本测试验证结果与相同公式计算的期望值一致。
func TestLocalOpenXClaimedValue(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 非零多项式 f_0(X)：evals[j] = j+1。
	evals := make([]fr.Element, T)
	for j := uint64(0); j < T; j++ {
		evals[j].SetUint64(j + 1)
	}

	// 求值点。
	var alpha fr.Element
	alpha.SetUint64(42)

	proof, err := LocalOpenX(0, evals, alpha, srs)
	if err != nil {
		t.Fatalf("LocalOpenX: %v", err)
	}

	// 期望值：通过 Lagrange 求值计算。
	expected, err := evalPolyLagrange(evals, alpha, srs.DomainX.Generator)
	if err != nil {
		t.Fatalf("evalPolyLagrange: %v", err)
	}

	if !proof.ClaimedValue.Equal(&expected) {
		t.Errorf("ClaimedValue 不匹配：得到 %v，期望 %v", proof.ClaimedValue, expected)
	}
}

// TestLocalOpenXQuotientConsistency 验证商多项式证明满足 KZG 等式。
//
// X 轴的 KZG 恒等式为：
//
//	com_i - v_i·Vy[i] = (τ_X - alpha) · π_{0,i}
//
// 其中 v_i = f_i(alpha)，Vy[i] = g^{R_i(τ_Y)}，com_i = g^{R_i(τ_Y)·f_i(τ_X)}。
// 由于 SRS 是 Lagrange 形式，我们通过已知陷门 τ_X=5 直接在 G1 中验证该等式。
func TestLocalOpenXQuotientConsistency(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	evals := make([]fr.Element, T)
	for j := uint64(0); j < T; j++ {
		evals[j].SetUint64(j + 1)
	}

	var alpha fr.Element
	alpha.SetUint64(3) // 求值点

	proof, err := LocalOpenX(0, evals, alpha, srs)
	if err != nil {
		t.Fatalf("LocalOpenX: %v", err)
	}

	// com_i = CommitLocal(0, evals)。
	com, err := CommitLocal(0, evals, srs)
	if err != nil {
		t.Fatalf("CommitLocal: %v", err)
	}

	// v_i · Vy[0]：声明值乘以 Y 轴基承诺。
	// 注意：Vy[i] = g^{R_i(τ_Y)}，com_i = g^{R_i(τ_Y)·f_i(τ_X)}，
	// 故 [v_i·R_i(τ_Y)]_1 = v_i · Vy[i]。
	var vBig big.Int
	proof.ClaimedValue.BigInt(&vBig)
	var vVy bn254.G1Affine
	vVy.ScalarMultiplication(&srs.Vy[0], &vBig)

	// LHS = com_i - v_i·Vy[0]，应等于 (τ_X - alpha) · π_{0,i}。
	var lhsJac, comJac, vVyJac bn254.G1Jac
	comJac.FromAffine(&com)
	vVyJac.FromAffine(&vVy)
	lhsJac.Set(&comJac)
	lhsJac.SubAssign(&vVyJac)

	// RHS = (τ_X - alpha) · π_{0,i}。
	var tauMinusAlpha fr.Element
	var tauXFr fr.Element
	tauXFr.SetBigInt(tauX)
	tauMinusAlpha.Sub(&tauXFr, &alpha)
	var tauMinusAlphaBig big.Int
	tauMinusAlpha.BigInt(&tauMinusAlphaBig)

	var rhsJac bn254.G1Jac
	var rhsAff bn254.G1Affine
	rhsAff.ScalarMultiplication(&proof.H, &tauMinusAlphaBig)
	rhsJac.FromAffine(&rhsAff)

	if !lhsJac.Equal(&rhsJac) {
		t.Error("商多项式恒等式失败：com_i - v_i·Vy[i] ≠ (τ_X - alpha)·π_{0,i}")
	}
}

// TestAggregateOpenX 验证 AggregateOpenX 返回的 com_VF 与手工计算结果一致。
func TestAggregateOpenX(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(4)
	T := uint64(8)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	allEvals := make([][]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		allEvals[i] = make([]fr.Element, T)
		for j := uint64(0); j < T; j++ {
			allEvals[i][j].SetUint64(i*100 + j + 1)
		}
	}

	var alpha fr.Element
	alpha.SetUint64(1234)

	// 收集各子节点的本地证明。
	localProofs := make([]OpeningProofX, M)
	for i := uint64(0); i < M; i++ {
		localProofs[i], err = LocalOpenX(i, allEvals[i], alpha, srs)
		if err != nil {
			t.Fatalf("LocalOpenX(%d): %v", i, err)
		}
	}

	comVF, _, err := AggregateOpenX(localProofs, srs)
	if err != nil {
		t.Fatalf("AggregateOpenX: %v", err)
	}

	// com_VF 应等于 Σ_i v_i · Vy[i]，其中 v_i = f_i(alpha)，通过重新计算验证。
	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i] = localProofs[i].ClaimedValue
	}
	var expectedComVF bn254.G1Affine
	if _, err := expectedComVF.MultiExp(srs.Vy, viEvals, ecc.MultiExpConfig{}); err != nil {
		t.Fatalf("MultiExp: %v", err)
	}

	if !comVF.Equal(&expectedComVF) {
		t.Error("AggregateOpenX 的 com_VF 与 Σ_i v_i · Vy[i] 不一致")
	}
}

// TestOpenYClaimedValue 验证 Y 轴开放的声明值正确性。
func TestOpenYClaimedValue(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(4)
	T := uint64(8)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	allEvals := make([][]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		allEvals[i] = make([]fr.Element, T)
		for j := uint64(0); j < T; j++ {
			allEvals[i][j].SetUint64(i*10 + j + 1)
		}
	}

	var alpha fr.Element
	alpha.SetUint64(999)

	// 获取各子节点的声明值 v_i。
	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i], err = evalPolyLagrange(allEvals[i], alpha, srs.DomainX.Generator)
		if err != nil {
			t.Fatalf("evalPolyLagrange(%d): %v", i, err)
		}
	}

	var beta fr.Element
	beta.SetUint64(7777)

	proofY, err := OpenY(viEvals, beta, srs)
	if err != nil {
		t.Fatalf("OpenY: %v", err)
	}

	// 验证 z = V_F(beta) = Σ_i v_i · R_i(beta)。
	expected, err := evalPolyLagrange(viEvals, beta, srs.DomainY.Generator)
	if err != nil {
		t.Fatalf("evalPolyLagrange Y: %v", err)
	}
	if !proofY.ClaimedValue.Equal(&expected) {
		t.Errorf("Y 轴声明值不匹配：得到 %v，期望 %v", proofY.ClaimedValue, expected)
	}
}

// TestOpenYQuotientConsistency 验证 Y 轴商多项式恒等式。
//
// 恒等式：com_VF - z·g1 = (τ_Y - beta) · π_{1,F}
func TestOpenYQuotientConsistency(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 简单的 v_i 值。
	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i].SetUint64(i + 2)
	}

	var beta fr.Element
	beta.SetUint64(13)

	proofY, err := OpenY(viEvals, beta, srs)
	if err != nil {
		t.Fatalf("OpenY: %v", err)
	}

	// 验证 proofY.ComVF = Σ_i v_i · Vy[i]。
	var expectedComVF bn254.G1Affine
	if _, err := expectedComVF.MultiExp(srs.Vy, viEvals, ecc.MultiExpConfig{}); err != nil {
		t.Fatalf("MultiExp: %v", err)
	}
	if !proofY.ComVF.Equal(&expectedComVF) {
		t.Error("OpeningProofY 中的 ComVF 错误")
	}

	// 验证商多项式恒等式：com_VF - z·g1 = (τ_Y - beta) · π_{1,F}。
	_, _, g1Aff, _ := bn254.Generators()

	var zBig big.Int
	proofY.ClaimedValue.BigInt(&zBig)
	var zG1 bn254.G1Affine
	zG1.ScalarMultiplication(&g1Aff, &zBig)

	var lhsJac, comVFJac, zG1Jac bn254.G1Jac
	comVFJac.FromAffine(&proofY.ComVF)
	zG1Jac.FromAffine(&zG1)
	lhsJac.Set(&comVFJac)
	lhsJac.SubAssign(&zG1Jac)

	// (τ_Y - beta) · π_{1,F}。
	var tauYFr, tauMinusBeta fr.Element
	tauYFr.SetBigInt(tauY)
	tauMinusBeta.Sub(&tauYFr, &beta)
	var tauMinusBetaBig big.Int
	tauMinusBeta.BigInt(&tauMinusBetaBig)

	var rhsAff bn254.G1Affine
	rhsAff.ScalarMultiplication(&proofY.H, &tauMinusBetaBig)
	var rhsJac bn254.G1Jac
	rhsJac.FromAffine(&rhsAff)

	if !lhsJac.Equal(&rhsJac) {
		t.Error("Y 轴商多项式恒等式失败：com_VF - z·g1 ≠ (τ_Y-beta)·π_{1,F}")
	}
}

// TestOpenErrors 验证开放函数在各类非法输入下能返回正确错误。
func TestOpenErrors(t *testing.T) {
	tauX := big.NewInt(2)
	tauY := big.NewInt(3)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	evals := make([]fr.Element, T)
	var alpha fr.Element
	alpha.SetUint64(42)

	// 节点索引越界。
	if _, err := LocalOpenX(M, evals, alpha, srs); err != ErrInvalidNodeIndex {
		t.Errorf("期望 ErrInvalidNodeIndex，得到 %v", err)
	}

	// 求值向量长度错误。
	if _, err := LocalOpenX(0, evals[:T-1], alpha, srs); err != ErrInvalidPolySize {
		t.Errorf("期望 ErrInvalidPolySize，得到 %v", err)
	}

	// alpha 是单位根。
	alphaRoot := srs.DomainX.Generator
	if _, err := LocalOpenX(0, evals, alphaRoot, srs); err != ErrAlphaIsRootOfUnityX {
		t.Errorf("期望 ErrAlphaIsRootOfUnityX，得到 %v", err)
	}

	// beta 是 OpenY 中的单位根。
	viEvals := make([]fr.Element, M)
	betaRoot := srs.DomainY.Generator
	if _, err := OpenY(viEvals, betaRoot, srs); err != ErrBetaIsRootOfUnityY {
		t.Errorf("期望 ErrBetaIsRootOfUnityY，得到 %v", err)
	}
}
