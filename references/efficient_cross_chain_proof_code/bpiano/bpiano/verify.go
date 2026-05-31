package bpiano

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/piano"
)

// VerifyCompressed 通过 4 次配对验证 BPiano 压缩证明。
//
// X 轴 Shplonk 验证：Q_X(τX)·Z_T(τX) = C1·(τX-ωα) + C2·(τX-α)，等价于
//
//	e(ComQX, ZTG2) = e(C1, [τX-ωα]_2) · e(C2, [τX-α]_2)
//	               = e(C1+C2, G2[1]) · e(-ωα·C1 - α·C2, G2[0])
//
// Y 轴 KZG（缩放因子 ρ）：e(ρ·Pi1AggH, [τY-β]_2) = e(ρ(ComGY-gYβ·g1), G2[0])
//
// 合并配对验证（乘积为 1）：
//
//	e(ComQX, [Z_T(τ_X)]_2)
//	· e(ρ·Pi1AggH, [τ_Y-β]_2)
//	· e(-(C1+C2), G2[1])
//	· e(ωα·C1 + α·C2 - ρ·(ComGY-[G_Y(β)]_1), G2[0]) = 1
func VerifyCompressed(proof *CompressedProof, vk *piano.VerifyingKey, publicInputs [][]fr.Element) error {
	T := vk.SizeX
	M := vk.SizeY
	srs := vk.DKZGSRS

	// ── 1. 重放 Fiat-Shamir 挑战 ─────────────────────────────────────────────
	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc,
		"gamma", "eta", "lambda", "alpha", "nu", "beta", "mu")

	if err := bindPublicDataBP(fs, "gamma", vk, publicInputs); err != nil {
		return err
	}
	gamma, err := deriveChallengeBP(fs, "gamma",
		[]dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
	if err != nil {
		return err
	}
	eta, err := deriveChallengeBP(fs, "eta", nil)
	if err != nil {
		return err
	}
	lambda, err := deriveChallengeBP(fs, "lambda", []dkzg.Digest{proof.Z})
	if err != nil {
		return err
	}
	alpha, err := deriveChallengeBP(fs, "alpha",
		[]dkzg.Digest{proof.Hx[0], proof.Hx[1], proof.Hx[2]})
	if err != nil {
		return err
	}

	alphaPowT := new(fr.Element)
	alphaPowT.Exp(alpha, new(big.Int).SetUint64(T))
	foldedHxDig := piano.FoldDigests(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)

	nu, err := deriveChallengeBP(fs, "nu",
		[]dkzg.Digest{foldedHxDig, proof.LRO[0], proof.LRO[1], proof.LRO[2], proof.Z})
	if err != nil {
		return err
	}
	beta, err := deriveChallengeBP(fs, "beta", []dkzg.Digest{proof.ComQX, proof.ComVFAlpha, proof.ComVFZS})
	if err != nil {
		return err
	}
	mu, err := deriveChallengeBP(fs, "mu", nil)
	if err != nil {
		return err
	}

	alphaShifted := new(fr.Element).Mul(&alpha, &vk.GeneratorX)

	// ── 2. 代数约束验证 ──────────────────────────────────────────────────────
	if err := verifyAlgebraicConstraintBP(proof, alpha, beta, lambda, eta, gamma, T, M, vk); err != nil {
		return err
	}

	// ── 3. 派生 ρ ─────────────────────────────────────────────────────────────
	rho := deriveRhoBP(proof)

	// ── 4. G1 生成元 ──────────────────────────────────────────────────────────
	_, _, g1Gen, _ := bn254.Generators()

	// ── 5. 构建 C1 和 C2 ──────────────────────────────────────────────────────
	nuPow := buildNuPow(nu, 14)

	sAlphaComs := []dkzg.Digest{
		foldedHxDig, proof.LRO[0], proof.LRO[1], proof.LRO[2], proof.Z,
		vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3,
	}
	sAlphaEvals := []fr.Element{
		proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO, proof.EvalZ,
		proof.EvalQl, proof.EvalQr, proof.EvalQm, proof.EvalQo, proof.EvalQk,
		proof.EvalS1, proof.EvalS2, proof.EvalS3,
	}

	// C1 = Σ_{k=0..12} ν^k · (com_k - eval_k·g1)
	c1 := computeC(sAlphaComs, sAlphaEvals, nuPow[:13], g1Gen)

	// C2 = ν^13 · (com_Z - evalZS·g1)
	c2 := computeC(
		[]dkzg.Digest{proof.Z},
		[]fr.Element{proof.EvalZS},
		[]fr.Element{nuPow[13]},
		g1Gen,
	)

	// 修正：对于 witness 多项式（k=0..4：fhx,A,B,O,Z），Shplonk 要求
	// com_VF_k = CommitY(evalVec_k)，一般情况下 ≠ evalK·g1（当 τ_Y ≠ β 时二者不同）。
	// c1_correct = c1_wrong + evalAlphaCorr·g1 - ComVFAlpha
	// c2_correct = c2_wrong + ν^13·evalZS·g1   - ν^13·ComVFZS
	{
		// evalAlphaCorr = Σ_{k=0}^{4} ν^k · evalK
		var evalAlphaCorr fr.Element
		for k, e := range []fr.Element{
			proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO, proof.EvalZ,
		} {
			var t fr.Element
			t.Mul(&nuPow[k], &e)
			evalAlphaCorr.Add(&evalAlphaCorr, &t)
		}
		var evalCorrG1 bn254.G1Jac
		evalCorrG1.FromAffine(&g1Gen)
		mulJacByFr(&evalCorrG1, evalAlphaCorr)
		var comVFAlphaJac bn254.G1Jac
		comVFAlphaJac.FromAffine(&proof.ComVFAlpha)
		c1.AddAssign(&evalCorrG1)
		c1.SubAssign(&comVFAlphaJac)

		// ν^13·evalZS 对 C2 的修正
		var nuZSEvalZS fr.Element
		nuZSEvalZS.Mul(&nuPow[13], &proof.EvalZS)
		var nuZSEvalZSG1 bn254.G1Jac
		nuZSEvalZSG1.FromAffine(&g1Gen)
		mulJacByFr(&nuZSEvalZSG1, nuZSEvalZS)
		var comVFZSScaled bn254.G1Jac
		comVFZSScaled.FromAffine(&proof.ComVFZS)
		mulJacByFr(&comVFZSScaled, nuPow[13])
		c2.AddAssign(&nuZSEvalZSG1)
		c2.SubAssign(&comVFZSScaled)
	}

	// ── 6. [Z_T(τ_X)]_2 = G2[2] - (α+ω·α)·G2[1] + α·ω·α·G2[0] ─────────────
	zTG2 := computeZTG2(alpha, *alphaShifted, srs)

	// ── 7. [τ_Y-β]_2 = G2Y[1] - β·G2Y[0] ────────────────────────────────────
	tauYBetaG2 := computeTauYMinusBetaG2(beta, srs)

	// ── 8. G_Y(β) = Σ_k μ^k · yEvals[k] ─────────────────────────────────────
	muPow := buildNuPow(mu, 7)
	yEvals := []fr.Element{
		proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO,
		proof.EvalZ, proof.EvalZS, proof.EvalHy,
	}
	var gYBeta fr.Element
	for k, e := range yEvals {
		var term fr.Element
		term.Mul(&muPow[k], &e)
		gYBeta.Add(&gYBeta, &term)
	}

	// ── 9. 组装 4 个 G1 点 ────────────────────────────────────────────────────
	// P0 = ComQX                                          与 zTG2 配对
	// P1 = ρ·Pi1AggH                                      与 tauYBetaG2 配对
	// P2 = -D_lin = -(ωα·C1 + α·C2 + ρ·(ComGY-gYβ·g1))  与 G2[0] 配对
	// P3 = -D_tau = (C1+C2) - ρ·π_{1,agg}                与 G2[1] 配对

	// ρ·Pi1AggH（P1 和 P3 中均使用）
	var rhoPi1Jac bn254.G1Jac
	rhoPi1Jac.FromAffine(&proof.Pi1AggH)
	mulJacByFr(&rhoPi1Jac, rho)

	// ρ·(ComGY - gYBeta·g1)
	var rhoComGYDiff bn254.G1Jac
	rhoComGYDiff.FromAffine(&proof.ComGY)
	var gYBetaG1 bn254.G1Jac
	gYBetaG1.FromAffine(&g1Gen)
	mulJacByFr(&gYBetaG1, gYBeta)
	rhoComGYDiff.SubAssign(&gYBetaG1)
	mulJacByFr(&rhoComGYDiff, rho)

	// (ω·α)·C1
	c1ScaledAlphaS := c1
	mulJacByFr(&c1ScaledAlphaS, *alphaShifted)

	// α·C2
	c2ScaledAlpha := c2
	mulJacByFr(&c2ScaledAlpha, alpha)

	// 修正后的 4 配对（由 X 验证 × Y 验证推导）：
	// e(ComQX,ZT)·e(ρ·π₁,τY-β)·e(P2,G2[0])·e(P3,G2[1]) = 1
	// 其中 P2 = ωα·C1 + α·C2 - ρ·(ComGY-gYβ·g1)（无取反；减去 ρ·Y 项）
	//      P3 = -(C1+C2)                              （取反；无 π₁ 项）
	var p2Jac bn254.G1Jac
	p2Jac.Set(&c1ScaledAlphaS)
	p2Jac.AddAssign(&c2ScaledAlpha)
	p2Jac.SubAssign(&rhoComGYDiff) // 减去 ρ·(ComGY - gYBeta·g1)

	// P3（与 G2[1] 配对）= -(C1+C2)
	var p3Jac bn254.G1Jac
	p3Jac.Set(&c1)
	p3Jac.AddAssign(&c2)
	p3Jac.Neg(&p3Jac)

	// 转换为仿射坐标。
	var rhoPi1Aff, p2Aff, p3Aff bn254.G1Affine
	rhoPi1Aff.FromJacobian(&rhoPi1Jac)
	p2Aff.FromJacobian(&p2Jac)
	p3Aff.FromJacobian(&p3Jac)

	// ── 10. 4 配对验证 ────────────────────────────────────────────────────────
	// e(ComQX, ZTG2) · e(ρ·π_{1,agg}, tauYBeta) · e(-D_lin, G2[0]) · e(-D_tau, G2[1]) = 1
	ok, err2 := bn254.PairingCheck(
		[]bn254.G1Affine{proof.ComQX, rhoPi1Aff, p2Aff, p3Aff},
		[]bn254.G2Affine{zTG2, tauYBetaG2, srs.G2[0], srs.G2[1]},
	)
	if err2 != nil {
		return err2
	}
	if !ok {
		return fmt.Errorf("bpiano: 4 配对验证失败")
	}
	return nil
}

// VerifyBatch 以常数 4 次 pairing 批量验证由 CoordinateChallenges + AggregateProofs
// 生成的聚合证明（论文 §4.3.3）。
//
// 验证步骤：
//  1. 从所有证明的 Hx^{(k)} 重推共享 α；从 ComQX^{(k)} 等重推共享 β。
//  2. 对每个 k：重推 per-proof 挑战（γ_k/η_k/λ_k 经 FS；ν_k/μ_k 经协调哈希）。
//  3. 派生聚合系数 r_k，验证 agg.ComQXTotal / agg.Pi1Total 与期望值一致。
//  4. O(K) 代数约束检查。
//  5. 构建 C1_total、C2_total、D_{Y,total}；派生聚合 ρ。
//  6. 1 次 4-pairing 验证。
func VerifyBatch(agg *AggregatedProof, vk *piano.VerifyingKey, publicInputs [][][]fr.Element) error {
	K := agg.K
	proofs := agg.Proofs
	T := vk.SizeX
	M := vk.SizeY
	srs := vk.DKZGSRS

	if len(proofs) != K {
		return fmt.Errorf("bpiano: agg.K=%d 但 len(Proofs)=%d", K, len(proofs))
	}
	if len(publicInputs) != K {
		return fmt.Errorf("bpiano: publicInputs 长度 %d ≠ K=%d", len(publicInputs), K)
	}

	// ── 1. 重推共享挑战 ───────────────────────────────────────────────────────
	sharedAlpha := deriveSharedAlphaFromProofs(proofs)
	sharedBeta := deriveSharedBetaFromProofs(sharedAlpha, proofs)

	alphaPowT := new(fr.Element)
	alphaPowT.Exp(sharedAlpha, new(big.Int).SetUint64(T))
	var alphaShifted fr.Element
	alphaShifted.Mul(&sharedAlpha, &vk.GeneratorX)

	// ── 2. 派生聚合系数 ───────────────────────────────────────────────────────
	rk := deriveAggCoeffs(proofs)

	_, _, g1Gen, _ := bn254.Generators()

	// ── 3. 逐证明计算 C1_k、C2_k、D_{Y,k}，并累加 ────────────────────────────
	var c1TotalJac, c2TotalJac, dYTotalJac bn254.G1Jac
	var comQXExpectedJac, pi1ExpectedJac bn254.G1Jac

	for k, proof := range proofs {
		// ── 重推 per-proof γ_k/η_k/λ_k（标准 FS 格式，与 CompressStage1 相同）──
		hFunc := sha256.New()
		fs := fiatshamir.NewTranscript(hFunc,
			"gamma", "eta", "lambda", "alpha", "nu", "beta", "mu")

		if err := bindPublicDataBP(fs, "gamma", vk, publicInputs[k]); err != nil {
			return fmt.Errorf("bpiano VerifyBatch: proof[%d] bindPublicData: %w", k, err)
		}
		gamma_k, err := deriveChallengeBP(fs, "gamma",
			[]dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
		if err != nil {
			return fmt.Errorf("bpiano VerifyBatch: proof[%d] derive gamma: %w", k, err)
		}
		eta_k, err := deriveChallengeBP(fs, "eta", nil)
		if err != nil {
			return fmt.Errorf("bpiano VerifyBatch: proof[%d] derive eta: %w", k, err)
		}
		lambda_k, err := deriveChallengeBP(fs, "lambda", []dkzg.Digest{proof.Z})
		if err != nil {
			return fmt.Errorf("bpiano VerifyBatch: proof[%d] derive lambda: %w", k, err)
		}

		// ── 代数约束检查（O(K)）────────────────────────────────────────────────
		if err := verifyAlgebraicConstraintBP(proof, sharedAlpha, sharedBeta,
			lambda_k, eta_k, gamma_k, T, M, vk); err != nil {
			return fmt.Errorf("bpiano VerifyBatch: proof[%d] 代数约束失败: %w", k, err)
		}

		// ── 重推 per-proof ν_k / μ_k（协调哈希格式）──────────────────────────
		foldedHxDig_k := piano.FoldDigests(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)
		nu_k := deriveNuCoord(sharedAlpha, foldedHxDig_k, proof.LRO, proof.Z)
		nuPow_k := buildNuPow(nu_k, 14)
		mu_k := deriveMuCoord(sharedBeta)
		muPow_k := buildNuPow(mu_k, 7)

		// ── 构建 C1_k（同 VerifyCompressed）──────────────────────────────────
		sAlphaComs := []dkzg.Digest{
			foldedHxDig_k,
			proof.LRO[0], proof.LRO[1], proof.LRO[2], proof.Z,
			vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3,
		}
		sAlphaEvals := []fr.Element{
			proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO, proof.EvalZ,
			proof.EvalQl, proof.EvalQr, proof.EvalQm, proof.EvalQo, proof.EvalQk,
			proof.EvalS1, proof.EvalS2, proof.EvalS3,
		}
		c1_k := computeC(sAlphaComs, sAlphaEvals, nuPow_k[:13], g1Gen)
		c2_k := computeC(
			[]dkzg.Digest{proof.Z},
			[]fr.Element{proof.EvalZS},
			[]fr.Element{nuPow_k[13]},
			g1Gen,
		)

		// ComVFAlpha / ComVFZS 修正（同 VerifyCompressed）
		{
			var evalAlphaCorr fr.Element
			for j, e := range []fr.Element{
				proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO, proof.EvalZ,
			} {
				var t fr.Element
				t.Mul(&nuPow_k[j], &e)
				evalAlphaCorr.Add(&evalAlphaCorr, &t)
			}
			var evalCorrG1 bn254.G1Jac
			evalCorrG1.FromAffine(&g1Gen)
			mulJacByFr(&evalCorrG1, evalAlphaCorr)
			var comVFAlphaJac bn254.G1Jac
			comVFAlphaJac.FromAffine(&proof.ComVFAlpha)
			c1_k.AddAssign(&evalCorrG1)
			c1_k.SubAssign(&comVFAlphaJac)

			var nuZSEvalZS fr.Element
			nuZSEvalZS.Mul(&nuPow_k[13], &proof.EvalZS)
			var nuZSEvalZSG1 bn254.G1Jac
			nuZSEvalZSG1.FromAffine(&g1Gen)
			mulJacByFr(&nuZSEvalZSG1, nuZSEvalZS)
			var comVFZSScaled bn254.G1Jac
			comVFZSScaled.FromAffine(&proof.ComVFZS)
			mulJacByFr(&comVFZSScaled, nuPow_k[13])
			c2_k.AddAssign(&nuZSEvalZSG1)
			c2_k.SubAssign(&comVFZSScaled)
		}

		// ── D_{Y,k} = ComGY_k - G_Y(β_k)·g1 ─────────────────────────────────
		yEvals_k := []fr.Element{
			proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO,
			proof.EvalZ, proof.EvalZS, proof.EvalHy,
		}
		var gYBeta_k fr.Element
		for j, e := range yEvals_k {
			var term fr.Element
			term.Mul(&muPow_k[j], &e)
			gYBeta_k.Add(&gYBeta_k, &term)
		}
		var dY_k bn254.G1Jac
		dY_k.FromAffine(&proof.ComGY)
		var gYBetaG1_k bn254.G1Jac
		gYBetaG1_k.FromAffine(&g1Gen)
		mulJacByFr(&gYBetaG1_k, gYBeta_k)
		dY_k.SubAssign(&gYBetaG1_k)

		// ── r_k 加权累加 ──────────────────────────────────────────────────────
		c1_k_scaled := c1_k
		mulJacByFr(&c1_k_scaled, rk[k])
		c1TotalJac.AddAssign(&c1_k_scaled)

		c2_k_scaled := c2_k
		mulJacByFr(&c2_k_scaled, rk[k])
		c2TotalJac.AddAssign(&c2_k_scaled)

		dY_k_scaled := dY_k
		mulJacByFr(&dY_k_scaled, rk[k])
		dYTotalJac.AddAssign(&dY_k_scaled)

		// 同时计算期望的 ComQXTotal 和 Pi1Total（用于一致性校验）
		var comQX_k bn254.G1Jac
		comQX_k.FromAffine(&proof.ComQX)
		mulJacByFr(&comQX_k, rk[k])
		comQXExpectedJac.AddAssign(&comQX_k)

		var pi1_k bn254.G1Jac
		pi1_k.FromAffine(&proof.Pi1AggH)
		mulJacByFr(&pi1_k, rk[k])
		pi1ExpectedJac.AddAssign(&pi1_k)
	}

	// ── 4. 一致性校验：agg.ComQXTotal / agg.Pi1Total 与本地重算结果相符 ───────
	var comQXExpectedAff, pi1ExpectedAff bn254.G1Affine
	comQXExpectedAff.FromJacobian(&comQXExpectedJac)
	pi1ExpectedAff.FromJacobian(&pi1ExpectedJac)

	if !comQXExpectedAff.Equal(&agg.ComQXTotal) {
		return fmt.Errorf("bpiano VerifyBatch: ComQXTotal 一致性校验失败")
	}
	if !pi1ExpectedAff.Equal(&agg.Pi1Total) {
		return fmt.Errorf("bpiano VerifyBatch: Pi1Total 一致性校验失败")
	}

	// ── 5. 派生聚合 ρ 并组装 4 个 G1 点 ─────────────────────────────────────
	var dYTotalAff bn254.G1Affine
	dYTotalAff.FromJacobian(&dYTotalJac)
	rho := deriveRhoBatch(agg.ComQXTotal, dYTotalAff, agg.Pi1Total)

	// ZTG2 = [Z_T(τ_X)]_2（仅依赖共享 α）
	zTG2 := computeZTG2(sharedAlpha, alphaShifted, srs)
	// tauYBetaG2 = [τ_Y - β]_2（仅依赖共享 β）
	tauYBetaG2 := computeTauYMinusBetaG2(sharedBeta, srs)

	// P0 = ComQXTotal
	p0 := agg.ComQXTotal

	// P1 = ρ·Pi1Total
	var p1Jac bn254.G1Jac
	p1Jac.FromAffine(&agg.Pi1Total)
	mulJacByFr(&p1Jac, rho)
	var p1Aff bn254.G1Affine
	p1Aff.FromJacobian(&p1Jac)

	// P2 = ωα·C1_total + α·C2_total - ρ·D_{Y,total}
	c1Scaled := c1TotalJac
	mulJacByFr(&c1Scaled, alphaShifted)
	c2Scaled := c2TotalJac
	mulJacByFr(&c2Scaled, sharedAlpha)
	rhoD_Y := dYTotalJac
	mulJacByFr(&rhoD_Y, rho)

	var p2Jac bn254.G1Jac
	p2Jac.Set(&c1Scaled)
	p2Jac.AddAssign(&c2Scaled)
	p2Jac.SubAssign(&rhoD_Y)
	var p2Aff bn254.G1Affine
	p2Aff.FromJacobian(&p2Jac)

	// P3 = -(C1_total + C2_total)
	var p3Jac bn254.G1Jac
	p3Jac.Set(&c1TotalJac)
	p3Jac.AddAssign(&c2TotalJac)
	p3Jac.Neg(&p3Jac)
	var p3Aff bn254.G1Affine
	p3Aff.FromJacobian(&p3Jac)

	// ── 6. 4-pairing 验证 ────────────────────────────────────────────────────
	ok, err2 := bn254.PairingCheck(
		[]bn254.G1Affine{p0, p1Aff, p2Aff, p3Aff},
		[]bn254.G2Affine{zTG2, tauYBetaG2, srs.G2[0], srs.G2[1]},
	)
	if err2 != nil {
		return err2
	}
	if !ok {
		return fmt.Errorf("bpiano VerifyBatch: 聚合 4-pairing 验证失败")
	}
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// VerifyBatch 辅助函数
// ────────────────────────────────────────────────────────────────────────────

// deriveSharedAlphaFromProofs 从 K 个证明的 Hx^{(k)}[0..2] 重推共享 α（验证端使用）。
// 格式与 CoordinateChallenges 中的 deriveSharedAlpha 完全相同。
func deriveSharedAlphaFromProofs(proofs []*CompressedProof) fr.Element {
	h := sha256.New()
	h.Write([]byte("coord-alpha"))
	for _, p := range proofs {
		for _, d := range p.Hx {
			b := d.Bytes()
			h.Write(b[:])
		}
	}
	var alpha fr.Element
	alpha.SetBytes(h.Sum(nil))
	return alpha
}

// deriveSharedBetaFromProofs 从 K 个证明的 ComQX 等重推共享 β（验证端使用）。
// 格式与 CoordinateChallenges 中的 deriveSharedBeta 完全相同。
func deriveSharedBetaFromProofs(sharedAlpha fr.Element, proofs []*CompressedProof) fr.Element {
	h := sha256.New()
	h.Write([]byte("coord-beta"))
	b := sharedAlpha.Bytes()
	h.Write(b[:])
	for _, p := range proofs {
		for _, d := range []dkzg.Digest{p.ComQX, p.ComVFAlpha, p.ComVFZS} {
			b2 := d.Bytes()
			h.Write(b2[:])
		}
	}
	var beta fr.Element
	beta.SetBytes(h.Sum(nil))
	return beta
}

// deriveRhoBatch 为聚合验证派生随机挑战 ρ。
//
// 格式：SHA256("agg-rho" || ComQXTotal || D_{Y,total} || Pi1Total)
func deriveRhoBatch(comQXTotal dkzg.Digest, dYTotal bn254.G1Affine, pi1Total dkzg.Digest) fr.Element {
	h := sha256.New()
	h.Write([]byte("agg-rho"))
	b := comQXTotal.Bytes()
	h.Write(b[:])
	b2 := dYTotal.Bytes()
	h.Write(b2[:])
	b3 := pi1Total.Bytes()
	h.Write(b3[:])
	var rho fr.Element
	rho.SetBytes(h.Sum(nil))
	return rho
}

// ────────────────────────────────────────────────────────────────────────────
// 代数约束验证
// ────────────────────────────────────────────────────────────────────────────

func verifyAlgebraicConstraintBP(
	proof *CompressedProof,
	alpha, beta, lambda, eta, gamma fr.Element,
	T, M uint64, vk *piano.VerifyingKey,
) error {
	a, b, o := proof.EvalA, proof.EvalB, proof.EvalO
	z, zs := proof.EvalZ, proof.EvalZS
	hx, hy := proof.EvalHx, proof.EvalHy
	ql, qr, qm, qo, qk := proof.EvalQl, proof.EvalQr, proof.EvalQm, proof.EvalQo, proof.EvalQk
	s1, s2, s3 := proof.EvalS1, proof.EvalS2, proof.EvalS3

	var one fr.Element
	one.SetOne()

	var gate fr.Element
	{
		var t0, t1, t2, t3 fr.Element
		t0.Mul(&ql, &a); t1.Mul(&qr, &b)
		t2.Mul(&qm, &a); t2.Mul(&t2, &b)
		t3.Mul(&qo, &o)
		gate.Add(&t0, &t1).Add(&gate, &t2).Add(&gate, &t3).Add(&gate, &qk)
	}

	l0 := piano.ComputeLagrange0(alpha, vk.DKZGSRS.DomainX.CardinalityInv)
	var boundary fr.Element
	{
		var t fr.Element
		t.Sub(&z, &one)
		boundary.Mul(&l0, &t)
	}

	u := vk.CosetShift
	var uSq, idA, idB, idC fr.Element
	uSq.Square(&u)
	idA.Set(&alpha); idB.Mul(&u, &alpha); idC.Mul(&uSq, &alpha)

	var F, G fr.Element
	{
		var f0, f1, f2 fr.Element
		f0.Mul(&eta, &idA).Add(&f0, &a).Add(&f0, &gamma)
		f1.Mul(&eta, &idB).Add(&f1, &b).Add(&f1, &gamma)
		f2.Mul(&eta, &idC).Add(&f2, &o).Add(&f2, &gamma)
		F.Mul(&f0, &f1).Mul(&F, &f2).Mul(&F, &z)

		var g0, g1, g2 fr.Element
		g0.Mul(&eta, &s1).Add(&g0, &a).Add(&g0, &gamma)
		g1.Mul(&eta, &s2).Add(&g1, &b).Add(&g1, &gamma)
		g2.Mul(&eta, &s3).Add(&g2, &o).Add(&g2, &gamma)
		G.Mul(&g0, &g1).Mul(&G, &g2).Mul(&G, &zs)
	}
	var perm fr.Element
	perm.Sub(&G, &F)

	var lhs fr.Element
	{
		var tmp fr.Element
		tmp.Mul(&lambda, &boundary).Add(&tmp, &perm)
		lhs.Mul(&lambda, &tmp).Add(&lhs, &gate)
	}

	var vanX, vanY fr.Element
	vanX.Exp(alpha, new(big.Int).SetUint64(T))
	vanX.Sub(&vanX, &one)
	vanY.Exp(beta, new(big.Int).SetUint64(M))
	vanY.Sub(&vanY, &one)

	var rhs fr.Element
	{
		var t0, t1 fr.Element
		t0.Mul(&vanX, &hx)
		t1.Mul(&vanY, &hy)
		rhs.Add(&t0, &t1)
	}

	if !lhs.Equal(&rhs) {
		return fmt.Errorf("bpiano: 代数约束不满足：lhs=%s rhs=%s",
			lhs.String(), rhs.String())
	}
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// G1 辅助函数
// ────────────────────────────────────────────────────────────────────────────

// mulJacByFr 原地将 G1Jac 点乘以域元素。
func mulJacByFr(p *bn254.G1Jac, s fr.Element) {
	var b big.Int
	s.BigInt(&b)
	p.ScalarMultiplication(p, &b)
}

// computeC 计算 Σ_k nuPow[k]·(coms[k] - evals[k]·g1)（结果为 G1Jac）。
func computeC(coms []dkzg.Digest, evals []fr.Element, nuPow []fr.Element, g1 bn254.G1Affine) bn254.G1Jac {
	var result bn254.G1Jac
	for k := range coms {
		var comJac bn254.G1Jac
		comJac.FromAffine(&coms[k])

		var evalG1 bn254.G1Jac
		evalG1.FromAffine(&g1)
		mulJacByFr(&evalG1, evals[k])
		comJac.SubAssign(&evalG1)

		mulJacByFr(&comJac, nuPow[k])
		result.AddAssign(&comJac)
	}
	return result
}

// computeZTG2 计算 [Z_T(τ_X)]_2 = G2[2] - (α+ω·α)·G2[1] + α·ω·α·G2[0]。
func computeZTG2(alpha, alphaShifted fr.Element, srs *dkzg.SRS) bn254.G2Affine {
	var coeff1, coeff0 fr.Element
	coeff1.Add(&alpha, &alphaShifted)
	coeff0.Mul(&alpha, &alphaShifted)

	var result bn254.G2Jac
	result.FromAffine(&srs.G2[2])

	var tmp bn254.G2Jac
	var b big.Int

	tmp.FromAffine(&srs.G2[1])
	coeff1.BigInt(&b)
	tmp.ScalarMultiplication(&tmp, &b)
	result.SubAssign(&tmp)

	tmp.FromAffine(&srs.G2[0])
	coeff0.BigInt(&b)
	tmp.ScalarMultiplication(&tmp, &b)
	result.AddAssign(&tmp)

	var aff bn254.G2Affine
	aff.FromJacobian(&result)
	return aff
}

// computeTauYMinusBetaG2 计算 [τ_Y - β]_2 = G2Y[1] - β·G2Y[0]。
func computeTauYMinusBetaG2(beta fr.Element, srs *dkzg.SRS) bn254.G2Affine {
	var result bn254.G2Jac
	result.FromAffine(&srs.G2Y[1])

	var tmp bn254.G2Jac
	tmp.FromAffine(&srs.G2Y[0])
	var b big.Int
	beta.BigInt(&b)
	tmp.ScalarMultiplication(&tmp, &b)
	result.SubAssign(&tmp)

	var aff bn254.G2Affine
	aff.FromJacobian(&result)
	return aff
}

// deriveRhoBP 对关键证明承诺和求值进行哈希，生成随机挑战 ρ。
func deriveRhoBP(proof *CompressedProof) fr.Element {
	h := sha256.New()
	for _, d := range []dkzg.Digest{proof.ComQX, proof.ComGY, proof.Pi1AggH} {
		b := d.Bytes()
		h.Write(b[:])
	}
	for _, e := range []fr.Element{
		proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO,
		proof.EvalZ, proof.EvalZS, proof.EvalHy,
	} {
		b := e.Bytes()
		h.Write(b[:])
	}
	var rho fr.Element
	rho.SetBytes(h.Sum(nil))
	return rho
}
