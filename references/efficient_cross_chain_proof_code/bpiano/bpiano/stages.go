package bpiano

// stages.go 实现 BPiano 证明聚合所需的三阶段证明生成函数。
//
// 现有 Compress 函数一字不改。三个新函数将 Compress 的逻辑拆分为：
//
//   CompressStage1：witness 承诺 → Z → X 轴商多项式 → Hx 承诺
//                   停在 α 派生之前，输出 ProveState1
//
//   CompressStage2：接受外部共享 α；完成 Shplonk X 轴聚合和 Y 轴商多项式
//                   停在 β 派生之前，输出 ProveState2
//
//   CompressStage3：接受外部共享 β；完成 Y 轴聚合，输出 CompressedProof
//
// 由 CoordinateChallenges（aggregate.go）调用，实现 §4.3.1 两轮挑战协调。

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

// ────────────────────────────────────────────────────────────────────────────
// 中间状态类型
// ────────────────────────────────────────────────────────────────────────────

// hx123Polys 保存单个子节点的 X 轴商多项式的规范系数形式。
type hx123Polys struct {
	hx1, hx2, hx3 []fr.Element // 规范系数形式，长度 T
}

// ProveState1 是 CompressStage1 的输出，保存继续完成证明所需的全部中间状态。
// 外部协调节点可从中读取 Hx[0..2] 参与共享 α 的派生。
type ProveState1 struct {
	// 原始输入（Stage2/3 继续使用）
	pk           *piano.ProvingKey
	witnesses    []piano.WitnessInstance
	publicInputs [][]fr.Element

	// 已派生的挑战（γ/η/λ 不依赖 α，保留供 Stage2/3 使用）
	gamma, eta, lambda fr.Element

	// 已计算的承诺（对应 CompressedProof 中同名字段）
	lro [3]dkzg.Digest
	z   dkzg.Digest
	hx  [3]dkzg.Digest

	// 每个子节点的中间多项式（Stage2 Shplonk 计算需要）
	zLagrange [][]fr.Element // zLagrange[i]：第 i 个子节点的置换累加器 Lagrange 向量
	hx123     []hx123Polys  // hx1/hx2/hx3 规范系数（M 组，Stage2 折叠 fhx 需要）
}

// ProveState2 是 CompressStage2 的输出，在 ProveState1 基础上追加 X 轴聚合结果。
// 外部协调节点可从中读取 ComQX/ComVFAlpha/ComVFZS 参与共享 β 的派生。
type ProveState2 struct {
	ProveState1

	// 协调输入的共享挑战
	sharedAlpha  fr.Element
	alphaShifted fr.Element
	alphaPowT    fr.Element

	// Stage2 派生的 per-proof 挑战
	nu    fr.Element
	nuPow []fr.Element // [ν^0..ν^13]，长度 14

	// Stage2 产出的承诺（对应 CompressedProof 中同名字段）
	comQX      dkzg.Digest
	comVFAlpha dkzg.Digest
	comVFZS    dkzg.Digest

	// 共享多项式在 α 处的求值（Stage3 Y 轴约束计算需要）
	qlAlpha, qrAlpha, qmAlpha, qoAlpha, qkAlpha fr.Element
	s1Alpha, s2Alpha, s3Alpha                   fr.Element

	// X 轴求值向量（长度 M，Stage3 Y 轴聚合需要）
	aVec, bVec, oVec, zVec, zsVec, fhxVec []fr.Element

	// foldedHxDig（VerifyBatch 重派生 nu_k 时需要，Stage3 也用于 checkConstraint）
	foldedHxDig dkzg.Digest

	// Hy 承诺与 Lagrange（Stage3 Y 轴折叠需要）
	hyDig               [3]dkzg.Digest
	hy1Lag, hy2Lag, hy3Lag []fr.Element
}

// ────────────────────────────────────────────────────────────────────────────
// CompressStage1
// ────────────────────────────────────────────────────────────────────────────

// CompressStage1 执行证明生成第一阶段：
//
//	witness 承诺 → Z 多项式 → X 轴商多项式 → Hx 承诺
//
// 停在 α 派生之前。返回的 ProveState1.hx 即为向协调节点提交的 Hx[0..2]。
// 获得共享 α 后调用 CompressStage2 继续。
func CompressStage1(
	pk *piano.ProvingKey,
	witnesses []piano.WitnessInstance,
	publicInputs [][]fr.Element,
) (*ProveState1, error) {
	M := int(pk.Vk.SizeY)
	T := int(pk.Vk.SizeX)

	if len(witnesses) != M {
		return nil, fmt.Errorf("bpiano stage1: 收到 %d 个 witness，期望 %d", len(witnesses), M)
	}

	// ── FS 转录（gamma/eta/lambda，与原 Compress 完全相同格式）──────────────
	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc,
		"gamma", "eta", "lambda", "alpha", "nu", "beta", "mu")

	// ── witness 承诺（并行）──────────────────────────────────────────────────
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
	state := &ProveState1{
		pk:           pk,
		witnesses:    witnesses,
		publicInputs: publicInputs,
		zLagrange:    make([][]fr.Element, M),
		hx123:        make([]hx123Polys, M),
	}

	state.lro[0], err = dkzg.AggregateDigests(comA)
	if err != nil {
		return nil, err
	}
	state.lro[1], err = dkzg.AggregateDigests(comB)
	if err != nil {
		return nil, err
	}
	state.lro[2], err = dkzg.AggregateDigests(comO)
	if err != nil {
		return nil, err
	}

	if err := bindPublicDataBP(fs, "gamma", pk.Vk, publicInputs); err != nil {
		return nil, err
	}
	state.gamma, err = deriveChallengeBP(fs, "gamma",
		[]dkzg.Digest{state.lro[0], state.lro[1], state.lro[2]})
	if err != nil {
		return nil, err
	}
	state.eta, err = deriveChallengeBP(fs, "eta", nil)
	if err != nil {
		return nil, err
	}

	// ── Z 多项式（并行）──────────────────────────────────────────────────────
	comZ := make([]dkzg.Digest, M)
	errCh2 := make(chan error, M)
	var wg2 sync.WaitGroup
	for i := 0; i < M; i++ {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			z, err := piano.ComputeZLagrange(
				witnesses[i].L, witnesses[i].R, witnesses[i].O,
				pk, state.eta, state.gamma)
			if err != nil {
				errCh2 <- err
				return
			}
			state.zLagrange[i] = z
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

	state.z, err = dkzg.AggregateDigests(comZ)
	if err != nil {
		return nil, err
	}
	state.lambda, err = deriveChallengeBP(fs, "lambda", []dkzg.Digest{state.z})
	if err != nil {
		return nil, err
	}

	// ── X 轴商多项式（并行）─────────────────────────────────────────────────
	// 预计算 coset 数据（仅依赖 pk）
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
			zCan := piano.LagrangeToCanonical(state.zLagrange[i], &pk.DomainX)
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
					t0.Mul(&ql, &a)
					t1.Mul(&qr, &b)
					t2.Mul(&qm, &a)
					t2.Mul(&t2, &b)
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
					f0.Mul(&state.eta, &idA).Add(&f0, &a).Add(&f0, &state.gamma)
					f1.Mul(&state.eta, &idB).Add(&f1, &b).Add(&f1, &state.gamma)
					f2.Mul(&state.eta, &idC).Add(&f2, &o).Add(&f2, &state.gamma)
					F.Mul(&f0, &f1).Mul(&F, &f2).Mul(&F, &z)

					var g0, g1, g2 fr.Element
					g0.Mul(&state.eta, &s1).Add(&g0, &a).Add(&g0, &state.gamma)
					g1.Mul(&state.eta, &s2).Add(&g1, &b).Add(&g1, &state.gamma)
					g2.Mul(&state.eta, &s3).Add(&g2, &o).Add(&g2, &state.gamma)
					G.Mul(&g0, &g1).Mul(&G, &g2).Mul(&G, &zs)
				}
				var perm fr.Element
				perm.Sub(&G, &F)

				var num fr.Element
				{
					var tmp fr.Element
					tmp.Mul(&state.lambda, &boundary).Add(&tmp, &perm)
					num.Mul(&state.lambda, &tmp).Add(&num, &gate)
				}
				h[j].Mul(&num, &vanXCosetInv[j])
			}
			pk.DomainXL.FFTInverse(h, fft.DIT, fft.OnCoset())

			hx1 := make([]fr.Element, T)
			hx2 := make([]fr.Element, T)
			hx3 := make([]fr.Element, T)
			copy(hx1, h[0:T])
			copy(hx2, h[T:2*T])
			copy(hx3, h[2*T:3*T])
			state.hx123[i] = hx123Polys{hx1, hx2, hx3}

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

	state.hx[0], err = dkzg.AggregateDigests(comHx1)
	if err != nil {
		return nil, err
	}
	state.hx[1], err = dkzg.AggregateDigests(comHx2)
	if err != nil {
		return nil, err
	}
	state.hx[2], err = dkzg.AggregateDigests(comHx3)
	if err != nil {
		return nil, err
	}

	return state, nil
}

// ────────────────────────────────────────────────────────────────────────────
// CompressStage2
// ────────────────────────────────────────────────────────────────────────────

// CompressStage2 在外部共享 α 下执行第二阶段：
//
//	Shplonk X 轴聚合 → ComQX/ComVFAlpha/ComVFZS
//	Y 轴商多项式 → Hy 承诺
//
// 停在 β 派生之前。返回的 ProveState2 包含向协调节点提交的
// ComQX/ComVFAlpha/ComVFZS，以及全部 Hy 中间结果。
// 获得共享 β 后调用 CompressStage3 完成证明。
func CompressStage2(state *ProveState1, sharedAlpha fr.Element) (*ProveState2, error) {
	pk := state.pk
	M := int(pk.Vk.SizeY)
	T := int(pk.Vk.SizeX)

	s2 := &ProveState2{
		ProveState1: *state,
		sharedAlpha: sharedAlpha,
	}

	s2.alphaShifted.Mul(&sharedAlpha, &pk.DomainX.Generator)
	{
		bT := new(big.Int).SetInt64(int64(T))
		s2.alphaPowT.Exp(sharedAlpha, bT)
	}

	// ── 共享多项式在 α 处的求值 ──────────────────────────────────────────────
	gen := pk.DomainX.Generator
	s2.qlAlpha = piano.EvalPolyLagrange(pk.Ql, sharedAlpha, gen)
	s2.qrAlpha = piano.EvalPolyLagrange(pk.Qr, sharedAlpha, gen)
	s2.qmAlpha = piano.EvalPolyLagrange(pk.Qm, sharedAlpha, gen)
	s2.qoAlpha = piano.EvalPolyLagrange(pk.Qo, sharedAlpha, gen)
	s2.qkAlpha = piano.EvalPolyLagrange(pk.Qk, sharedAlpha, gen)
	s2.s1Alpha = piano.EvalPolyLagrange(pk.S1, sharedAlpha, gen)
	s2.s2Alpha = piano.EvalPolyLagrange(pk.S2, sharedAlpha, gen)
	s2.s3Alpha = piano.EvalPolyLagrange(pk.S3, sharedAlpha, gen)

	// ── foldedHxDig（用于 nu 派生和 Stage3 约束检查）────────────────────────
	s2.foldedHxDig = piano.FoldDigests(
		state.hx[0], state.hx[1], state.hx[2], &s2.alphaPowT)

	// ── nu：per-proof 确定性派生（绑定 sharedAlpha + foldedHxDig + LRO + Z）──
	s2.nu = deriveNuCoord(sharedAlpha, s2.foldedHxDig, state.lro, state.z)
	s2.nuPow = buildNuPow(s2.nu, 14)

	// ── Shplonk X 轴聚合（并行）──────────────────────────────────────────────
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
	for i := 0; i < M; i++ {
		wg4.Add(1)
		go func(i int) {
			defer wg4.Done()
			sr := &shResults[i]

			fhxCan := piano.FoldQuotient(
				state.hx123[i].hx1, state.hx123[i].hx2, state.hx123[i].hx3,
				s2.alphaPowT)
			fhxLag := piano.CanonicalToLagrange(fhxCan, &pk.DomainX)

			sr.fhxAlpha = piano.EvalPolyCanonical(fhxCan, sharedAlpha)
			sr.aAlpha = piano.EvalPolyLagrange(state.witnesses[i].L, sharedAlpha, gen)
			sr.bAlpha = piano.EvalPolyLagrange(state.witnesses[i].R, sharedAlpha, gen)
			sr.oAlpha = piano.EvalPolyLagrange(state.witnesses[i].O, sharedAlpha, gen)
			sr.zAlpha = piano.EvalPolyLagrange(state.zLagrange[i], sharedAlpha, gen)
			sr.zsAlpha = piano.EvalPolyLagrange(state.zLagrange[i], s2.alphaShifted, gen)

			polys := [][]fr.Element{
				fhxLag, state.witnesses[i].L, state.witnesses[i].R,
				state.witnesses[i].O, state.zLagrange[i],
				pk.Ql, pk.Qr, pk.Qm, pk.Qo, pk.Qk, pk.S1, pk.S2, pk.S3,
			}
			evals := []fr.Element{
				sr.fhxAlpha, sr.aAlpha, sr.bAlpha, sr.oAlpha, sr.zAlpha,
				s2.qlAlpha, s2.qrAlpha, s2.qmAlpha, s2.qoAlpha, s2.qkAlpha,
				s2.s1Alpha, s2.s2Alpha, s2.s3Alpha,
			}
			qxi := computeShplonkQuotient(
				polys, evals, s2.nuPow[:13], sharedAlpha,
				state.zLagrange[i], sr.zsAlpha, s2.nuPow[13],
				s2.alphaShifted, uint64(T), gen)

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

	var err error
	s2.comQX, err = dkzg.AggregateDigests(comQXLocal)
	if err != nil {
		return nil, err
	}
	for i := 0; i < M; i++ {
		aVec[i] = shResults[i].aAlpha
		bVec[i] = shResults[i].bAlpha
		oVec[i] = shResults[i].oAlpha
		zVec[i] = shResults[i].zAlpha
		zsVec[i] = shResults[i].zsAlpha
		fhxVec[i] = shResults[i].fhxAlpha
	}
	s2.aVec = aVec
	s2.bVec = bVec
	s2.oVec = oVec
	s2.zVec = zVec
	s2.zsVec = zsVec
	s2.fhxVec = fhxVec

	// ── Y 轴商多项式（依赖 α，不依赖 β，在 Stage2 完成）─────────────────────
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
		vanishingX.Exp(sharedAlpha, bT)
		var one fr.Element
		one.SetOne()
		vanishingX.Sub(vanishingX, &one)
	}
	lagrange0Alpha := piano.ComputeLagrange0(sharedAlpha, pk.DomainX.CardinalityInv)
	u := pk.DomainX.FrMultiplicativeGen
	var uSq, idA, idB, idC fr.Element
	uSq.Square(&u)
	idA.Set(&sharedAlpha)
	idB.Mul(&u, &sharedAlpha)
	idC.Mul(&uSq, &sharedAlpha)

	etaIdA := new(fr.Element)
	etaIdA.Mul(&state.eta, &idA)
	etaIdB := new(fr.Element)
	etaIdB.Mul(&state.eta, &idB)
	etaIdC := new(fr.Element)
	etaIdC.Mul(&state.eta, &idC)
	etaS1 := new(fr.Element)
	etaS1.Mul(&state.eta, &s2.s1Alpha)
	etaS2 := new(fr.Element)
	etaS2.Mul(&state.eta, &s2.s2Alpha)
	etaS3 := new(fr.Element)
	etaS3.Mul(&state.eta, &s2.s3Alpha)

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
			t0.Mul(&s2.qlAlpha, &a)
			t1.Mul(&s2.qrAlpha, &b)
			t2.Mul(&s2.qmAlpha, &a)
			t2.Mul(&t2, &b)
			t3.Mul(&s2.qoAlpha, &o)
			gate.Add(&t0, &t1).Add(&gate, &t2).Add(&gate, &t3).Add(&gate, &s2.qkAlpha)
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
			f0.Add(etaIdA, &a).Add(&f0, &state.gamma)
			f1.Add(etaIdB, &b).Add(&f1, &state.gamma)
			f2.Add(etaIdC, &o).Add(&f2, &state.gamma)
			F.Mul(&f0, &f1).Mul(&F, &f2).Mul(&F, &z)

			var g0, g1, g2 fr.Element
			g0.Add(etaS1, &a).Add(&g0, &state.gamma)
			g1.Add(etaS2, &b).Add(&g1, &state.gamma)
			g2.Add(etaS3, &o).Add(&g2, &state.gamma)
			G.Mul(&g0, &g1).Mul(&G, &g2).Mul(&G, &zs)
		}
		var perm fr.Element
		perm.Sub(&G, &F)

		var num fr.Element
		{
			var tmp fr.Element
			tmp.Mul(&state.lambda, &boundary).Add(&tmp, &perm)
			num.Mul(&state.lambda, &tmp).Add(&num, &gate)
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

	s2.hy1Lag = piano.CanonicalToLagrange(hy1, &pk.DomainY)
	s2.hy2Lag = piano.CanonicalToLagrange(hy2, &pk.DomainY)
	s2.hy3Lag = piano.CanonicalToLagrange(hy3, &pk.DomainY)

	s2.hyDig[0], err = piano.CommitY(s2.hy1Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	s2.hyDig[1], err = piano.CommitY(s2.hy2Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	s2.hyDig[2], err = piano.CommitY(s2.hy3Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// ── ComVFZS 和 ComVFAlpha ─────────────────────────────────────────────────
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
	s2.comVFZS, err = piano.CommitY(zsVec, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	{
		comVFs := []dkzg.Digest{comVFfhx, comVFA, comVFB, comVFO, comVFZ}
		var acc bn254.G1Jac
		for k, com := range comVFs {
			var j bn254.G1Jac
			j.FromAffine(&com)
			var b big.Int
			s2.nuPow[k].BigInt(&b)
			j.ScalarMultiplication(&j, &b)
			acc.AddAssign(&j)
		}
		s2.comVFAlpha.FromJacobian(&acc)
	}

	return s2, nil
}

// ────────────────────────────────────────────────────────────────────────────
// CompressStage3
// ────────────────────────────────────────────────────────────────────────────

// CompressStage3 在外部共享 β 下完成证明生成：
//
//	Y 轴聚合 → ComGY → Pi1AggH → 填充求值字段
//
// 返回完整的 CompressedProof。
func CompressStage3(state *ProveState2, sharedBeta fr.Element) (*CompressedProof, error) {
	pk := state.pk
	M := int(pk.Vk.SizeY)

	proof := &CompressedProof{}

	// 将 Stage1/2 的承诺写入 proof
	proof.LRO = state.lro
	proof.Z = state.z
	proof.Hx = state.hx
	proof.ComQX = state.comQX
	proof.ComVFAlpha = state.comVFAlpha
	proof.ComVFZS = state.comVFZS

	// ── β 相关预计算 ──────────────────────────────────────────────────────────
	betaPowM := new(fr.Element)
	{
		bM := new(big.Int).SetInt64(int64(M))
		betaPowM.Exp(sharedBeta, bM)
	}
	foldedHyLag := piano.FoldQuotientLagrange(
		state.hy1Lag, state.hy2Lag, state.hy3Lag, *betaPowM)
	foldedHyDig := piano.FoldDigests(
		state.hyDig[0], state.hyDig[1], state.hyDig[2], betaPowM)

	// ── mu：per-proof 确定性派生（绑定 sharedBeta）───────────────────────────
	mu := deriveMuCoord(sharedBeta)
	muPow := buildNuPow(mu, 7)

	// ── G_Y 聚合 ──────────────────────────────────────────────────────────────
	yPolys := [][]fr.Element{
		state.fhxVec, state.aVec, state.bVec, state.oVec,
		state.zVec, state.zsVec, foldedHyLag,
	}

	gYLag := make([]fr.Element, M)
	for k := 0; k < 7; k++ {
		for j := 0; j < M; j++ {
			var term fr.Element
			term.Mul(&muPow[k], &yPolys[k][j])
			gYLag[j].Add(&gYLag[j], &term)
		}
	}

	var err error
	proof.ComGY, err = piano.CommitY(gYLag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// ── 在 β 处求值各 Y 轴多项式 ─────────────────────────────────────────────
	lagBeta, err := evalLagrangeBasisY(sharedBeta, uint64(M), pk.DomainY.Generator)
	if err != nil {
		return nil, fmt.Errorf("bpiano stage3: beta 是 Y 轴单位根")
	}
	yEvals := make([]fr.Element, 7)
	for k := 0; k < 7; k++ {
		for j := 0; j < M; j++ {
			var term fr.Element
			term.Mul(&yPolys[k][j], &lagBeta[j])
			yEvals[k].Add(&yEvals[k], &term)
		}
	}

	var gYBeta fr.Element
	for k := 0; k < 7; k++ {
		var term fr.Element
		term.Mul(&muPow[k], &yEvals[k])
		gYBeta.Add(&gYBeta, &term)
	}

	// ── Pi1AggH = KZG 开放证明 ────────────────────────────────────────────────
	genY := pk.DomainY.Generator
	qY := make([]fr.Element, M)
	denomsY := make([]fr.Element, M)
	var genYPow fr.Element
	genYPow.SetOne()
	for j := 0; j < M; j++ {
		qY[j].Sub(&gYLag[j], &gYBeta)
		denomsY[j].Sub(&genYPow, &sharedBeta)
		genYPow.Mul(&genYPow, &genY)
	}
	denomsY = fr.BatchInvert(denomsY)
	for j := 0; j < M; j++ {
		qY[j].Mul(&qY[j], &denomsY[j])
	}
	proof.Pi1AggH, err = piano.CommitY(qY, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// ── 填充求值字段 ──────────────────────────────────────────────────────────
	proof.EvalHx = yEvals[0]
	proof.EvalA = yEvals[1]
	proof.EvalB = yEvals[2]
	proof.EvalO = yEvals[3]
	proof.EvalZ = yEvals[4]
	proof.EvalZS = yEvals[5]
	proof.EvalHy = yEvals[6]

	proof.EvalQl = state.qlAlpha
	proof.EvalQr = state.qrAlpha
	proof.EvalQm = state.qmAlpha
	proof.EvalQo = state.qoAlpha
	proof.EvalQk = state.qkAlpha
	proof.EvalS1 = state.s1Alpha
	proof.EvalS2 = state.s2Alpha
	proof.EvalS3 = state.s3Alpha

	// ── 代数约束验证（调试检查）──────────────────────────────────────────────
	if err := checkConstraintBP(
		state.sharedAlpha, sharedBeta, proof,
		state.eta, state.gamma, state.lambda, pk,
		state.qlAlpha, state.qrAlpha, state.qmAlpha, state.qoAlpha, state.qkAlpha,
		state.s1Alpha, state.s2Alpha, state.s3Alpha,
		foldedHyDig, state.foldedHxDig, betaPowM,
	); err != nil {
		return nil, err
	}

	return proof, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 协调挑战派生辅助函数
// ────────────────────────────────────────────────────────────────────────────

// deriveNuCoord 为聚合方案派生 per-proof ν 挑战。
//
// 格式：SHA256("coord-nu" || sharedAlpha_bytes || foldedHxDig_bytes
//
//	|| lro[0]_bytes || lro[1]_bytes || lro[2]_bytes || z_bytes)
//
// VerifyBatch 使用完全相同的格式重新派生 ν_k。
func deriveNuCoord(
	sharedAlpha fr.Element,
	foldedHxDig dkzg.Digest,
	lro [3]dkzg.Digest,
	z dkzg.Digest,
) fr.Element {
	h := sha256.New()
	h.Write([]byte("coord-nu"))
	b := sharedAlpha.Bytes()
	h.Write(b[:])
	b2 := foldedHxDig.Bytes()
	h.Write(b2[:])
	for _, d := range lro {
		b3 := d.Bytes()
		h.Write(b3[:])
	}
	b4 := z.Bytes()
	h.Write(b4[:])
	var nu fr.Element
	nu.SetBytes(h.Sum(nil))
	return nu
}

// deriveMuCoord 为聚合方案派生 per-proof μ 挑战。
//
// 格式：SHA256("coord-mu" || sharedBeta_bytes)
//
// VerifyBatch 使用完全相同的格式重新派生 μ_k。
func deriveMuCoord(sharedBeta fr.Element) fr.Element {
	h := sha256.New()
	h.Write([]byte("coord-mu"))
	b := sharedBeta.Bytes()
	h.Write(b[:])
	var mu fr.Element
	mu.SetBytes(h.Sum(nil))
	return mu
}
