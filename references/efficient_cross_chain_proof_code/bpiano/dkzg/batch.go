package dkzg

import (
	"crypto/sha256"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// BatchOpenResult 保存单个 (alpha, beta) 点在全部 K 个证明实例上的聚合数据。
type BatchOpenResult struct {
	ComF   Digest         // 全局承诺
	Alpha  fr.Element     // X 轴求值点
	Beta   fr.Element     // Y 轴求值点
	PiXAgg bn254.G1Affine // 聚合后的 X 轴商多项式证明
	ProofY OpeningProofY  // Y 轴证明（包含 z、π_{1,F}、com_VF）
}

// BatchVerify 使用随机线性组合同时验证 K 个 DKZG 开放证明，
// 将 K×2 次 pairing 校验压缩为 2 次。
//
// 给定 K 个证明，每个有自己的 (comF_k, alpha_k, beta_k, piX_k, proofY_k)，
// BatchVerify 抽取随机标量 r，检查：
//
//	Σ_k r^k · [X 轴方程_k] = 0  （X 轴批量）
//	Σ_k r^k · [Y 轴方程_k] = 0  （Y 轴批量）
//
// X 轴方程为：
//
//	e(Σ_k r^k·(comF_k - comVF_k), g2) = e(Σ_k r^k·piX_k, g2^{τ_X}) · ...
//
// 实际上通过选取随机 r 后验证：
//
//	e(lhsX, g2) = e(rhsX, g2^{τ_X})
//
// 单证明方程参见 Verify。
//
// randomness 是用于生成 r^k 的非零域元素，实践中应由 Fiat-Shamir 哈希派生，
// 此处由调用方提供。
func BatchVerify(proofs []BatchOpenResult, randomness fr.Element, srs *SRS) error {
	K := len(proofs)
	if K == 0 {
		return nil
	}
	if K == 1 {
		return Verify(proofs[0].ComF, proofs[0].Alpha, proofs[0].Beta,
			proofs[0].PiXAgg, proofs[0].ProofY, srs)
	}

	_, _, g1Aff, _ := bn254.Generators()

	// 构建随机幂次：rPow[k] = randomness^k。
	rPow := make([]fr.Element, K)
	rPow[0].SetOne()
	for k := 1; k < K; k++ {
		rPow[k].Mul(&rPow[k-1], &randomness)
	}

	// 对不同 alpha_k 和 beta_k 的一般情形处理：
	// 标准批量技巧按求值点分组，此处每个证明可有不同的求值点。
	//
	// 做法：对第 k 个证明，X 轴方程为：
	//   (comF_k - comVF_k) = (τ_X - alpha_k) · piX_k
	//
	// 改写：(comF_k - comVF_k) - τ_X·piX_k + alpha_k·piX_k = 0
	//
	// 批量化：Σ_k r^k·[(comF_k - comVF_k) - τ_X·piX_k + alpha_k·piX_k] = 0
	// => e(Σ_k r^k·(comF_k - comVF_k + alpha_k·piX_k), g2) = e(Σ_k r^k·piX_k, g2^{τ_X})
	//
	// Y 轴类似。

	// 累积 X 轴的 LHS 和 RHS。
	var lhsXJac, rhsXJac bn254.G1Jac
	for k := 0; k < K; k++ {
		proof := proofs[k]

		var alphaBig big.Int
		proof.Alpha.BigInt(&alphaBig)

		// r^k · piX_k。
		var rPowBig big.Int
		rPow[k].BigInt(&rPowBig)

		var piXScaled bn254.G1Affine
		piXScaled.ScalarMultiplication(&proof.PiXAgg, &rPowBig)

		// RHS 累积：Σ r^k · piX_k。
		var piXScaledJac bn254.G1Jac
		piXScaledJac.FromAffine(&piXScaled)
		rhsXJac.AddAssign(&piXScaledJac)

		// (comF_k - comVF_k) · r^k。
		var diffScaledJac bn254.G1Jac
		{
			var comFJac, comVFJac bn254.G1Jac
			comFJac.FromAffine(&proof.ComF)
			comVFJac.FromAffine(&proof.ProofY.ComVF)
			var diffJac bn254.G1Jac
			diffJac.Set(&comFJac)
			diffJac.SubAssign(&comVFJac)
			var diffAff bn254.G1Affine
			diffAff.FromJacobian(&diffJac)
			var diffScaled bn254.G1Affine
			diffScaled.ScalarMultiplication(&diffAff, &rPowBig)
			diffScaledJac.FromAffine(&diffScaled)
		}
		// alpha_k · piX_k · r^k。
		var alphaPiXRaw bn254.G1Affine
		alphaPiXRaw.ScalarMultiplication(&proof.PiXAgg, &alphaBig)
		alphaPiXRaw.ScalarMultiplication(&alphaPiXRaw, &rPowBig)
		var alphaPiXRawJac bn254.G1Jac
		alphaPiXRawJac.FromAffine(&alphaPiXRaw)

		diffScaledJac.AddAssign(&alphaPiXRawJac)
		lhsXJac.AddAssign(&diffScaledJac)
	}

	// X 轴 pairing 校验：
	// e(lhsX, g2) = e(rhsX, g2^{τ_X})
	// ⟺ e(lhsX, g2) · e(-rhsX, g2^{τ_X}) = 1。
	var lhsX, rhsX bn254.G1Affine
	lhsX.FromJacobian(&lhsXJac)
	rhsX.FromJacobian(&rhsXJac)

	ok1, err := bn254.PairingCheck(
		[]bn254.G1Affine{lhsX, negG1(rhsX)},
		[]bn254.G2Affine{srs.G2[0], srs.G2[1]},
	)
	if err != nil {
		return err
	}
	if !ok1 {
		return ErrVerification
	}

	// 累积 Y 轴批量。
	// 第 k 个证明的 Y 轴方程：
	//   (comVF_k - z_k·g1) = (τ_Y - beta_k) · piY_k
	//   comVF_k - z_k·g1 - τ_Y·piY_k + beta_k·piY_k = 0
	// 批量化：e(Σ_k r^k·(comVF_k - z_k·g1 + beta_k·piY_k), g2) = e(Σ_k r^k·piY_k, g2^{τ_Y})
	var lhsYJac, rhsYJac bn254.G1Jac
	for k := 0; k < K; k++ {
		proof := proofs[k]

		var rPowBig big.Int
		rPow[k].BigInt(&rPowBig)

		// piY_k · r^k → 累积到 RHS。
		var piYScaled bn254.G1Affine
		piYScaled.ScalarMultiplication(&proof.ProofY.H, &rPowBig)
		var piYScaledJac bn254.G1Jac
		piYScaledJac.FromAffine(&piYScaled)
		rhsYJac.AddAssign(&piYScaledJac)

		// comVF_k - z_k·g1。
		var zBig big.Int
		proof.ProofY.ClaimedValue.BigInt(&zBig)
		var zG1 bn254.G1Affine
		zG1.ScalarMultiplication(&g1Aff, &zBig)
		var diffYJac, comVFJac, zG1Jac bn254.G1Jac
		comVFJac.FromAffine(&proof.ProofY.ComVF)
		zG1Jac.FromAffine(&zG1)
		diffYJac.Set(&comVFJac)
		diffYJac.SubAssign(&zG1Jac)

		// (comVF_k - z_k·g1) · r^k。
		var diffYAff bn254.G1Affine
		diffYAff.FromJacobian(&diffYJac)
		var diffYScaled bn254.G1Affine
		diffYScaled.ScalarMultiplication(&diffYAff, &rPowBig)
		var diffYScaledJac bn254.G1Jac
		diffYScaledJac.FromAffine(&diffYScaled)

		// beta_k · piY_k · r^k。
		var betaBig big.Int
		proof.Beta.BigInt(&betaBig)
		var betaPiY bn254.G1Affine
		betaPiY.ScalarMultiplication(&proof.ProofY.H, &betaBig)
		betaPiY.ScalarMultiplication(&betaPiY, &rPowBig)
		var betaPiYJac bn254.G1Jac
		betaPiYJac.FromAffine(&betaPiY)

		diffYScaledJac.AddAssign(&betaPiYJac)
		lhsYJac.AddAssign(&diffYScaledJac)
	}

	var lhsY, rhsY bn254.G1Affine
	lhsY.FromJacobian(&lhsYJac)
	rhsY.FromJacobian(&rhsYJac)

	ok2, err := bn254.PairingCheck(
		[]bn254.G1Affine{lhsY, negG1(rhsY)},
		[]bn254.G2Affine{srs.G2Y[0], srs.G2Y[1]},
	)
	if err != nil {
		return err
	}
	if !ok2 {
		return ErrVerification
	}

	return nil
}

// PrepareProof 是一个辅助函数，对单个实例运行完整的证明协议，
// 返回可供 BatchVerify 使用的 BatchOpenResult。
func PrepareProof(
	allEvals [][]fr.Element,
	alpha, beta fr.Element,
	srs *SRS,
) (BatchOpenResult, error) {
	M := uint64(len(srs.Ux))

	comF, err := CommitGlobal(allEvals, srs)
	if err != nil {
		return BatchOpenResult{}, err
	}

	localProofs := make([]OpeningProofX, M)
	for i := uint64(0); i < M; i++ {
		localProofs[i], err = LocalOpenX(i, allEvals[i], alpha, srs)
		if err != nil {
			return BatchOpenResult{}, err
		}
	}

	_, piXAgg, err := AggregateOpenX(localProofs, srs)
	if err != nil {
		return BatchOpenResult{}, err
	}

	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i] = localProofs[i].ClaimedValue
	}

	proofY, err := OpenY(viEvals, beta, srs)
	if err != nil {
		return BatchOpenResult{}, err
	}

	return BatchOpenResult{
		ComF:   comF,
		Alpha:  alpha,
		Beta:   beta,
		PiXAgg: piXAgg,
		ProofY: proofY,
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 批量折叠类型
// ────────────────────────────────────────────────────────────────────────────

// BatchedProofX 是 K 个多项式在单一求值点处的折叠 X 轴批量开放证明。
// 由 BatchOpenX 生成，由 VerifyBatchedAndAggregatedX 验证。
type BatchedProofX struct {
	H              bn254.G1Affine   // 折叠商多项式：Σ_k γ^k · piXAgg_k
	ClaimedDigests []bn254.G1Affine // K 个 comVF 值，每个多项式一个
}

// AggregatedProofX 是单个多项式跨 M 个子节点聚合后的 X 轴开放证明。
type AggregatedProofX struct {
	H     bn254.G1Affine // Σ_i π_{0,i}（聚合后的 X 轴商多项式）
	ComVF bn254.G1Affine // Σ_i v_i · Vy[i]（Y 轴多项式承诺）
}

// BatchedProofY 是 K 个 Y 轴多项式在 β 处的折叠 Y 轴批量开放证明。
// 由 BatchOpenY 生成，由 VerifyBatchedProofY 验证。
type BatchedProofY struct {
	H             bn254.G1Affine // 折叠 Y 轴商多项式
	ClaimedValues []fr.Element   // K 个标量求值（在 β 处）
}

// deriveBatchGamma 通过对 (point, comFs...) 进行哈希派生批量折叠随机数 γ。
func deriveBatchGamma(point fr.Element, comFs []Digest) fr.Element {
	h := sha256.New()
	pointBytes := point.Bytes()
	h.Write(pointBytes[:])
	for i := range comFs {
		b := comFs[i].Bytes()
		h.Write(b[:])
	}
	var gamma fr.Element
	gamma.SetBytes(h.Sum(nil))
	if gamma.IsZero() {
		gamma.SetOne()
	}
	return gamma
}

// buildGammaPow 计算 [γ^0, γ^1, ..., γ^{K-1}]。
func buildGammaPow(gamma fr.Element, K int) []fr.Element {
	pows := make([]fr.Element, K)
	pows[0].SetOne()
	for k := 1; k < K; k++ {
		pows[k].Mul(&pows[k-1], &gamma)
	}
	return pows
}

// AggregateProofX 将 M 个本地 X 轴开放证明聚合为一个 AggregatedProofX。
func AggregateProofX(localProofs []OpeningProofX, srs *SRS) (AggregatedProofX, error) {
	comVF, piX, err := AggregateOpenX(localProofs, srs)
	if err != nil {
		return AggregatedProofX{}, err
	}
	return AggregatedProofX{H: piX, ComVF: comVF}, nil
}

// BatchOpenX 使用 Fiat-Shamir 随机数 γ，将 K 个聚合 X 轴证明折叠为单个 BatchedProofX。
//
// aggProofs[k] 是第 k 个多项式的 AggregatedProofX。
// comFs[k] 是第 k 个多项式的全局承诺。
// gamma 由 (alpha, comFs) 派生。
//
// 返回的 BatchedProofX 包含：
//   - H = Σ_k γ^k · aggProofs[k].H
//   - ClaimedDigests[k] = aggProofs[k].ComVF
func BatchOpenX(aggProofs []AggregatedProofX, comFs []Digest, alpha fr.Element) (BatchedProofX, error) {
	K := len(aggProofs)
	if K == 0 || len(comFs) != K {
		return BatchedProofX{}, ErrMismatchedInputs
	}

	gamma := deriveBatchGamma(alpha, comFs)
	gammaPow := buildGammaPow(gamma, K)

	Hs := make([]bn254.G1Affine, K)
	claimedDigests := make([]bn254.G1Affine, K)
	for k := 0; k < K; k++ {
		Hs[k] = aggProofs[k].H
		claimedDigests[k] = aggProofs[k].ComVF
	}

	var foldedH bn254.G1Affine
	if _, err := foldedH.MultiExp(Hs, gammaPow, ecc.MultiExpConfig{}); err != nil {
		return BatchedProofX{}, err
	}

	return BatchedProofX{H: foldedH, ClaimedDigests: claimedDigests}, nil
}

// BatchOpenY 将 K 个 Y 轴多项式的求值折叠为在 beta 处的单个 Y 轴开放证明。
//
// comVFs[k] 是第 k 个 Y 轴多项式的承诺。
// yPolysEvals[k] 是第 k 个多项式的 Lagrange 求值向量（长度为 M）。
// gamma 由 (beta, comVFs) 派生。
func BatchOpenY(comVFs []Digest, yPolysEvals [][]fr.Element, beta fr.Element, srs *SRS) (BatchedProofY, error) {
	K := len(yPolysEvals)
	M := uint64(len(srs.Vy))
	if K == 0 || len(comVFs) != K {
		return BatchedProofY{}, ErrMismatchedInputs
	}

	lagBeta, err := evalLagrangeBasis(beta, M, srs.DomainY.Generator)
	if err != nil {
		return BatchedProofY{}, ErrBetaIsRootOfUnityY
	}

	// 计算每个 Y 多项式在 beta 处的声明值。
	claimedValues := make([]fr.Element, K)
	for k := 0; k < K; k++ {
		if uint64(len(yPolysEvals[k])) != M {
			return BatchedProofY{}, ErrInvalidEvalsSize
		}
		for i := uint64(0); i < M; i++ {
			var term fr.Element
			term.Mul(&yPolysEvals[k][i], &lagBeta[i])
			claimedValues[k].Add(&claimedValues[k], &term)
		}
	}

	gamma := deriveBatchGamma(beta, comVFs)
	gammaPow := buildGammaPow(gamma, K)

	// 将 K 个多项式按 γ 权重折叠为单个求值向量。
	foldedEvals := make([]fr.Element, M)
	for k := 0; k < K; k++ {
		for i := uint64(0); i < M; i++ {
			var term fr.Element
			term.Mul(&yPolysEvals[k][i], &gammaPow[k])
			foldedEvals[i].Add(&foldedEvals[i], &term)
		}
	}

	foldedProof, err := OpenY(foldedEvals, beta, srs)
	if err != nil {
		return BatchedProofY{}, err
	}

	return BatchedProofY{H: foldedProof.H, ClaimedValues: claimedValues}, nil
}

// VerifyBatchedAndAggregatedX 用单次 2-pairing 校验验证两个不同求值点处的 X 轴证明，
// 将以下两个验证合并：
//
//  1. 在 alpha 处的 K 多项式批量证明：
//     e(foldedComF - foldedComVF + α·H_batch, g2) = e(H_batch, g2^{τX})
//
//  2. 在 alphaShifted（即 Z 在 ω·α 处）的单个聚合证明：
//     e(singleComF - singleProof.ComVF + ω·α·H_ZS, g2) = e(H_ZS, g2^{τX})
//
// 两个校验通过从证明数据派生的随机数 r 组合。
func VerifyBatchedAndAggregatedX(
	batchComFs []Digest, batchProof BatchedProofX, alpha fr.Element,
	singleComF Digest, singleProof AggregatedProofX, alphaShifted fr.Element,
	srs *SRS,
) error {
	K := len(batchComFs)
	if K == 0 || len(batchProof.ClaimedDigests) != K {
		return ErrMismatchedInputs
	}

	// 重新派生 γ 并折叠批量承诺。
	gamma := deriveBatchGamma(alpha, batchComFs)
	gammaPow := buildGammaPow(gamma, K)

	var foldedComF Digest
	if _, err := foldedComF.MultiExp(batchComFs, gammaPow, ecc.MultiExpConfig{}); err != nil {
		return err
	}
	var foldedComVF bn254.G1Affine
	if _, err := foldedComVF.MultiExp(batchProof.ClaimedDigests, gammaPow, ecc.MultiExpConfig{}); err != nil {
		return err
	}

	// 派生随机数 r 以组合两个方程。
	r := deriveBatchGamma(alpha, []Digest{foldedComF, singleComF})

	var alphaBig, alphaShiftedBig, rBig big.Int
	alpha.BigInt(&alphaBig)
	alphaShifted.BigInt(&alphaShiftedBig)
	r.BigInt(&rBig)

	// LHS1 = foldedComF - foldedComVF + α·H_batch
	var lhs1Jac bn254.G1Jac
	{
		var a, b, c bn254.G1Jac
		a.FromAffine(&foldedComF)
		b.FromAffine(&foldedComVF)
		var alphaH bn254.G1Affine
		alphaH.ScalarMultiplication(&batchProof.H, &alphaBig)
		c.FromAffine(&alphaH)
		lhs1Jac.Set(&a)
		lhs1Jac.SubAssign(&b)
		lhs1Jac.AddAssign(&c)
	}

	// LHS2 = singleComF - singleProof.ComVF + ω·α·H_ZS
	var lhs2Jac bn254.G1Jac
	{
		var a, b, c bn254.G1Jac
		a.FromAffine(&singleComF)
		b.FromAffine(&singleProof.ComVF)
		var shiftedH bn254.G1Affine
		shiftedH.ScalarMultiplication(&singleProof.H, &alphaShiftedBig)
		c.FromAffine(&shiftedH)
		lhs2Jac.Set(&a)
		lhs2Jac.SubAssign(&b)
		lhs2Jac.AddAssign(&c)
	}

	// LHS = LHS1 + r·LHS2
	{
		var lhs2Aff bn254.G1Affine
		lhs2Aff.FromJacobian(&lhs2Jac)
		var rLHS2 bn254.G1Affine
		rLHS2.ScalarMultiplication(&lhs2Aff, &rBig)
		var rLHS2Jac bn254.G1Jac
		rLHS2Jac.FromAffine(&rLHS2)
		lhs1Jac.AddAssign(&rLHS2Jac)
	}
	var lhs bn254.G1Affine
	lhs.FromJacobian(&lhs1Jac)

	// RHS = H_batch + r·H_ZS
	var rhsJac bn254.G1Jac
	{
		rhsJac.FromAffine(&batchProof.H)
		var rHZS bn254.G1Affine
		rHZS.ScalarMultiplication(&singleProof.H, &rBig)
		var rHZSJac bn254.G1Jac
		rHZSJac.FromAffine(&rHZS)
		rhsJac.AddAssign(&rHZSJac)
	}
	var rhs bn254.G1Affine
	rhs.FromJacobian(&rhsJac)

	// e(LHS, G2) = e(RHS, G2^{τX})
	// ⟺ e(LHS, G2) · e(-RHS, G2^{τX}) = 1
	ok, err := bn254.PairingCheck(
		[]bn254.G1Affine{lhs, negG1(rhs)},
		[]bn254.G2Affine{srs.G2[0], srs.G2[1]},
	)
	if err != nil {
		return err
	}
	if !ok {
		return ErrVerification
	}
	return nil
}

// VerifyBatchedProofY 在 beta 处验证折叠后的 Y 轴批量开放证明。
//
// comVFs[k] 是 Y 轴承诺（comVF_k = Σ_i v_ki · Vy[i]）。
// proof.ClaimedValues[k] 是在 beta 处的声明标量求值。
// 校验：e(foldedComVF - foldedValue·G1, G2_Y) = e(H, [τY - β]_2)
func VerifyBatchedProofY(comVFs []Digest, proof BatchedProofY, beta fr.Element, srs *SRS) error {
	K := len(comVFs)
	if K == 0 || len(proof.ClaimedValues) != K {
		return ErrMismatchedInputs
	}

	gamma := deriveBatchGamma(beta, comVFs)
	gammaPow := buildGammaPow(gamma, K)

	var foldedComVF Digest
	if _, err := foldedComVF.MultiExp(comVFs, gammaPow, ecc.MultiExpConfig{}); err != nil {
		return err
	}

	// 折叠声明值：foldedValue = Σ_k γ^k · claimedValues[k]。
	var foldedValue fr.Element
	for k := 0; k < K; k++ {
		var term fr.Element
		term.Mul(&gammaPow[k], &proof.ClaimedValues[k])
		foldedValue.Add(&foldedValue, &term)
	}

	_, _, g1Aff, _ := bn254.Generators()
	var fvBig big.Int
	foldedValue.BigInt(&fvBig)
	var fvG1 bn254.G1Affine
	fvG1.ScalarMultiplication(&g1Aff, &fvBig)

	// 计算 foldedComVF - foldedValue·g1。
	var diffJac, comVFJac, fvG1Jac bn254.G1Jac
	comVFJac.FromAffine(&foldedComVF)
	fvG1Jac.FromAffine(&fvG1)
	diffJac.Set(&comVFJac)
	diffJac.SubAssign(&fvG1Jac)
	var diff bn254.G1Affine
	diff.FromJacobian(&diffJac)

	// 计算 [τ_Y - beta]_2。
	var betaBig big.Int
	beta.BigInt(&betaBig)
	var betaG2Y bn254.G2Affine
	betaG2Y.ScalarMultiplication(&srs.G2Y[0], &betaBig)

	var tauYMinusBeta bn254.G2Affine
	{
		var jac, tauYJac, betaJac bn254.G2Jac
		tauYJac.FromAffine(&srs.G2Y[1])
		betaJac.FromAffine(&betaG2Y)
		jac.Set(&tauYJac)
		jac.SubAssign(&betaJac)
		tauYMinusBeta.FromJacobian(&jac)
	}

	ok, err := bn254.PairingCheck(
		[]bn254.G1Affine{diff, negG1(proof.H)},
		[]bn254.G2Affine{srs.G2Y[0], tauYMinusBeta},
	)
	if err != nil {
		return err
	}
	if !ok {
		return ErrVerification
	}
	return nil
}
