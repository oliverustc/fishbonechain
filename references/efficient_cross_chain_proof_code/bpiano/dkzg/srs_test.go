package dkzg

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
)

// TestSRSCorrectness 通过手工计算值验证 SRS 元素的正确性。
//
// 使用 τ_X = 2，τ_Y = 3，M = 2，T = 4，验证：
//  1. Vy[i] == g1^{R_i(τ_Y)}
//  2. Ux[i][j] == g1^{R_i(τ_Y) · L_j(τ_X)}
//  3. G2[1] == g2^{τ_X}，G2Y[1] == g2^{τ_Y}
func TestSRSCorrectness(t *testing.T) {
	tauX := big.NewInt(2)
	tauY := big.NewInt(3)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	_, _, g1Aff, g2Aff := bn254.Generators()

	// 手工计算 Lagrange 基值。
	lagY, err := evalLagrangeBasis(fieldFromBig(tauY), M, srs.DomainY.Generator)
	if err != nil {
		t.Fatalf("evalLagrangeBasis Y: %v", err)
	}
	lagX, err := evalLagrangeBasis(fieldFromBig(tauX), T, srs.DomainX.Generator)
	if err != nil {
		t.Fatalf("evalLagrangeBasis X: %v", err)
	}

	// 验证 Vy[i] == g1^{lagY[i]}。
	for i := uint64(0); i < M; i++ {
		var lagYBig big.Int
		lagY[i].BigInt(&lagYBig)
		var expected bn254.G1Affine
		expected.ScalarMultiplication(&g1Aff, &lagYBig)
		if !expected.Equal(&srs.Vy[i]) {
			t.Errorf("Vy[%d] 不匹配", i)
		}
	}

	// 验证 Ux[i][j] == g1^{lagY[i] * lagX[j]}。
	for i := uint64(0); i < M; i++ {
		for j := uint64(0); j < T; j++ {
			var scalar fr.Element
			scalar.Mul(&lagY[i], &lagX[j])
			var scalarBig big.Int
			scalar.BigInt(&scalarBig)

			var expected bn254.G1Affine
			expected.ScalarMultiplication(&g1Aff, &scalarBig)
			if !expected.Equal(&srs.Ux[i][j]) {
				t.Errorf("Ux[%d][%d] 不匹配", i, j)
			}
		}
	}

	// 验证 G2 元素。
	var expectedG2X bn254.G2Affine
	expectedG2X.ScalarMultiplication(&g2Aff, tauX)
	if !expectedG2X.Equal(&srs.G2[1]) {
		t.Error("G2[1] 不匹配")
	}

	var expectedG2Y bn254.G2Affine
	expectedG2Y.ScalarMultiplication(&g2Aff, tauY)
	if !expectedG2Y.Equal(&srs.G2Y[1]) {
		t.Error("G2Y[1] 不匹配")
	}
}

// TestSRSPartitionOfUnity 验证单位划分性质：
//   - 对任意非单位根 α，Σ_j L_j(α) = 1
//   - Σ_j Ux[i][j] = Vy[i]（G1 行求和等于 Y 轴基元素）
func TestSRSPartitionOfUnity(t *testing.T) {
	tauX := big.NewInt(7)
	tauY := big.NewInt(11)
	M := uint64(4)
	T := uint64(8)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	var one fr.Element
	one.SetOne()

	// X 轴单位划分。
	var alpha fr.Element
	alpha.SetUint64(1234567)
	lagX, err := evalLagrangeBasis(alpha, T, srs.DomainX.Generator)
	if err != nil {
		t.Fatalf("evalLagrangeBasis X: %v", err)
	}
	var sumX fr.Element
	for _, l := range lagX {
		sumX.Add(&sumX, &l)
	}
	if !sumX.Equal(&one) {
		t.Errorf("X 轴单位划分失败：sum = %v", sumX)
	}

	// Y 轴单位划分。
	var beta fr.Element
	beta.SetUint64(9999999)
	lagY, err := evalLagrangeBasis(beta, M, srs.DomainY.Generator)
	if err != nil {
		t.Fatalf("evalLagrangeBasis Y: %v", err)
	}
	var sumY fr.Element
	for _, r := range lagY {
		sumY.Add(&sumY, &r)
	}
	if !sumY.Equal(&one) {
		t.Errorf("Y 轴单位划分失败：sum = %v", sumY)
	}

	// G1 行求和：Σ_j Ux[i][j] = Vy[i]。
	// 由 Σ_j L_j(τ_X) = 1 推导：
	//   Σ_j g^{R_i(τ_Y)·L_j(τ_X)} = g^{R_i(τ_Y)·1} = Vy[i]。
	for i := uint64(0); i < M; i++ {
		var rowSum bn254.G1Jac
		for j := uint64(0); j < T; j++ {
			var pt bn254.G1Jac
			pt.FromAffine(&srs.Ux[i][j])
			rowSum.AddAssign(&pt)
		}
		var rowSumAff bn254.G1Affine
		rowSumAff.FromJacobian(&rowSum)
		if !rowSumAff.Equal(&srs.Vy[i]) {
			t.Errorf("行求和 Ux[%d][:] != Vy[%d]", i, i)
		}
	}
}

// TestSRSErrors 验证非法参数能返回正确的错误。
func TestSRSErrors(t *testing.T) {
	tauX := big.NewInt(2)
	tauY := big.NewInt(3)

	if _, err := NewTestSRS(3, 4, tauX, tauY); err != ErrMNotPowerOfTwo {
		t.Errorf("期望 ErrMNotPowerOfTwo，得到 %v", err)
	}
	if _, err := NewTestSRS(2, 6, tauX, tauY); err != ErrTNotPowerOfTwo {
		t.Errorf("期望 ErrTNotPowerOfTwo，得到 %v", err)
	}
	if _, err := NewTestSRS(1, 4, tauX, tauY); err != ErrMNotPowerOfTwo {
		t.Errorf("期望 ErrMNotPowerOfTwo（M=1），得到 %v", err)
	}
	if _, err := NewTestSRS(0, 4, tauX, tauY); err != ErrMNotPowerOfTwo {
		t.Errorf("期望 ErrMNotPowerOfTwo（M=0），得到 %v", err)
	}
}

// TestEvalLagrangeBasisRootOfUnity 验证传入单位根时返回错误。
func TestEvalLagrangeBasisRootOfUnity(t *testing.T) {
	T := uint64(4)
	domain := fft.NewDomain(T)

	// 生成元本身是单位根。
	gen := domain.Generator
	_, err := evalLagrangeBasis(gen, T, domain.Generator)
	if err == nil {
		t.Error("alpha 为单位根时应返回错误，但得到 nil")
	}

	// 测试 alpha = 1（平凡单位根，即 gen^0）。
	var one fr.Element
	one.SetOne()
	_, err = evalLagrangeBasis(one, T, domain.Generator)
	if err == nil {
		t.Error("alpha = 1（单位根）时应返回错误，但得到 nil")
	}
}

// TestSRSLagrangeKronecker 在非单位根点处验证 L_j 的取值，
// 并通过直接域运算检查 Lagrange 插值恒等式。
func TestSRSLagrangeKronecker(t *testing.T) {
	T := uint64(4)
	M := uint64(2)
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 在非单位根点处求 X 轴 Lagrange 基，并验证求和为 1。
	var alpha fr.Element
	alpha.SetUint64(42)
	lagX, err := evalLagrangeBasis(alpha, T, srs.DomainX.Generator)
	if err != nil {
		t.Fatalf("evalLagrangeBasis: %v", err)
	}

	var sum fr.Element
	for _, l := range lagX {
		sum.Add(&sum, &l)
	}
	var one fr.Element
	one.SetOne()
	if !sum.Equal(&one) {
		t.Errorf("Lagrange 求和 != 1：得到 %v", sum)
	}

	// 直接用公式计算并与 evalLagrangeBasis 结果对比：
	// 正确公式：L_j(α) = ω^j · (α^N - 1) / (N · (α - ω^j))
	var alphaN fr.Element
	{
		exp := new(big.Int).SetUint64(T)
		alphaN.Exp(alpha, exp)
	}
	var numeratorRaw fr.Element
	var oneField fr.Element
	oneField.SetOne()
	numeratorRaw.Sub(&alphaN, &oneField)

	var NField, NInv fr.Element
	NField.SetUint64(T)
	NInv.Inverse(&NField)

	var numer fr.Element
	numer.Mul(&numeratorRaw, &NInv)

	gen := srs.DomainX.Generator
	genPow := oneField
	for j := uint64(0); j < T; j++ {
		var denom, denomInv fr.Element
		denom.Sub(&alpha, &genPow)
		denomInv.Inverse(&denom)
		var lj fr.Element
		lj.Mul(&numer, &denomInv)
		lj.Mul(&lj, &genPow) // 乘以 ω^j
		if !lj.Equal(&lagX[j]) {
			t.Errorf("L_%d 不匹配：得到 %v，期望 %v", j, lagX[j], lj)
		}
		genPow.Mul(&genPow, &gen)
	}
}

// fieldFromBig 将 *big.Int 转换为 fr.Element。
func fieldFromBig(x *big.Int) fr.Element {
	var e fr.Element
	e.SetBigInt(x)
	return e
}
