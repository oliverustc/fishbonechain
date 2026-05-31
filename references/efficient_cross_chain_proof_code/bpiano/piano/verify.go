package piano

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
	"github.com/oliverustc/bpiano/dkzg"
)

// Verify 使用验证密钥和公开输入验证 Piano 证明。
//
// publicInputs[i] 是第 i 个子电路的公开输入。若无公开输入，传 nil 即可。
func Verify(vk *VerifyingKey, proof *Proof, publicInputs [][]fr.Element) error {
	T := int(vk.SizeX)
	M := int(vk.SizeY)

	// ── 步骤 1：重放 Fiat-Shamir 挑战 ────────────────────────────────────────
	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc, "gamma", "eta", "lambda", "alpha", "beta")

	if err := bindPublicData(fs, "gamma", vk, publicInputs); err != nil {
		return err
	}
	gamma, err := deriveChallenge(fs, "gamma", []dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
	if err != nil {
		return err
	}
	eta, err := deriveChallenge(fs, "eta", nil)
	if err != nil {
		return err
	}
	lambda, err := deriveChallenge(fs, "lambda", []dkzg.Digest{proof.Z})
	if err != nil {
		return err
	}
	alpha, err := deriveChallenge(fs, "alpha", []dkzg.Digest{proof.Hx[0], proof.Hx[1], proof.Hx[2]})
	if err != nil {
		return err
	}
	// 绑定 BatchedProofX.H + ClaimedDigests[0..12] + Hy[0..2] 以派生 beta。
	betaDigests := make([]dkzg.Digest, 0, 1+13+3)
	betaDigests = append(betaDigests, proof.BatchedProofX.H)
	for k := range proof.BatchedProofX.ClaimedDigests {
		betaDigests = append(betaDigests, proof.BatchedProofX.ClaimedDigests[k])
	}
	betaDigests = append(betaDigests, proof.Hy[0], proof.Hy[1], proof.Hy[2])
	beta, err := deriveChallenge(fs, "beta", betaDigests)
	if err != nil {
		return err
	}

	// ── 步骤 2：验证 (β, α) 处的代数约束 ───────────────────────────────────
	if err := verifyAlgebraicConstraint(alpha, beta, proof, eta, gamma, lambda, vk); err != nil {
		return err
	}

	// ── 步骤 3：验证 DKZG 证明 ────────────────────────────────────────────────
	if err := verifyDKZGProofs(alpha, beta, proof, vk, T, M); err != nil {
		return err
	}

	return nil
}

// verifyAlgebraicConstraint 验证 (β, α) 处的主代数恒等式：
//
//	gate(β,α) + λ·L₀(α)·(Z(β,α)-1) + λ²·perm(β,α) = (α^T-1)·Hx(β,α) + (β^M-1)·Hy(β)
func verifyAlgebraicConstraint(
	alpha, beta fr.Element,
	proof *Proof,
	eta, gamma, lambda fr.Element,
	vk *VerifyingKey,
) error {
	T := int(vk.SizeX)
	M := int(vk.SizeY)

	a := proof.ClaimedA
	b := proof.ClaimedB
	o := proof.ClaimedO
	z := proof.ClaimedZ
	zs := proof.ClaimedZS
	hx := proof.ClaimedHx
	hy := proof.ClaimedHy

	ql := proof.ClaimedQl
	qr := proof.ClaimedQr
	qm := proof.ClaimedQm
	qo := proof.ClaimedQo
	qk := proof.ClaimedQk
	s1 := proof.ClaimedS1
	s2 := proof.ClaimedS2
	s3 := proof.ClaimedS3

	var one fr.Element
	one.SetOne()

	// 门约束：ql·a + qr·b + qm·a·b + qo·o + qk
	var gate fr.Element
	{
		var t0, t1, t2, t3 fr.Element
		t0.Mul(&ql, &a)
		t1.Mul(&qr, &b)
		t2.Mul(&qm, &a)
		t2.Mul(&t2, &b)
		t3.Mul(&qo, &o)
		gate.Add(&t0, &t1).Add(&gate, &t2).Add(&gate, &t3).Add(&gate, &qk)
	}

	// 边界约束：L₀(α)·(Z(β,α)-1)
	l0 := computeLagrange0(alpha, vk.CardinalityInvX())
	var boundary fr.Element
	{
		var t fr.Element
		t.Sub(&z, &one)
		boundary.Mul(&l0, &t)
	}

	// 置换约束：
	//   F = (A+η·α+γ)(B+η·u·α+γ)(O+η·u²·α+γ)·Z
	//   G = (A+η·s1+γ)(B+η·s2+γ)(O+η·s3+γ)·ZS
	u := vk.CosetShift
	var uSq, idA, idB, idC fr.Element
	uSq.Square(&u)
	idA.Set(&alpha)
	idB.Mul(&u, &alpha)
	idC.Mul(&uSq, &alpha)

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

	// 左端（LHS）= gate + λ·boundary + λ²·perm
	var lhs fr.Element
	{
		var tmp fr.Element
		tmp.Mul(&lambda, &boundary).Add(&tmp, &perm)
		lhs.Mul(&lambda, &tmp).Add(&lhs, &gate)
	}

	// 右端（RHS）= (α^T-1)·hx + (β^M-1)·hy
	var vanishX, vanishY fr.Element
	vanishX.Exp(alpha, new(big.Int).SetInt64(int64(T)))
	vanishX.Sub(&vanishX, &one)
	vanishY.Exp(beta, new(big.Int).SetInt64(int64(M)))
	vanishY.Sub(&vanishY, &one)

	var rhs fr.Element
	var t0, t1 fr.Element
	t0.Mul(&vanishX, &hx)
	t1.Mul(&vanishY, &hy)
	rhs.Add(&t0, &t1)

	if !lhs.Equal(&rhs) {
		return fmt.Errorf("piano: 代数约束不满足：lhs=%s rhs=%s",
			lhs.String(), rhs.String())
	}
	return nil
}

// verifyDKZGProofs 验证 Piano 证明中的所有 DKZG 开放证明。
//
// 执行两项主要验证：
//  1. X 轴：对 13 个多项式在 α 处以及 Z 在 ω·α 处调用 VerifyBatchedAndAggregatedX
//  2. Y 轴：对 15 个 Y 轴多项式在 β 处调用 VerifyBatchedProofY
func verifyDKZGProofs(alpha, beta fr.Element, proof *Proof, vk *VerifyingKey, T, M int) error {
	srs := vk.DKZGSRS
	var alphaShifted fr.Element
	alphaShifted.Mul(&alpha, &vk.GeneratorX)

	alphaPowT := new(fr.Element)
	{
		bT := new(big.Int).SetInt64(int64(T))
		alphaPowT.Exp(alpha, bT)
	}
	betaPowM := new(fr.Element)
	{
		bM := new(big.Int).SetInt64(int64(M))
		betaPowM.Exp(beta, bM)
	}

	// ── 重建 foldedHx 和 foldedHy 承诺 ──────────────────────────────────────
	foldedHxDig := foldDigests(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)
	foldedHyDig := foldDigests(proof.Hy[0], proof.Hy[1], proof.Hy[2], betaPowM)

	// ── X 轴批量验证 ──────────────────────────────────────────────────────────
	// 批量验证所用的 13 个承诺（顺序与证明方的 batchComFs 一致）。
	batchComFs := []dkzg.Digest{
		foldedHxDig,
		proof.LRO[0], proof.LRO[1], proof.LRO[2],
		vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk,
		vk.S1, vk.S2, vk.S3,
		proof.Z,
	}

	if err := dkzg.VerifyBatchedAndAggregatedX(
		batchComFs, proof.BatchedProofX, alpha,
		proof.Z, proof.ZShiftedProofX, alphaShifted,
		srs,
	); err != nil {
		return fmt.Errorf("piano: X 轴 DKZG 验证失败：%w", err)
	}

	// ── Y 轴批量验证 ──────────────────────────────────────────────────────────
	// 15 个 comVF 承诺：
	//   [0..12]：BatchedProofX.ClaimedDigests
	//   [13]：   ZShiftedProofX.ComVF
	//   [14]：   foldedHyDig
	yComVFs := make([]dkzg.Digest, 15)
	copy(yComVFs[:13], proof.BatchedProofX.ClaimedDigests)
	yComVFs[13] = proof.ZShiftedProofX.ComVF
	yComVFs[14] = foldedHyDig

	if err := dkzg.VerifyBatchedProofY(yComVFs, proof.BatchedProofY, beta, srs); err != nil {
		return fmt.Errorf("piano: Y 轴 DKZG 验证失败：%w", err)
	}

	return nil
}

// verifyYOnlyProof 验证 e(comVF - [z]_1, g2_Y[0]) = e(H, [τ_Y - β]_2)。
func verifyYOnlyProof(
	comVF dkzg.Digest,
	z fr.Element,
	H bn254.G1Affine,
	beta fr.Element,
	srs *dkzg.SRS,
) error {
	_, _, g1Aff, _ := bn254.Generators()

	// [z]_1 = z · g1。
	var zBig big.Int
	z.BigInt(&zBig)
	var zG1 bn254.G1Affine
	zG1.ScalarMultiplication(&g1Aff, &zBig)

	// comVF - [z]_1。
	var diffJac, comVFJac, zG1Jac bn254.G1Jac
	comVFJac.FromAffine(&comVF)
	zG1Jac.FromAffine(&zG1)
	diffJac.Set(&comVFJac)
	diffJac.SubAssign(&zG1Jac)
	var diff bn254.G1Affine
	diff.FromJacobian(&diffJac)

	// [τ_Y - β]_2 = srs.G2Y[1] - β·srs.G2Y[0]。
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
		[]bn254.G1Affine{diff, negG1(H)},
		[]bn254.G2Affine{srs.G2Y[0], tauYMinusBeta},
	)
	if err != nil {
		return err
	}
	if !ok {
		return errVerification
	}
	return nil
}

// CardinalityInvX 返回 1/T（fr.Element 形式），用于计算 L_0(α)。
// 为使用方便，该值直接从 VerifyingKey 中计算得到。
func (vk *VerifyingKey) CardinalityInvX() fr.Element {
	var inv fr.Element
	inv.SetUint64(vk.SizeX).Inverse(&inv)
	return inv
}
