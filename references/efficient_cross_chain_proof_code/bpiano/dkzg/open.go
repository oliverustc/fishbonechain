package dkzg

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// OpeningProofX 是子节点生成的 X 轴开放证明。
// 证明已承诺多项式 f_i 满足 f_i(alpha) = ClaimedValue。
type OpeningProofX struct {
	// ClaimedValue 是声明值 v_i = f_i(alpha) ∈ Fr。
	ClaimedValue fr.Element
	// H 是商多项式承诺 π_{0,i} = Σ_j q_j · Ux[i][j]，
	// 其中 q_j = (f_i(ω^j) - v_i) / (ω^j - alpha)。
	H bn254.G1Affine
}

// OpeningProofY 是主节点生成的 Y 轴开放证明。
// 证明 F(τ_Y, alpha) = z，其中 V_F(Y) = Σ_i v_i · R_i(Y)。
type OpeningProofY struct {
	// ClaimedValue 是声明值 z = F(beta, alpha) = V_F(beta) ∈ Fr。
	ClaimedValue fr.Element
	// H 是商多项式承诺 π_{1,F}。
	H bn254.G1Affine
	// ComVF 是 Y 轴多项式 V_F(Y) = Σ_i v_i · R_i(Y) 的承诺。
	// 验证方需要此值同时完成 X 轴和 Y 轴的验证。
	ComVF bn254.G1Affine
}

// LocalOpenX 计算子节点 i 在求值点 alpha 处的 X 轴开放证明。
//
// 给定 Lagrange 形式的 evals[j] = f_i(ω_X^j)，依次执行：
//
//  1. v_i = f_i(alpha) = Σ_j evals[j] · L_j(alpha)   （Lagrange 求值）
//  2. q_j = (evals[j] - v_i) / (ω_X^j - alpha)        （逐点商系数）
//  3. π_{0,i} = Σ_j q_j · Ux[nodeIdx][j]               （商多项式承诺）
//
// 若 alpha 属于 X 轴域，则返回 ErrAlphaIsRootOfUnityX。
func LocalOpenX(nodeIdx uint64, evals []fr.Element, alpha fr.Element, srs *SRS) (OpeningProofX, error) {
	M := uint64(len(srs.Ux))
	if nodeIdx >= M {
		return OpeningProofX{}, ErrInvalidNodeIndex
	}
	T := uint64(len(srs.Ux[nodeIdx]))
	if uint64(len(evals)) != T {
		return OpeningProofX{}, ErrInvalidPolySize
	}

	// 步骤 1：通过 Lagrange 插值计算 f_i(alpha)。
	lagAlpha, err := evalLagrangeBasis(alpha, T, srs.DomainX.Generator)
	if err != nil {
		// alpha 是单位根 —— 按协议规范视为错误处理。
		return OpeningProofX{}, ErrAlphaIsRootOfUnityX
	}

	var claimedValue fr.Element
	for j := uint64(0); j < T; j++ {
		var term fr.Element
		term.Mul(&evals[j], &lagAlpha[j])
		claimedValue.Add(&claimedValue, &term)
	}

	// 步骤 2：在 Lagrange 基下计算商系数。
	// q_j = (evals[j] - claimedValue) / (ω_X^j - alpha)
	//
	// 注：因 alpha 不是单位根，所以 ω_X^j - alpha ≠ 0。
	quotients := make([]fr.Element, T)
	gen := srs.DomainX.Generator
	var genPow fr.Element
	genPow.SetOne() // ω^0 = 1
	denoms := make([]fr.Element, T)
	for j := uint64(0); j < T; j++ {
		var numer fr.Element
		numer.Sub(&evals[j], &claimedValue)
		quotients[j] = numer          // 分子，分母在下方处理
		denoms[j].Sub(&genPow, &alpha) // ω^j - alpha
		genPow.Mul(&genPow, &gen)
	}
	denoms = fr.BatchInvert(denoms)
	for j := uint64(0); j < T; j++ {
		quotients[j].Mul(&quotients[j], &denoms[j])
	}

	// 步骤 3：对商多项式进行承诺。
	var h bn254.G1Affine
	if _, err := h.MultiExp(srs.Ux[nodeIdx], quotients, ecc.MultiExpConfig{}); err != nil {
		return OpeningProofX{}, err
	}

	return OpeningProofX{
		ClaimedValue: claimedValue,
		H:            h,
	}, nil
}

// AggregateOpenX 将 M 个子节点的 X 轴开放证明聚合为全局证明。
//
// 主节点执行：
//  1. 构建 V_F(Y) = Σ_i v_i · R_i(Y) 并计算承诺：com_VF = Σ_i v_i · Vy[i]
//  2. 聚合 X 轴商多项式证明：π_{0,F} = Σ_i π_{0,i}
//
// 返回 (com_VF, 聚合后的证明)。
func AggregateOpenX(localProofs []OpeningProofX, srs *SRS) (comVF bn254.G1Affine, piX bn254.G1Affine, err error) {
	M := uint64(len(srs.Ux))
	if uint64(len(localProofs)) != M {
		return comVF, piX, ErrMismatchedInputs
	}

	// 收集各子节点的声明值 v_i。
	claimedValues := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		claimedValues[i] = localProofs[i].ClaimedValue
	}

	// com_VF = Σ_i v_i · Vy[i]（在 Y 轴基上做 MSM）。
	if _, err = comVF.MultiExp(srs.Vy, claimedValues, ecc.MultiExpConfig{}); err != nil {
		return comVF, piX, err
	}

	// π_{0,F} = Σ_i π_{0,i}（G1 中各商承诺求和）。
	var piXJac bn254.G1Jac
	for i := uint64(0); i < M; i++ {
		var pt bn254.G1Jac
		pt.FromAffine(&localProofs[i].H)
		piXJac.AddAssign(&pt)
	}
	piX.FromJacobian(&piXJac)

	return comVF, piX, nil
}

// OpenY 计算在求值点 beta 处的 Y 轴开放证明。
//
// 主节点将 {v_i} 视为 V_F(Y) 的 Lagrange 求值：
//
//	V_F(ω_Y^i) = v_i = f_i(alpha)
//
// 并在 beta 处打开 V_F：
//
//  1. z = V_F(beta) = Σ_i v_i · R_i(beta)
//  2. r_i = (v_i - z) / (ω_Y^i - beta)   （Y 轴商系数）
//  3. π_{1,F} = Σ_i r_i · Vy[i]
//
// 若 beta 属于 Y 轴域，则返回 ErrBetaIsRootOfUnityY。
func OpenY(claimedValues []fr.Element, beta fr.Element, srs *SRS) (OpeningProofY, error) {
	M := uint64(len(srs.Vy))
	if uint64(len(claimedValues)) != M {
		return OpeningProofY{}, ErrInvalidEvalsSize
	}

	// 步骤 1：通过 Lagrange 插值计算 V_F(beta)。
	lagBeta, err := evalLagrangeBasis(beta, M, srs.DomainY.Generator)
	if err != nil {
		return OpeningProofY{}, ErrBetaIsRootOfUnityY
	}

	var z fr.Element
	for i := uint64(0); i < M; i++ {
		var term fr.Element
		term.Mul(&claimedValues[i], &lagBeta[i])
		z.Add(&z, &term)
	}

	// 步骤 2：计算 Y 轴商系数。
	// r_i = (v_i - z) / (ω_Y^i - beta)
	quotients := make([]fr.Element, M)
	gen := srs.DomainY.Generator
	var genPow fr.Element
	genPow.SetOne() // ω_Y^0 = 1
	denoms := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		quotients[i].Sub(&claimedValues[i], &z)
		denoms[i].Sub(&genPow, &beta) // ω_Y^i - beta
		genPow.Mul(&genPow, &gen)
	}
	denoms = fr.BatchInvert(denoms)
	for i := uint64(0); i < M; i++ {
		quotients[i].Mul(&quotients[i], &denoms[i])
	}

	// 步骤 3：使用 Vy 基对 Y 轴商多项式进行承诺。
	var piY bn254.G1Affine
	if _, err := piY.MultiExp(srs.Vy, quotients, ecc.MultiExpConfig{}); err != nil {
		return OpeningProofY{}, err
	}

	// 构建 com_VF = Σ_i v_i · Vy[i]。
	var comVF bn254.G1Affine
	if _, err := comVF.MultiExp(srs.Vy, claimedValues, ecc.MultiExpConfig{}); err != nil {
		return OpeningProofY{}, err
	}

	return OpeningProofY{
		ClaimedValue: z,
		H:            piY,
		ComVF:        comVF,
	}, nil
}

// evalPolyLagrange 在任意点处对 Lagrange 形式的多项式进行求值。
//
// evals[j] = p(ω^j)，alpha 为求值点（不能是单位根）。
// 返回 p(alpha) = Σ_j evals[j] · L_j(alpha)。
func evalPolyLagrange(evals []fr.Element, alpha fr.Element, gen fr.Element) (fr.Element, error) {
	N := uint64(len(evals))
	lagAlpha, err := evalLagrangeBasis(alpha, N, gen)
	if err != nil {
		return fr.Element{}, err
	}
	var result fr.Element
	for j := uint64(0); j < N; j++ {
		var term fr.Element
		term.Mul(&evals[j], &lagAlpha[j])
		result.Add(&result, &term)
	}
	return result, nil
}

// newBigFromFr 将 fr.Element 转换为 *big.Int（用于标量乘法）。
func newBigFromFr(e fr.Element) *big.Int {
	b := new(big.Int)
	e.BigInt(b)
	return b
}
