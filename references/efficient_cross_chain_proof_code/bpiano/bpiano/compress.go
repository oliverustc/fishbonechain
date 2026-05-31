// Package bpiano 实现 BPiano 协议：在 Piano 数据并行 PLONK 系统之上进行
// 证明压缩与批量聚合。
//
// 与 Piano 相比，BPiano 通过以下方式减小证明大小和验证开销：
//  1. 用一个 Shplonk 商承诺 ComQX 替代逐多项式的 X 轴开放证明
//     （消除 13+ 个 G1 ClaimedDigests）。
//  2. 将所有 Y 轴多项式折叠为一个聚合多项式 G_Y，
//     其承诺 ComGY 和开放证明 Pi1AggH 替代多个单独的证明。
//  3. 将 X 轴和 Y 轴验证合并为一个 4-配对方程。
//
// 批量聚合（CoordinateChallenges + Aggregate + VerifyBatch）可将 K 个证明
// 的验证进一步压缩为常数 4 次配对运算。
package bpiano

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"sync"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/piano"
)

// CompressedProof 是 BPiano 压缩证明。
//
// 与 piano.Proof 相比，它替换了：
//   - BatchedProofX（H + 13 个 ClaimedDigests）和 ZShiftedProofX（H + ComVF）
//     → ComQX（1 个 G1，Shplonk 聚合商承诺）
//   - Hy[3]、BatchedProofY（H + 15 个 ClaimedValues）
//     → ComGY（1 个 G1）+ Pi1AggH（1 个 G1）+ 7 个标量
//
// 总大小：10 个 G1 + 7 个 fr.Element（BN254 曲线约 544 字节）。
type CompressedProof struct {
	// 多项式承诺（与 Piano 相同）。
	LRO [3]dkzg.Digest // com_A, com_B, com_O
	Z   dkzg.Digest    // com_Z
	Hx  [3]dkzg.Digest // com_{H_X,0..2}

	// Shplonk X 轴聚合商承诺。
	// 编码 Q_X(Y,X)：S_α={foldedHx,A,B,O,Z} 在 α 处，{Z} 在 ω·α 处。
	ComQX dkzg.Digest

	// witness 多项式求值向量的 Y 轴承诺。
	// ComVFAlpha = Σ_{k=0}^{4} ν^k · CommitY([f_{k,i}(α)]_i)（fhx,A,B,O,Z 在 α 处）
	// ComVFZS   = CommitY([Z_i(ωα)]_i)                        （Z 在 ω·α 处）
	// 这些用于在双变量 Shplonk 配对验证中正确构造 C1 和 C2，
	// 因为 DKZG 全局承诺在 Y 方向上编码的是 τ_Y 而非 β。
	ComVFAlpha dkzg.Digest
	ComVFZS    dkzg.Digest

	// Y 轴聚合：G_Y(Y) = Σ_k μ^k · P_k(Y) 的承诺。
	ComGY dkzg.Digest
	// Y 轴开放商：g^{Q_Y(τ_Y)}，其中 Q_Y = (G_Y - G_Y(β))/(Y-β)。
	Pi1AggH dkzg.Digest

	// (β, α) 处的 7 个求值标量。
	EvalA   fr.Element // A(β, α)
	EvalB   fr.Element // B(β, α)
	EvalO   fr.Element // O(β, α)
	EvalZ   fr.Element // Z(β, α)
	EvalZS  fr.Element // Z(β, ω·α)
	EvalHx  fr.Element // foldedHx(β, α)
	EvalHy  fr.Element // foldedHy(β)

	// 共享多项式在 α 处的 8 个求值（由 Shplonk 证明，用于代数验证）。
	EvalQl, EvalQr, EvalQm, EvalQo, EvalQk fr.Element
	EvalS1, EvalS2, EvalS3                 fr.Element
}

// Compress 生成 BPiano 压缩证明（CompressedProof）。
//
// 执行与 Piano Prove 相同的 witness/Z/商多项式计算，
// 但将 X 轴开放替换为 Shplonk，将 Y 轴开放替换为 G_Y 聚合。
func Compress(pk *piano.ProvingKey, witnesses []piano.WitnessInstance, publicInputs [][]fr.Element) (*CompressedProof, error) {
	M := int(pk.Vk.SizeY)
	T := int(pk.Vk.SizeX)

	if len(witnesses) != M {
		return nil, fmt.Errorf("bpiano: 收到 %d 个 witness，期望 %d", len(witnesses), M)
	}

	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc, "gamma", "eta", "lambda", "alpha", "nu", "beta", "mu")

	proof := &CompressedProof{}

	// ── 第一轮：witness 承诺（与 Piano 相同）──────────────────────────────────
	comA := make([]dkzg.Digest, M)
	comB := make([]dkzg.Digest, M)
	comO := make([]dkzg.Digest, M)

	var wg sync.WaitGroup
	errCh := make(chan error, M*3)
	for i := 0; i < M; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			comA[i], err = dkzg.CommitLocal(uint64(i), witnesses[i].L, pk.DKZGSRS)
			if err != nil {
				errCh <- err
				return
			}
			comB[i], err = dkzg.CommitLocal(uint64(i), witnesses[i].R, pk.DKZGSRS)
			if err != nil {
				errCh <- err
				return
			}
			comO[i], err = dkzg.CommitLocal(uint64(i), witnesses[i].O, pk.DKZGSRS)
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	if err := <-errCh; err != nil {
		return nil, err
	}

	var err error
	proof.LRO[0], err = dkzg.AggregateDigests(comA)
	if err != nil {
		return nil, err
	}
	proof.LRO[1], err = dkzg.AggregateDigests(comB)
	if err != nil {
		return nil, err
	}
	proof.LRO[2], err = dkzg.AggregateDigests(comO)
	if err != nil {
		return nil, err
	}

	if err := bindPublicDataBP(fs, "gamma", pk.Vk, publicInputs); err != nil {
		return nil, err
	}
	gamma, err := deriveChallengeBP(fs, "gamma", []dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
	if err != nil {
		return nil, err
	}
	eta, err := deriveChallengeBP(fs, "eta", nil)
	if err != nil {
		return nil, err
	}

	// ── 第二轮：置换累加器 Z（与 Piano 相同）────────────────────────────────
	zLagrange := make([][]fr.Element, M)
	comZ := make([]dkzg.Digest, M)

	errCh2 := make(chan error, M)
	var wg2 sync.WaitGroup
	for i := 0; i < M; i++ {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			z, err := piano.ComputeZLagrange(witnesses[i].L, witnesses[i].R, witnesses[i].O, pk, eta, gamma)
			if err != nil {
				errCh2 <- err
				return
			}
			zLagrange[i] = z
			comZ[i], err = dkzg.CommitLocal(uint64(i), z, pk.DKZGSRS)
			if err != nil {
				errCh2 <- err
			}
		}(i)
	}
	wg2.Wait()
	close(errCh2)
	if err := <-errCh2; err != nil {
		return nil, err
	}

	proof.Z, err = dkzg.AggregateDigests(comZ)
	if err != nil {
		return nil, err
	}
	lambda, err := deriveChallengeBP(fs, "lambda", []dkzg.Digest{proof.Z})
	if err != nil {
		return nil, err
	}

	// ── 第三轮：X 轴商多项式（与 Piano 相同）────────────────────────────────
	qlCan := piano.LagrangeToCanonical(pk.Ql, &pk.DomainX)
	qrCan := piano.LagrangeToCanonical(pk.Qr, &pk.DomainX)
	qmCan := piano.LagrangeToCanonical(pk.Qm, &pk.DomainX)
	qoCan := piano.LagrangeToCanonical(pk.Qo, &pk.DomainX)
	qkCan := piano.LagrangeToCanonical(pk.Qk, &pk.DomainX)
	s1Can := piano.LagrangeToCanonical(pk.S1, &pk.DomainX)
	s2Can := piano.LagrangeToCanonical(pk.S2, &pk.DomainX)
	s3Can := piano.LagrangeToCanonical(pk.S3, &pk.DomainX)

	qlCoset := piano.CosetEval(qlCan, &pk.DomainXL)
	qrCoset := piano.CosetEval(qrCan, &pk.DomainXL)
	qmCoset := piano.CosetEval(qmCan, &pk.DomainXL)
	qoCoset := piano.CosetEval(qoCan, &pk.DomainXL)
	qkCoset := piano.CosetEval(qkCan, &pk.DomainXL)
	s1Coset := piano.CosetEval(s1Can, &pk.DomainXL)
	s2Coset := piano.CosetEval(s2Can, &pk.DomainXL)
	s3Coset := piano.CosetEval(s3Can, &pk.DomainXL)

	l0Coset := piano.L0OnCoset(&pk.DomainXL, T)
	vanXCoset := piano.VanishingOnCoset(&pk.DomainXL, uint64(T))
	fft.BitReverse(vanXCoset)
	vanXCosetInv := make([]fr.Element, len(vanXCoset))
	copy(vanXCosetInv, vanXCoset)
	vanXCosetInv = fr.BatchInvert(vanXCosetInv)

	type quotientResult struct {
		hx1, hx2, hx3 []fr.Element
	}
	qResults := make([]quotientResult, M)
	comHx1 := make([]dkzg.Digest, M)
	comHx2 := make([]dkzg.Digest, M)
	comHx3 := make([]dkzg.Digest, M)

	errCh3 := make(chan error, M)
	var wg3 sync.WaitGroup
	for i := 0; i < M; i++ {
		wg3.Add(1)
		go func(i int) {
			defer wg3.Done()
			aCan := piano.LagrangeToCanonical(witnesses[i].L, &pk.DomainX)
			bCan := piano.LagrangeToCanonical(witnesses[i].R, &pk.DomainX)
			oCan := piano.LagrangeToCanonical(witnesses[i].O, &pk.DomainX)
			zCan := piano.LagrangeToCanonical(zLagrange[i], &pk.DomainX)
			zShiftCan := piano.ShiftCanonical(zCan, pk.DomainX.Generator)

			aCoset := piano.CosetEval(aCan, &pk.DomainXL)
			bCoset := piano.CosetEval(bCan, &pk.DomainXL)
			oCoset := piano.CosetEval(oCan, &pk.DomainXL)
			zCoset := piano.CosetEval(zCan, &pk.DomainXL)
			zsCoset := piano.CosetEval(zShiftCan, &pk.DomainXL)

			idCoset := piano.CosetPoints(&pk.DomainXL)
			u := pk.DomainX.FrMultiplicativeGen
			var uSq fr.Element
			uSq.Square(&u)

			N4T := int(pk.DomainXL.Cardinality)
			h := make([]fr.Element, N4T)
			var one fr.Element
			one.SetOne()

			for j := 0; j < N4T; j++ {
				a, b, o := aCoset[j], bCoset[j], oCoset[j]
				z, zs := zCoset[j], zsCoset[j]
				ql, qr, qm, qo, qk := qlCoset[j], qrCoset[j], qmCoset[j], qoCoset[j], qkCoset[j]
				s1, s2, s3 := s1Coset[j], s2Coset[j], s3Coset[j]
				l0 := l0Coset[j]
				idA := idCoset[j]
				var idB, idC fr.Element
				idB.Mul(&u, &idA)
				idC.Mul(&uSq, &idA)

				var gate fr.Element
				{
					var t0, t1, t2, t3 fr.Element
					t0.Mul(&ql, &a); t1.Mul(&qr, &b)
					t2.Mul(&qm, &a); t2.Mul(&t2, &b)
					t3.Mul(&qo, &o)
					gate.Add(&t0, &t1).Add(&gate, &t2).Add(&gate, &t3).Add(&gate, &qk)
				}
				var boundary fr.Element
				{
					var t fr.Element
					t.Sub(&z, &one)
					boundary.Mul(&l0, &t)
				}
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

				var num fr.Element
				var tmp fr.Element
				tmp.Mul(&lambda, &boundary).Add(&tmp, &perm)
				num.Mul(&lambda, &tmp).Add(&num, &gate)

				h[j].Mul(&num, &vanXCosetInv[j])
			}
			pk.DomainXL.FFTInverse(h, fft.DIT, fft.OnCoset())

			hx1 := make([]fr.Element, T)
			hx2 := make([]fr.Element, T)
			hx3 := make([]fr.Element, T)
			copy(hx1, h[0:T])
			copy(hx2, h[T:2*T])
			copy(hx3, h[2*T:3*T])
			qResults[i] = quotientResult{hx1, hx2, hx3}

			hx1Lag := piano.CanonicalToLagrange(hx1, &pk.DomainX)
			hx2Lag := piano.CanonicalToLagrange(hx2, &pk.DomainX)
			hx3Lag := piano.CanonicalToLagrange(hx3, &pk.DomainX)

			var e error
			comHx1[i], e = dkzg.CommitLocal(uint64(i), hx1Lag, pk.DKZGSRS)
			if e != nil {
				errCh3 <- e
				return
			}
			comHx2[i], e = dkzg.CommitLocal(uint64(i), hx2Lag, pk.DKZGSRS)
			if e != nil {
				errCh3 <- e
				return
			}
			comHx3[i], e = dkzg.CommitLocal(uint64(i), hx3Lag, pk.DKZGSRS)
			if e != nil {
				errCh3 <- e
			}
		}(i)
	}
	wg3.Wait()
	close(errCh3)
	if err := <-errCh3; err != nil {
		return nil, err
	}

	proof.Hx[0], err = dkzg.AggregateDigests(comHx1)
	if err != nil {
		return nil, err
	}
	proof.Hx[1], err = dkzg.AggregateDigests(comHx2)
	if err != nil {
		return nil, err
	}
	proof.Hx[2], err = dkzg.AggregateDigests(comHx3)
	if err != nil {
		return nil, err
	}

	alpha, err := deriveChallengeBP(fs, "alpha", []dkzg.Digest{proof.Hx[0], proof.Hx[1], proof.Hx[2]})
	if err != nil {
		return nil, err
	}

	// ── 第四轮：Shplonk X 轴聚合 ─────────────────────────────────────────────
	alphaShifted := new(fr.Element)
	alphaShifted.Mul(&alpha, &pk.DomainX.Generator)

	alphaPowT := new(fr.Element)
	{
		bT := new(big.Int).SetInt64(int64(T))
		alphaPowT.Exp(alpha, bT)
	}

	// 计算共享多项式在 α 处的求值。
	qlAlpha := piano.EvalPolyLagrange(pk.Ql, alpha, pk.DomainX.Generator)
	qrAlpha := piano.EvalPolyLagrange(pk.Qr, alpha, pk.DomainX.Generator)
	qmAlpha := piano.EvalPolyLagrange(pk.Qm, alpha, pk.DomainX.Generator)
	qoAlpha := piano.EvalPolyLagrange(pk.Qo, alpha, pk.DomainX.Generator)
	qkAlpha := piano.EvalPolyLagrange(pk.Qk, alpha, pk.DomainX.Generator)
	s1Alpha := piano.EvalPolyLagrange(pk.S1, alpha, pk.DomainX.Generator)
	s2Alpha := piano.EvalPolyLagrange(pk.S2, alpha, pk.DomainX.Generator)
	s3Alpha := piano.EvalPolyLagrange(pk.S3, alpha, pk.DomainX.Generator)

	// foldedHx 承诺。
	foldedHxDig := piano.FoldDigests(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)

	// 从 (alpha, foldedHxDig, LRO[0..2], Z) — S_α ∪ S_ω 承诺 — 派生 ν。
	nuDigests := []dkzg.Digest{foldedHxDig, proof.LRO[0], proof.LRO[1], proof.LRO[2], proof.Z}
	nu, err := deriveChallengeBP(fs, "nu", nuDigests)
	if err != nil {
		return nil, err
	}

	// 每个子节点：计算 Shplonk 商 Q_{X,i} 并在 α 处求值所有 S_α 多项式。
	type shplonkResult struct {
		aAlpha, bAlpha, oAlpha fr.Element
		zAlpha, zsAlpha        fr.Element
		fhxAlpha               fr.Element
		comQX                  bn254.G1Affine
	}
	shResults := make([]shplonkResult, M)
	aVec := make([]fr.Element, M)
	bVec := make([]fr.Element, M)
	oVec := make([]fr.Element, M)
	zVec := make([]fr.Element, M)
	zsVec := make([]fr.Element, M)
	fhxVec := make([]fr.Element, M)
	comQXLocal := make([]dkzg.Digest, M)

	errCh4 := make(chan error, M)
	var wg4 sync.WaitGroup

	// 预计算 ν 的幂次：[ν^0..ν^13]，用于 13 个 S_α 多项式和 S_ω 的 ν^13。
	nuPow := buildNuPow(nu, 14)

	for i := 0; i < M; i++ {
		wg4.Add(1)
		go func(i int) {
			defer wg4.Done()
			sr := &shResults[i]
			gen := pk.DomainX.Generator

			// 折叠 fhx 规范系数形式。
			fhxCan := piano.FoldQuotient(qResults[i].hx1, qResults[i].hx2, qResults[i].hx3, *alphaPowT)
			fhxLag := piano.CanonicalToLagrange(fhxCan, &pk.DomainX)

			// 在 α 和 ω·α 处求值各多项式。
			sr.fhxAlpha = piano.EvalPolyCanonical(fhxCan, alpha)
			sr.aAlpha = piano.EvalPolyLagrange(witnesses[i].L, alpha, gen)
			sr.bAlpha = piano.EvalPolyLagrange(witnesses[i].R, alpha, gen)
			sr.oAlpha = piano.EvalPolyLagrange(witnesses[i].O, alpha, gen)
			sr.zAlpha = piano.EvalPolyLagrange(zLagrange[i], alpha, gen)
			sr.zsAlpha = piano.EvalPolyLagrange(zLagrange[i], *alphaShifted, gen)

			// 以 Lagrange 形式构建 Shplonk 商 Q_{X,i}。
			// S_α = {fhx,A,B,O,Z,Ql,Qr,Qm,Qo,Qk,S1,S2,S3}（13 个多项式）在 α 处。
			// S_ω = {Z} 在 ω·α 处，权重为 ν^13。
			polys := [][]fr.Element{
				fhxLag, witnesses[i].L, witnesses[i].R, witnesses[i].O, zLagrange[i],
				pk.Ql, pk.Qr, pk.Qm, pk.Qo, pk.Qk, pk.S1, pk.S2, pk.S3,
			}
			evals := []fr.Element{
				sr.fhxAlpha, sr.aAlpha, sr.bAlpha, sr.oAlpha, sr.zAlpha,
				qlAlpha, qrAlpha, qmAlpha, qoAlpha, qkAlpha, s1Alpha, s2Alpha, s3Alpha,
			}

			qxi := computeShplonkQuotient(polys, evals, nuPow[:13], alpha,
				zLagrange[i], sr.zsAlpha, nuPow[13], *alphaShifted, uint64(T), gen)

			// 承诺 Q_{X,i}。
			var e error
			comQXLocal[i], e = dkzg.CommitLocal(uint64(i), qxi, pk.DKZGSRS)
			if e != nil {
				errCh4 <- e
			}
		}(i)
	}
	wg4.Wait()
	close(errCh4)
	if err := <-errCh4; err != nil {
		return nil, err
	}

	// 聚合 ComQX。
	proof.ComQX, err = dkzg.AggregateDigests(comQXLocal)
	if err != nil {
		return nil, err
	}

	// 收集求值向量。
	for i := 0; i < M; i++ {
		aVec[i] = shResults[i].aAlpha
		bVec[i] = shResults[i].bAlpha
		oVec[i] = shResults[i].oAlpha
		zVec[i] = shResults[i].zAlpha
		zsVec[i] = shResults[i].zsAlpha
		fhxVec[i] = shResults[i].fhxAlpha
	}

	// ── 第五轮：Y 轴商多项式（与 Piano 相同）────────────────────────────────
	aCanY := piano.LagrangeToCanonical(aVec, &pk.DomainY)
	bCanY := piano.LagrangeToCanonical(bVec, &pk.DomainY)
	oCanY := piano.LagrangeToCanonical(oVec, &pk.DomainY)
	zCanY := piano.LagrangeToCanonical(zVec, &pk.DomainY)
	zsCanY := piano.LagrangeToCanonical(zsVec, &pk.DomainY)
	fhxCanY := piano.LagrangeToCanonical(fhxVec, &pk.DomainY)

	aCosetY := piano.CosetEval(aCanY, &pk.DomainYL)
	bCosetY := piano.CosetEval(bCanY, &pk.DomainYL)
	oCosetY := piano.CosetEval(oCanY, &pk.DomainYL)
	zCosetY := piano.CosetEval(zCanY, &pk.DomainYL)
	zsCosetY := piano.CosetEval(zsCanY, &pk.DomainYL)
	fhxCosetY := piano.CosetEval(fhxCanY, &pk.DomainYL)

	vanishingX := new(fr.Element)
	{
		bT := new(big.Int).SetInt64(int64(T))
		vanishingX.Exp(alpha, bT)
		var one fr.Element
		one.SetOne()
		vanishingX.Sub(vanishingX, &one)
	}
	lagrange0Alpha := piano.ComputeLagrange0(alpha, pk.DomainX.CardinalityInv)
	u := pk.DomainX.FrMultiplicativeGen
	var uSq, idA, idB, idC fr.Element
	uSq.Square(&u)
	idA.Set(&alpha)
	idB.Mul(&u, &alpha)
	idC.Mul(&uSq, &alpha)

	etaIdA := new(fr.Element); etaIdA.Mul(&eta, &idA)
	etaIdB := new(fr.Element); etaIdB.Mul(&eta, &idB)
	etaIdC := new(fr.Element); etaIdC.Mul(&eta, &idC)
	etaS1 := new(fr.Element); etaS1.Mul(&eta, &s1Alpha)
	etaS2 := new(fr.Element); etaS2.Mul(&eta, &s2Alpha)
	etaS3 := new(fr.Element); etaS3.Mul(&eta, &s3Alpha)

	N4M := int(pk.DomainYL.Cardinality)
	hY := make([]fr.Element, N4M)
	vanYCoset := piano.VanishingOnCoset(&pk.DomainYL, uint64(M))
	fft.BitReverse(vanYCoset)
	vanYCosetInv := make([]fr.Element, len(vanYCoset))
	copy(vanYCosetInv, vanYCoset)
	vanYCosetInv = fr.BatchInvert(vanYCosetInv)

	var one fr.Element
	one.SetOne()

	for j := 0; j < N4M; j++ {
		a, b, o := aCosetY[j], bCosetY[j], oCosetY[j]
		z, zs := zCosetY[j], zsCosetY[j]
		fhx := fhxCosetY[j]

		var gate fr.Element
		{
			var t0, t1, t2, t3 fr.Element
			t0.Mul(&qlAlpha, &a); t1.Mul(&qrAlpha, &b)
			t2.Mul(&qmAlpha, &a); t2.Mul(&t2, &b)
			t3.Mul(&qoAlpha, &o)
			gate.Add(&t0, &t1).Add(&gate, &t2).Add(&gate, &t3).Add(&gate, &qkAlpha)
		}
		var boundary fr.Element
		{
			var t fr.Element
			t.Sub(&z, &one)
			boundary.Mul(&lagrange0Alpha, &t)
		}
		var F, G fr.Element
		{
			var f0, f1, f2 fr.Element
			f0.Add(etaIdA, &a).Add(&f0, &gamma)
			f1.Add(etaIdB, &b).Add(&f1, &gamma)
			f2.Add(etaIdC, &o).Add(&f2, &gamma)
			F.Mul(&f0, &f1).Mul(&F, &f2).Mul(&F, &z)

			var g0, g1, g2 fr.Element
			g0.Add(etaS1, &a).Add(&g0, &gamma)
			g1.Add(etaS2, &b).Add(&g1, &gamma)
			g2.Add(etaS3, &o).Add(&g2, &gamma)
			G.Mul(&g0, &g1).Mul(&G, &g2).Mul(&G, &zs)
		}
		var perm fr.Element
		perm.Sub(&G, &F)

		var num fr.Element
		{
			var tmp fr.Element
			tmp.Mul(&lambda, &boundary).Add(&tmp, &perm)
			num.Mul(&lambda, &tmp).Add(&num, &gate)
		}
		var subtractHx fr.Element
		subtractHx.Mul(vanishingX, &fhx)
		num.Sub(&num, &subtractHx)

		hY[j].Mul(&num, &vanYCosetInv[j])
	}
	pk.DomainYL.FFTInverse(hY, fft.DIT, fft.OnCoset())

	hy1 := make([]fr.Element, M)
	hy2 := make([]fr.Element, M)
	hy3 := make([]fr.Element, M)
	copy(hy1, hY[0:M])
	copy(hy2, hY[M:2*M])
	copy(hy3, hY[2*M:3*M])

	hy1Lag := piano.CanonicalToLagrange(hy1, &pk.DomainY)
	hy2Lag := piano.CanonicalToLagrange(hy2, &pk.DomainY)
	hy3Lag := piano.CanonicalToLagrange(hy3, &pk.DomainY)

	// 使用 Y 轴 Lagrange SRS 承诺 Hy 各部分。
	hyDig := [3]dkzg.Digest{}
	hyDig[0], err = piano.CommitY(hy1Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	hyDig[1], err = piano.CommitY(hy2Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	hyDig[2], err = piano.CommitY(hy3Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// 计算 witness 多项式求值向量的 Y 轴承诺。
	// 用于修正双变量 Shplonk：对 witness 多项式而言，CommitY(evalVec) ≠ eval·g1。
	comVFfhx, err := piano.CommitY(fhxVec, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	comVFA, err := piano.CommitY(aVec, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	comVFB, err := piano.CommitY(bVec, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	comVFO, err := piano.CommitY(oVec, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	comVFZ, err := piano.CommitY(zVec, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	proof.ComVFZS, err = piano.CommitY(zsVec, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// ComVFAlpha = Σ_{k=0}^{4} ν^k · com_VF_k（G1 线性组合）
	{
		comVFs := []dkzg.Digest{comVFfhx, comVFA, comVFB, comVFO, comVFZ}
		var acc bn254.G1Jac
		for k, com := range comVFs {
			var j bn254.G1Jac
			j.FromAffine(&com)
			var b big.Int
			nuPow[k].BigInt(&b)
			j.ScalarMultiplication(&j, &b)
			acc.AddAssign(&j)
		}
		proof.ComVFAlpha.FromJacobian(&acc)
	}

	// 从 ComQX、ComVFAlpha、ComVFZS 派生 β——三者均参与绑定。
	beta, err := deriveChallengeBP(fs, "beta", []dkzg.Digest{proof.ComQX, proof.ComVFAlpha, proof.ComVFZS})
	if err != nil {
		return nil, err
	}

	// ── 第六轮：Y 轴聚合 ──────────────────────────────────────────────────────
	betaPowM := new(fr.Element)
	{
		bM := new(big.Int).SetInt64(int64(M))
		betaPowM.Exp(beta, bM)
	}
	foldedHyLag := piano.FoldQuotientLagrange(hy1Lag, hy2Lag, hy3Lag, *betaPowM)
	foldedHyDig := piano.FoldDigests(hyDig[0], hyDig[1], hyDig[2], betaPowM)

	// 7 个 Y 轴多项式：
	// P_0 = fhxVec, P_1 = aVec, P_2 = bVec, P_3 = oVec,
	// P_4 = zVec, P_5 = zsVec, P_6 = foldedHyLag
	yPolys := [][]fr.Element{fhxVec, aVec, bVec, oVec, zVec, zsVec, foldedHyLag}

	// 从当前转录状态派生 μ（在 beta 之后，无需额外绑定）。
	mu, err := deriveChallengeBP(fs, "mu", nil)
	if err != nil {
		return nil, err
	}

	// 构建 G_Y(Y) 的 Lagrange 向量：G_Y[j] = Σ_k μ^k * P_k[j]。
	muPow := buildNuPow(mu, 7)
	gYLag := make([]fr.Element, M)
	for k := 0; k < 7; k++ {
		for j := 0; j < M; j++ {
			var term fr.Element
			term.Mul(&muPow[k], &yPolys[k][j])
			gYLag[j].Add(&gYLag[j], &term)
		}
	}

	// ComGY = commitY(G_Y_lag)。
	proof.ComGY, err = piano.CommitY(gYLag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// 在 β 处求值各 Y 轴多项式，计算 G_Y(β)。
	lagBeta, err := evalLagrangeBasisY(beta, uint64(M), pk.DomainY.Generator)
	if err != nil {
		return nil, fmt.Errorf("bpiano: beta 是 Y 轴单位根")
	}
	yEvals := make([]fr.Element, 7)
	for k := 0; k < 7; k++ {
		for j := 0; j < M; j++ {
			var term fr.Element
			term.Mul(&yPolys[k][j], &lagBeta[j])
			yEvals[k].Add(&yEvals[k], &term)
		}
	}
	// gYBeta = Σ_k μ^k * P_k(β)
	var gYBeta fr.Element
	for k := 0; k < 7; k++ {
		var term fr.Element
		term.Mul(&muPow[k], &yEvals[k])
		gYBeta.Add(&gYBeta, &term)
	}

	// 计算 Pi1AggH = G_Y 在 β 处的 Y 轴商。
	// q_j = (G_Y[j] - G_Y(β)) / (ω_Y^j - β)
	genY := pk.DomainY.Generator
	qY := make([]fr.Element, M)
	denomsY := make([]fr.Element, M)
	var genYPow fr.Element
	genYPow.SetOne()
	for j := 0; j < M; j++ {
		qY[j].Sub(&gYLag[j], &gYBeta)
		denomsY[j].Sub(&genYPow, &beta)
		genYPow.Mul(&genYPow, &genY)
	}
	denomsY = fr.BatchInvert(denomsY)
	for j := 0; j < M; j++ {
		qY[j].Mul(&qY[j], &denomsY[j])
	}
	// Pi1AggH = qY 与 Vy 的 MSM。
	proof.Pi1AggH, err = piano.CommitY(qY, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// 填写标量求值值。
	proof.EvalHx = yEvals[0]
	proof.EvalA = yEvals[1]
	proof.EvalB = yEvals[2]
	proof.EvalO = yEvals[3]
	proof.EvalZ = yEvals[4]
	proof.EvalZS = yEvals[5]
	proof.EvalHy = yEvals[6]

	// 填写共享多项式在 α 处的求值（包含在 Shplonk 验证中）。
	proof.EvalQl = qlAlpha
	proof.EvalQr = qrAlpha
	proof.EvalQm = qmAlpha
	proof.EvalQo = qoAlpha
	proof.EvalQk = qkAlpha
	proof.EvalS1 = s1Alpha
	proof.EvalS2 = s2Alpha
	proof.EvalS3 = s3Alpha

	// 验证 Y 轴约束（调试检查）。
	if err := checkConstraintBP(alpha, beta, proof, eta, gamma, lambda, pk,
		qlAlpha, qrAlpha, qmAlpha, qoAlpha, qkAlpha, s1Alpha, s2Alpha, s3Alpha,
		foldedHyDig, foldedHxDig, betaPowM); err != nil {
		return nil, err
	}

	return proof, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ────────────────────────────────────────────────────────────────────────────

// computeShplonkQuotient 以 Lagrange 形式计算 Shplonk 聚合商多项式 Q_{X,i}(X)：
//
//	Q(X) = Σ_{k=0..len(polys)-1} nuPow[k]*(polys[k][j]-evals[k])/(ω^j - alpha)
//	     + nuZS*(zPoly[j]-zsEval)/(ω^j - alphaShifted)
//
// 所有多项式均为大小 T 域上的 Lagrange 形式。
func computeShplonkQuotient(
	polys [][]fr.Element, evals []fr.Element, nuPow []fr.Element,
	alpha fr.Element,
	zPoly []fr.Element, zsEval fr.Element, nuZS fr.Element, alphaShifted fr.Element,
	T uint64, gen fr.Element,
) []fr.Element {
	// 各 j 的分母：(ω^j - alpha) 和 (ω^j - alphaShifted)。
	denomsA := make([]fr.Element, T) // ω^j - alpha
	denomsB := make([]fr.Element, T) // ω^j - alphaShifted
	var genPow fr.Element
	genPow.SetOne()
	for j := uint64(0); j < T; j++ {
		denomsA[j].Sub(&genPow, &alpha)
		denomsB[j].Sub(&genPow, &alphaShifted)
		genPow.Mul(&genPow, &gen)
	}
	denomsA = fr.BatchInvert(denomsA)
	denomsB = fr.BatchInvert(denomsB)

	q := make([]fr.Element, T)
	// S_α 项。
	for k, poly := range polys {
		for j := uint64(0); j < T; j++ {
			var numer, term fr.Element
			numer.Sub(&poly[j], &evals[k])
			term.Mul(&numer, &denomsA[j])
			term.Mul(&term, &nuPow[k])
			q[j].Add(&q[j], &term)
		}
	}
	// S_ω 项（Z 在 alphaShifted 处）。
	for j := uint64(0); j < T; j++ {
		var numer, term fr.Element
		numer.Sub(&zPoly[j], &zsEval)
		term.Mul(&numer, &denomsB[j])
		term.Mul(&term, &nuZS)
		q[j].Add(&q[j], &term)
	}
	return q
}

// buildNuPow 返回 [ν^0, ν^1, ..., ν^{K-1}]。
func buildNuPow(nu fr.Element, K int) []fr.Element {
	pows := make([]fr.Element, K)
	pows[0].SetOne()
	for k := 1; k < K; k++ {
		pows[k].Mul(&pows[k-1], &nu)
	}
	return pows
}

// evalLagrangeBasisY 在 Y 域上计算 β 处的所有 Lagrange 基值。
func evalLagrangeBasisY(beta fr.Element, M uint64, genY fr.Element) ([]fr.Element, error) {
	var betaN fr.Element
	betaN.Exp(beta, new(big.Int).SetUint64(M))
	var one fr.Element
	one.SetOne()
	var numer fr.Element
	numer.Sub(&betaN, &one)
	if numer.IsZero() {
		return nil, fmt.Errorf("beta 是单位根")
	}
	var nInv fr.Element
	nInv.SetUint64(M).Inverse(&nInv)
	numer.Mul(&numer, &nInv)

	denoms := make([]fr.Element, M)
	genPows := make([]fr.Element, M)
	var gp fr.Element
	gp.SetOne()
	for j := uint64(0); j < M; j++ {
		genPows[j] = gp
		denoms[j].Sub(&beta, &gp)
		gp.Mul(&gp, &genY)
	}
	denoms = fr.BatchInvert(denoms)
	result := make([]fr.Element, M)
	for j := uint64(0); j < M; j++ {
		result[j].Mul(&numer, &denoms[j])
		result[j].Mul(&result[j], &genPows[j])
	}
	return result, nil
}

// bindPublicDataBP 将公开数据写入 Fiat-Shamir 转录。
func bindPublicDataBP(fs *fiatshamir.Transcript, challenge string, vk *piano.VerifyingKey, publicInputs [][]fr.Element) error {
	for _, com := range []dkzg.Digest{vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3} {
		b := com.Bytes()
		if err := fs.Bind(challenge, b[:]); err != nil {
			return err
		}
	}
	for _, row := range publicInputs {
		for _, v := range row {
			b := v.Bytes()
			if err := fs.Bind(challenge, b[:]); err != nil {
				return err
			}
		}
	}
	return nil
}

// deriveChallengeBP 绑定承诺后从 Fiat-Shamir 转录中导出挑战值。
func deriveChallengeBP(fs *fiatshamir.Transcript, label string, digests []dkzg.Digest) (fr.Element, error) {
	for _, d := range digests {
		b := d.Bytes()
		if err := fs.Bind(label, b[:]); err != nil {
			return fr.Element{}, err
		}
	}
	b, err := fs.ComputeChallenge(label)
	if err != nil {
		return fr.Element{}, err
	}
	var challenge fr.Element
	challenge.SetBytes(b)
	return challenge, nil
}

// checkConstraintBP 验证 BPiano 的 Y 轴代数约束。
func checkConstraintBP(
	alpha, beta fr.Element,
	proof *CompressedProof,
	eta, gamma, lambda fr.Element,
	pk *piano.ProvingKey,
	qlAlpha, qrAlpha, qmAlpha, qoAlpha, qkAlpha fr.Element,
	s1Alpha, s2Alpha, s3Alpha fr.Element,
	foldedHyDig, foldedHxDig dkzg.Digest,
	betaPowM *fr.Element,
) error {
	T := int(pk.Vk.SizeX)
	M := int(pk.Vk.SizeY)

	a := proof.EvalA
	b := proof.EvalB
	o := proof.EvalO
	z := proof.EvalZ
	zs := proof.EvalZS
	hx := proof.EvalHx
	hy := proof.EvalHy

	var one fr.Element
	one.SetOne()

	var gate fr.Element
	{
		var t0, t1, t2, t3 fr.Element
		t0.Mul(&qlAlpha, &a); t1.Mul(&qrAlpha, &b)
		t2.Mul(&qmAlpha, &a); t2.Mul(&t2, &b)
		t3.Mul(&qoAlpha, &o)
		gate.Add(&t0, &t1).Add(&gate, &t2).Add(&gate, &t3).Add(&gate, &qkAlpha)
	}
	l0 := piano.ComputeLagrange0(alpha, pk.DomainX.CardinalityInv)
	var boundary fr.Element
	{
		var t fr.Element
		t.Sub(&z, &one)
		boundary.Mul(&l0, &t)
	}
	u := pk.DomainX.FrMultiplicativeGen
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
		g0.Mul(&eta, &s1Alpha).Add(&g0, &a).Add(&g0, &gamma)
		g1.Mul(&eta, &s2Alpha).Add(&g1, &b).Add(&g1, &gamma)
		g2.Mul(&eta, &s3Alpha).Add(&g2, &o).Add(&g2, &gamma)
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
		return fmt.Errorf("bpiano: 代数约束不满足：lhs=%s rhs=%s",
			lhs.String(), rhs.String())
	}
	return nil
}
