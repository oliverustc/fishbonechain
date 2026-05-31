package dkzg

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// Verify 对 F(beta, alpha) = z 的完整 DKZG 开放证明进行验证。
//
// 双变量多项式 F(Y,X) = Σ_i R_i(Y)·f_i(X) 的承诺为
// com_F = g^{F(τ_Y, τ_X)}。
//
// 在 (beta, alpha) 处的开放由以下部分组成：
//   - z = F(beta, alpha) ∈ Fr         （声明值）
//   - com_VF = g^{V_F(τ_Y)}           （V_F(Y) = Σ_i f_i(alpha)·R_i(Y) 的承诺）
//   - π_{0,F}                          （X 轴商多项式证明：com_F = com_VF + (τ_X-alpha)·π_{0,F}）
//   - π_{1,F}                          （Y 轴商多项式证明：com_VF = [z]_1 + (τ_Y-beta)·π_{1,F}）
//
// 验证需通过以下两个 pairing 等式：
//
//  1. e(com_F - com_VF, g2) = e(π_{0,F}, [τ_X - alpha]_2)
//  2. e(com_VF - [z]_1, g2) = e(π_{1,F}, [τ_Y - beta]_2)
//
// 注意：[z]_1 表示 z·g1（标量 z 乘以 G1 生成元），而非 z·Vy[i]。
// Y 轴多项式 V_F 使用 Vy 基承诺（其中编码了 R_i(τ_Y)），
// 故 com_VF 已包含 Y 轴结构。验证方以 g1 为常数承诺基检查 com_VF 在 beta 处的开放值为 z。
func Verify(
	comF Digest,
	alpha, beta fr.Element,
	proofX bn254.G1Affine, // π_{0,F}：聚合后的 X 轴商多项式证明
	proofY OpeningProofY,  // 包含 com_VF、z、π_{1,F}
	srs *SRS,
) error {
	_, _, g1Aff, _ := bn254.Generators()

	// 校验 1：e(com_F - com_VF, g2) = e(π_{0,F}, [τ_X - alpha]_2)
	//
	// 改写为：e(com_F - com_VF, g2) · e(-π_{0,F}, [τ_X - alpha]_2) = 1
	//         即 e(com_F - com_VF, g2) · e(π_{0,F}, [alpha - τ_X]_2) = 1
	//
	// [τ_X - alpha]_2 = srs.G2[1] - alpha·srs.G2[0]。

	var alphaBig big.Int
	alpha.BigInt(&alphaBig)

	var alphaG2 bn254.G2Affine
	alphaG2.ScalarMultiplication(&srs.G2[0], &alphaBig)

	var tauXMinusAlphaG2 bn254.G2Affine
	{
		var tauXMinusAlphaG2Jac bn254.G2Jac
		var tauXG2Jac, alphaG2Jac bn254.G2Jac
		tauXG2Jac.FromAffine(&srs.G2[1])
		alphaG2Jac.FromAffine(&alphaG2)
		tauXMinusAlphaG2Jac.Set(&tauXG2Jac)
		tauXMinusAlphaG2Jac.SubAssign(&alphaG2Jac)
		tauXMinusAlphaG2.FromJacobian(&tauXMinusAlphaG2Jac)
	}

	// 计算 com_F - com_VF。
	var diffXJac, comFJac, comVFJac bn254.G1Jac
	comFJac.FromAffine(&comF)
	comVFJac.FromAffine(&proofY.ComVF)
	diffXJac.Set(&comFJac)
	diffXJac.SubAssign(&comVFJac)
	var diffX bn254.G1Affine
	diffX.FromJacobian(&diffXJac)

	// 执行 pairing 校验 1。
	ok1, err := bn254.PairingCheck(
		[]bn254.G1Affine{diffX, negG1(proofX)},
		[]bn254.G2Affine{srs.G2[0], tauXMinusAlphaG2},
	)
	if err != nil {
		return err
	}
	if !ok1 {
		return ErrVerification
	}

	// 校验 2：e(com_VF - [z]_1, g2) = e(π_{1,F}, [τ_Y - beta]_2)
	//
	// [τ_Y - beta]_2 = srs.G2Y[1] - beta·srs.G2Y[0]。
	// [z]_1 = z · g1。

	var betaBig big.Int
	beta.BigInt(&betaBig)

	var betaG2Y bn254.G2Affine
	betaG2Y.ScalarMultiplication(&srs.G2Y[0], &betaBig)

	var tauYMinusBetaG2 bn254.G2Affine
	{
		var tauYMinusBetaG2Jac bn254.G2Jac
		var tauYG2Jac, betaG2YJac bn254.G2Jac
		tauYG2Jac.FromAffine(&srs.G2Y[1])
		betaG2YJac.FromAffine(&betaG2Y)
		tauYMinusBetaG2Jac.Set(&tauYG2Jac)
		tauYMinusBetaG2Jac.SubAssign(&betaG2YJac)
		tauYMinusBetaG2.FromJacobian(&tauYMinusBetaG2Jac)
	}

	// [z]_1 = z · g1。
	var zBig big.Int
	proofY.ClaimedValue.BigInt(&zBig)
	var zG1 bn254.G1Affine
	zG1.ScalarMultiplication(&g1Aff, &zBig)

	// 计算 com_VF - [z]_1。
	var diffYJac, zG1Jac bn254.G1Jac
	zG1Jac.FromAffine(&zG1)
	diffYJac.Set(&comVFJac)
	diffYJac.SubAssign(&zG1Jac)
	var diffY bn254.G1Affine
	diffY.FromJacobian(&diffYJac)

	// 执行 pairing 校验 2。
	ok2, err := bn254.PairingCheck(
		[]bn254.G1Affine{diffY, negG1(proofY.H)},
		[]bn254.G2Affine{srs.G2Y[0], tauYMinusBetaG2},
	)
	if err != nil {
		return err
	}
	if !ok2 {
		return ErrVerification
	}

	return nil
}

// negG1 返回 G1Affine 点的取反。
func negG1(p bn254.G1Affine) bn254.G1Affine {
	var neg bn254.G1Affine
	neg.Neg(&p)
	return neg
}
