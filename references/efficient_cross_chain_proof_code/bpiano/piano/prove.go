package piano

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
)

// Prove 为 M 个 witness 实例生成 Piano 证明。
//
// witnesses 的长度须为 M = vk.SizeY，每个实例的 L/R/O 切片长度为
// T = vk.SizeX，以 Lagrange 形式存储（evals[j] = poly(ω_X^j)）。
//
// publicInputs[i] 是第 i 个子电路的公开输入（长度均为 vk.NbPublicInputs）。
// 若无公开输入，传 nil 即可。
func Prove(pk *ProvingKey, witnesses []WitnessInstance, publicInputs [][]fr.Element) (*Proof, error) {
	M := int(pk.Vk.SizeY)
	T := int(pk.Vk.SizeX)

	if len(witnesses) != M {
		return nil, fmt.Errorf("piano: 收到 %d 个 witness，期望 %d", len(witnesses), M)
	}

	// ── Fiat-Shamir 转录 ──────────────────────────────────────────────────────
	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc, "gamma", "eta", "lambda", "alpha", "beta")

	proof := &Proof{}

	// ── 第一轮：witness 承诺 ──────────────────────────────────────────────────
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

	// 绑定公开数据并派生挑战 γ、η。
	if err := bindPublicData(fs, "gamma", pk.Vk, publicInputs); err != nil {
		return nil, err
	}
	gamma, err := deriveChallenge(fs, "gamma", []dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
	if err != nil {
		return nil, err
	}
	eta, err := deriveChallenge(fs, "eta", nil)
	if err != nil {
		return nil, err
	}

	// ── 第二轮：置换累加器 Z ──────────────────────────────────────────────────
	zLagrange := make([][]fr.Element, M)
	comZ := make([]dkzg.Digest, M)

	errCh2 := make(chan error, M)
	var wg2 sync.WaitGroup
	for i := 0; i < M; i++ {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			z, err := computeZLagrange(witnesses[i].L, witnesses[i].R, witnesses[i].O, pk, eta, gamma)
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

	lambda, err := deriveChallenge(fs, "lambda", []dkzg.Digest{proof.Z})
	if err != nil {
		return nil, err
	}

	// ── 第三轮：X 轴商多项式 ──────────────────────────────────────────────────
	// 将共享选择子和置换多项式一次性转换为规范系数形式。
	qlCan := lagrangeToCanonical(pk.Ql, &pk.DomainX)
	qrCan := lagrangeToCanonical(pk.Qr, &pk.DomainX)
	qmCan := lagrangeToCanonical(pk.Qm, &pk.DomainX)
	qoCan := lagrangeToCanonical(pk.Qo, &pk.DomainX)
	qkCan := lagrangeToCanonical(pk.Qk, &pk.DomainX)
	s1Can := lagrangeToCanonical(pk.S1, &pk.DomainX)
	s2Can := lagrangeToCanonical(pk.S2, &pk.DomainX)
	s3Can := lagrangeToCanonical(pk.S3, &pk.DomainX)

	// 共享多项式的陪集求值（只需计算一次）。
	qlCoset := cosetEval(qlCan, &pk.DomainXL)
	qrCoset := cosetEval(qrCan, &pk.DomainXL)
	qmCoset := cosetEval(qmCan, &pk.DomainXL)
	qoCoset := cosetEval(qoCan, &pk.DomainXL)
	qkCoset := cosetEval(qkCan, &pk.DomainXL)
	s1Coset := cosetEval(s1Can, &pk.DomainXL)
	s2Coset := cosetEval(s2Can, &pk.DomainXL)
	s3Coset := cosetEval(s3Can, &pk.DomainXL)

	// 预计算陪集上的 L_0(X) 和 X^T-1（所有子节点共享）。
	// vanishingOnCoset 返回自然序值；须调用 BitReverse 使其与 DIF 的比特逆序输出对齐。
	l0Coset := l0OnCoset(&pk.DomainXL, T)
	vanXCoset := vanishingOnCoset(&pk.DomainXL, uint64(T))
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
			// 规范系数形式。
			aCan := lagrangeToCanonical(witnesses[i].L, &pk.DomainX)
			bCan := lagrangeToCanonical(witnesses[i].R, &pk.DomainX)
			oCan := lagrangeToCanonical(witnesses[i].O, &pk.DomainX)
			zCan := lagrangeToCanonical(zLagrange[i], &pk.DomainX)
			// Z(ω·X)：将第 k 个系数乘以 ω^k。
			zShiftCan := shiftCanonical(zCan, pk.DomainX.Generator)

			// 各子节点私有的陪集求值。
			aCoset := cosetEval(aCan, &pk.DomainXL)
			bCoset := cosetEval(bCan, &pk.DomainXL)
			oCoset := cosetEval(oCan, &pk.DomainXL)
			zCoset := cosetEval(zCan, &pk.DomainXL)
			zsCoset := cosetEval(zShiftCan, &pk.DomainXL)

			// 大域陪集上的恒等置换求值。
			// 在陪集点 c_j = u·ω_{4T}^j 处：
			//   id_a(c_j) = c_j
			//   id_b(c_j) = u_small · c_j  （u_small = DomainX.FrMultiplicativeGen）
			//   id_c(c_j) = u_small² · c_j
			idCoset := cosetPoints(&pk.DomainXL)
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

				// 门约束：ql*a + qr*b + qm*a*b + qo*o + qk
				var gate fr.Element
				{
					var t0, t1, t2, t3 fr.Element
					t0.Mul(&ql, &a)
					t1.Mul(&qr, &b)
					t2.Mul(&qm, &a)
					t2.Mul(&t2, &b)
					t3.Mul(&qo, &o)
					gate.Add(&t0, &t1)
					gate.Add(&gate, &t2)
					gate.Add(&gate, &t3)
					gate.Add(&gate, &qk)
				}

				// 边界约束：L_0(X)·(Z-1)。
				var boundary fr.Element
				{
					var t fr.Element
					t.Sub(&z, &one)
					boundary.Mul(&l0, &t)
				}

				// 置换约束：
				//   F = (a+η·id_a+γ)(b+η·id_b+γ)(o+η·id_c+γ) · Z
				//   G = (a+η·s1+γ)(b+η·s2+γ)(o+η·s3+γ) · Zs
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

				// 合并：gate + lambda*perm + lambda^2*boundary
				var num fr.Element
				var tmp fr.Element
				tmp.Mul(&lambda, &boundary).Add(&tmp, &perm)
				num.Mul(&lambda, &tmp).Add(&num, &gate)

				h[j].Mul(&num, &vanXCosetInv[j])
			}

			// IFFT 变换回规范系数形式。
			pk.DomainXL.FFTInverse(h, fft.DIT, fft.OnCoset())

			// 拆分为各长度 T 的 hx1、hx2、hx3。
			hx1 := make([]fr.Element, T)
			hx2 := make([]fr.Element, T)
			hx3 := make([]fr.Element, T)
			copy(hx1, h[0:T])
			copy(hx2, h[T:2*T])
			copy(hx3, h[2*T:3*T])
			qResults[i] = quotientResult{hx1, hx2, hx3}

			// 承诺（CommitLocal 需要 Lagrange 形式）。
			hx1Lag := canonicalToLagrange(hx1, &pk.DomainX)
			hx2Lag := canonicalToLagrange(hx2, &pk.DomainX)
			hx3Lag := canonicalToLagrange(hx3, &pk.DomainX)

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

	alpha, err := deriveChallenge(fs, "alpha", []dkzg.Digest{proof.Hx[0], proof.Hx[1], proof.Hx[2]})
	if err != nil {
		return nil, err
	}

	// ── 第四轮：在 α 处的 X 轴开放 ──────────────────────────────────────────
	alphaShifted := new(fr.Element)
	alphaShifted.Mul(&alpha, &pk.DomainX.Generator)

	// 计算共享多项式在 α 处的求值。
	qlAlpha := evalPolyLagrange(pk.Ql, alpha, pk.DomainX.Generator)
	qrAlpha := evalPolyLagrange(pk.Qr, alpha, pk.DomainX.Generator)
	qmAlpha := evalPolyLagrange(pk.Qm, alpha, pk.DomainX.Generator)
	qoAlpha := evalPolyLagrange(pk.Qo, alpha, pk.DomainX.Generator)
	qkAlpha := evalPolyLagrange(pk.Qk, alpha, pk.DomainX.Generator)
	s1Alpha := evalPolyLagrange(pk.S1, alpha, pk.DomainX.Generator)
	s2Alpha := evalPolyLagrange(pk.S2, alpha, pk.DomainX.Generator)
	s3Alpha := evalPolyLagrange(pk.S3, alpha, pk.DomainX.Generator)

	// 每个子节点：在 α 和 ω·α 处求值 A、B、O、Z、ZS、foldedHx。
	type evalResult struct {
		aAlpha, bAlpha, oAlpha fr.Element
		zAlpha, zsAlpha        fr.Element
		fhxAlpha               fr.Element
		// X 轴开放证明（各多项式在 α 处的 H 值）。
		piXA, piXB, piXO, piXZ fr.Element // A、B、O、Z 在 α 处的 π_0
		piXZS                  fr.Element // Z 在 ω·α 处
		piXFhx                 fr.Element // foldedHx 在 α 处
		// 聚合后的 piX 证明（G1 点）。
		proofXA, proofXB, proofXO, proofXZ bn254.G1Affine
		proofXZS, proofXFhx                bn254.G1Affine
		// 共享多项式（仅通过子节点 0 风格的聚合计算一次）。
		proofXQl, proofXQr, proofXQm, proofXQo, proofXQk bn254.G1Affine
		proofXS1, proofXS2, proofXS3                      bn254.G1Affine
	}

	eResults := make([]evalResult, M)
	errCh4 := make(chan error, M)
	var wg4 sync.WaitGroup

	alphaPowT := new(fr.Element)
	{
		bT := new(big.Int).SetInt64(int64(T))
		alphaPowT.Exp(alpha, bT)
	}

	for i := 0; i < M; i++ {
		wg4.Add(1)
		go func(i int) {
			defer wg4.Done()
			er := &eResults[i]

			// 使用 Lagrange 插值在 α 处求值各多项式。
			er.aAlpha = evalPolyLagrange(witnesses[i].L, alpha, pk.DomainX.Generator)
			er.bAlpha = evalPolyLagrange(witnesses[i].R, alpha, pk.DomainX.Generator)
			er.oAlpha = evalPolyLagrange(witnesses[i].O, alpha, pk.DomainX.Generator)
			er.zAlpha = evalPolyLagrange(zLagrange[i], alpha, pk.DomainX.Generator)
			er.zsAlpha = evalPolyLagrange(zLagrange[i], *alphaShifted, pk.DomainX.Generator)

			// 折叠 H_X：fhx = hx1 + alphaPowT·hx2 + alphaPowT²·hx3（规范形式）。
			fhxCan := foldQuotient(qResults[i].hx1, qResults[i].hx2, qResults[i].hx3, *alphaPowT)
			// 从规范形式在 α 处求值。
			er.fhxAlpha = evalPolyCanonical(fhxCan, alpha)

			// 通过 LocalOpenX 生成 X 轴开放证明（需要 Lagrange 形式输入）。
			fhxLag := canonicalToLagrange(fhxCan, &pk.DomainX)

			proofA, err := dkzg.LocalOpenX(uint64(i), witnesses[i].L, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofB, err := dkzg.LocalOpenX(uint64(i), witnesses[i].R, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofO, err := dkzg.LocalOpenX(uint64(i), witnesses[i].O, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofZ, err := dkzg.LocalOpenX(uint64(i), zLagrange[i], alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofZS, err := dkzg.LocalOpenX(uint64(i), zLagrange[i], *alphaShifted, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofFhx, err := dkzg.LocalOpenX(uint64(i), fhxLag, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}

			// 共享多项式开放（各子节点使用相同的多项式）。
			proofQl, err := dkzg.LocalOpenX(uint64(i), pk.Ql, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofQr, err := dkzg.LocalOpenX(uint64(i), pk.Qr, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofQm, err := dkzg.LocalOpenX(uint64(i), pk.Qm, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofQo, err := dkzg.LocalOpenX(uint64(i), pk.Qo, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofQk, err := dkzg.LocalOpenX(uint64(i), pk.Qk, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofS1, err := dkzg.LocalOpenX(uint64(i), pk.S1, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofS2, err := dkzg.LocalOpenX(uint64(i), pk.S2, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}
			proofS3, err := dkzg.LocalOpenX(uint64(i), pk.S3, alpha, pk.DKZGSRS)
			if err != nil {
				errCh4 <- err
				return
			}

			er.proofXA = proofA.H
			er.proofXB = proofB.H
			er.proofXO = proofO.H
			er.proofXZ = proofZ.H
			er.proofXZS = proofZS.H
			er.proofXFhx = proofFhx.H
			er.proofXQl = proofQl.H
			er.proofXQr = proofQr.H
			er.proofXQm = proofQm.H
			er.proofXQo = proofQo.H
			er.proofXQk = proofQk.H
			er.proofXS1 = proofS1.H
			er.proofXS2 = proofS2.H
			er.proofXS3 = proofS3.H
		}(i)
	}
	wg4.Wait()
	close(errCh4)
	if err := <-errCh4; err != nil {
		return nil, err
	}

	// 收集各多项式的本地证明数组和 Y 轴求值向量。
	piXFhx := make([]dkzg.OpeningProofX, M)
	piXA := make([]dkzg.OpeningProofX, M)
	piXB := make([]dkzg.OpeningProofX, M)
	piXO := make([]dkzg.OpeningProofX, M)
	piXZ := make([]dkzg.OpeningProofX, M)
	piXZS := make([]dkzg.OpeningProofX, M)
	piXQl := make([]dkzg.OpeningProofX, M)
	piXQr := make([]dkzg.OpeningProofX, M)
	piXQm := make([]dkzg.OpeningProofX, M)
	piXQo := make([]dkzg.OpeningProofX, M)
	piXQk := make([]dkzg.OpeningProofX, M)
	piXS1 := make([]dkzg.OpeningProofX, M)
	piXS2 := make([]dkzg.OpeningProofX, M)
	piXS3 := make([]dkzg.OpeningProofX, M)

	aVec := make([]fr.Element, M)
	bVec := make([]fr.Element, M)
	oVec := make([]fr.Element, M)
	zVec := make([]fr.Element, M)
	zsVec := make([]fr.Element, M)
	fhxVec := make([]fr.Element, M)

	for i := 0; i < M; i++ {
		er := &eResults[i]
		piXFhx[i] = dkzg.OpeningProofX{H: er.proofXFhx, ClaimedValue: er.fhxAlpha}
		piXA[i] = dkzg.OpeningProofX{H: er.proofXA, ClaimedValue: er.aAlpha}
		piXB[i] = dkzg.OpeningProofX{H: er.proofXB, ClaimedValue: er.bAlpha}
		piXO[i] = dkzg.OpeningProofX{H: er.proofXO, ClaimedValue: er.oAlpha}
		piXZ[i] = dkzg.OpeningProofX{H: er.proofXZ, ClaimedValue: er.zAlpha}
		piXZS[i] = dkzg.OpeningProofX{H: er.proofXZS, ClaimedValue: er.zsAlpha}
		piXQl[i] = dkzg.OpeningProofX{H: er.proofXQl, ClaimedValue: qlAlpha}
		piXQr[i] = dkzg.OpeningProofX{H: er.proofXQr, ClaimedValue: qrAlpha}
		piXQm[i] = dkzg.OpeningProofX{H: er.proofXQm, ClaimedValue: qmAlpha}
		piXQo[i] = dkzg.OpeningProofX{H: er.proofXQo, ClaimedValue: qoAlpha}
		piXQk[i] = dkzg.OpeningProofX{H: er.proofXQk, ClaimedValue: qkAlpha}
		piXS1[i] = dkzg.OpeningProofX{H: er.proofXS1, ClaimedValue: s1Alpha}
		piXS2[i] = dkzg.OpeningProofX{H: er.proofXS2, ClaimedValue: s2Alpha}
		piXS3[i] = dkzg.OpeningProofX{H: er.proofXS3, ClaimedValue: s3Alpha}

		aVec[i] = er.aAlpha
		bVec[i] = er.bAlpha
		oVec[i] = er.oAlpha
		zVec[i] = er.zAlpha
		zsVec[i] = er.zsAlpha
		fhxVec[i] = er.fhxAlpha
	}

	// foldedHx 承诺：comFhx = Hx[0] + α^T·Hx[1] + α^{2T}·Hx[2]。
	var alphaPowTBig big.Int
	alphaPowT.BigInt(&alphaPowTBig)
	foldedHxDig := foldDigests(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)

	// 将每个多项式的 M 个本地证明聚合为 AggregatedProofX。
	aggregateX := func(localProofs []dkzg.OpeningProofX) (dkzg.AggregatedProofX, error) {
		return dkzg.AggregateProofX(localProofs, pk.DKZGSRS)
	}

	aggFhx, err := aggregateX(piXFhx)
	if err != nil {
		return nil, err
	}
	aggA, err := aggregateX(piXA)
	if err != nil {
		return nil, err
	}
	aggB, err := aggregateX(piXB)
	if err != nil {
		return nil, err
	}
	aggO, err := aggregateX(piXO)
	if err != nil {
		return nil, err
	}
	aggZ, err := aggregateX(piXZ)
	if err != nil {
		return nil, err
	}
	aggZS, err := aggregateX(piXZS)
	if err != nil {
		return nil, err
	}
	aggQl, err := aggregateX(piXQl)
	if err != nil {
		return nil, err
	}
	aggQr, err := aggregateX(piXQr)
	if err != nil {
		return nil, err
	}
	aggQm, err := aggregateX(piXQm)
	if err != nil {
		return nil, err
	}
	aggQo, err := aggregateX(piXQo)
	if err != nil {
		return nil, err
	}
	aggQk, err := aggregateX(piXQk)
	if err != nil {
		return nil, err
	}
	aggS1, err := aggregateX(piXS1)
	if err != nil {
		return nil, err
	}
	aggS2, err := aggregateX(piXS2)
	if err != nil {
		return nil, err
	}
	aggS3, err := aggregateX(piXS3)
	if err != nil {
		return nil, err
	}

	// ZShiftedProofX：Z 在 ω·α 处的独立聚合证明。
	proof.ZShiftedProofX = aggZS

	// BatchOpenX：将 13 个多项式证明折叠为一个 BatchedProofX。
	// 顺序：[foldedHx, A, B, O, Ql, Qr, Qm, Qo, Qk, S1, S2, S3, Z]
	batchAggProofs := []dkzg.AggregatedProofX{
		aggFhx, aggA, aggB, aggO,
		aggQl, aggQr, aggQm, aggQo, aggQk,
		aggS1, aggS2, aggS3,
		aggZ,
	}
	batchComFs := []dkzg.Digest{
		foldedHxDig, proof.LRO[0], proof.LRO[1], proof.LRO[2],
		pk.Vk.Ql, pk.Vk.Qr, pk.Vk.Qm, pk.Vk.Qo, pk.Vk.Qk,
		pk.Vk.S1, pk.Vk.S2, pk.Vk.S3,
		proof.Z,
	}
	proof.BatchedProofX, err = dkzg.BatchOpenX(batchAggProofs, batchComFs, alpha)
	if err != nil {
		return nil, err
	}

	proof.ClaimedA = aVec[0]
	proof.ClaimedB = bVec[0]
	proof.ClaimedO = oVec[0]
	proof.ClaimedZ = zVec[0]
	proof.ClaimedZS = zsVec[0]
	proof.ClaimedHx = fhxVec[0]

	// 检查 X 轴约束（调试用）。
	if err := checkConstraintX(alpha, qlAlpha, qrAlpha, qmAlpha, qoAlpha, qkAlpha,
		s1Alpha, s2Alpha, s3Alpha, pk.DomainX.FrMultiplicativeGen,
		aVec, bVec, oVec, zVec, zsVec, fhxVec,
		eta, gamma, lambda, pk); err != nil {
		return nil, err
	}

	// ── 第五轮：Y 轴商多项式 ──────────────────────────────────────────────────
	// 从每个子节点的求值向量构建 Y 轴规范系数多项式。
	aCanY := lagrangeToCanonical(aVec, &pk.DomainY)
	bCanY := lagrangeToCanonical(bVec, &pk.DomainY)
	oCanY := lagrangeToCanonical(oVec, &pk.DomainY)
	zCanY := lagrangeToCanonical(zVec, &pk.DomainY)
	zsCanY := lagrangeToCanonical(zsVec, &pk.DomainY)
	fhxCanY := lagrangeToCanonical(fhxVec, &pk.DomainY)

	// 在 4M 域上进行陪集求值。
	aCosetY := cosetEval(aCanY, &pk.DomainYL)
	bCosetY := cosetEval(bCanY, &pk.DomainYL)
	oCosetY := cosetEval(oCanY, &pk.DomainYL)
	zCosetY := cosetEval(zCanY, &pk.DomainYL)
	zsCosetY := cosetEval(zsCanY, &pk.DomainYL)
	fhxCosetY := cosetEval(fhxCanY, &pk.DomainYL)

	// Y 轴标量常数（共享选择子在 α 处的求值）。
	vanishingX := new(fr.Element)
	{
		bT := new(big.Int).SetInt64(int64(T))
		vanishingX.Exp(alpha, bT)
		var one fr.Element
		one.SetOne()
		vanishingX.Sub(vanishingX, &one)
	}

	// L_0(α) = (α^T-1) / (T·(α-1))。
	lagrange0Alpha := computeLagrange0(alpha, pk.DomainX.CardinalityInv)

	// 三个陪集上的恒等置换在 α 处的求值。
	// id_a(α) = α，id_b(α) = u·α，id_c(α) = u²·α。
	u := pk.DomainX.FrMultiplicativeGen
	var uSq, idA, idB, idC fr.Element
	uSq.Square(&u)
	idA.Set(&alpha)
	idB.Mul(&u, &alpha)
	idC.Mul(&uSq, &alpha)

	// 标量乘子（对 Y 域中所有 j 相同）。
	etaIdA := new(fr.Element)
	etaIdA.Mul(&eta, &idA)
	etaIdB := new(fr.Element)
	etaIdB.Mul(&eta, &idB)
	etaIdC := new(fr.Element)
	etaIdC.Mul(&eta, &idC)
	etaS1 := new(fr.Element)
	etaS1.Mul(&eta, &s1Alpha)
	etaS2 := new(fr.Element)
	etaS2.Mul(&eta, &s2Alpha)
	etaS3 := new(fr.Element)
	etaS3.Mul(&eta, &s3Alpha)

	N4M := int(pk.DomainYL.Cardinality)
	hY := make([]fr.Element, N4M)
	vanYCoset := vanishingOnCoset(&pk.DomainYL, uint64(M))
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

		// 门约束：ql(α)·A + qr(α)·B + qm(α)·A·B + qo(α)·O + qk(α)
		var gate fr.Element
		{
			var t0, t1, t2, t3 fr.Element
			t0.Mul(&qlAlpha, &a)
			t1.Mul(&qrAlpha, &b)
			t2.Mul(&qmAlpha, &a)
			t2.Mul(&t2, &b)
			t3.Mul(&qoAlpha, &o)
			gate.Add(&t0, &t1).Add(&gate, &t2).Add(&gate, &t3).Add(&gate, &qkAlpha)
		}

		// 边界约束：L_0(α)·(Z(Y,α)-1)
		var boundary fr.Element
		{
			var t fr.Element
			t.Sub(&z, &one)
			boundary.Mul(&lagrange0Alpha, &t)
		}

		// 置换约束：
		//   G = (A+η·s1+γ)(B+η·s2+γ)(O+η·s3+γ) · Zs
		//   F = (A+η·α+γ)(B+η·u·α+γ)(O+η·u²·α+γ) · Z
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

		// 合并：gate + λ·boundary + λ²·perm - (α^T-1)·Hx(Y,α)。
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

	// 拆分为各长度 M 的 hy1、hy2、hy3。
	hy1 := make([]fr.Element, M)
	hy2 := make([]fr.Element, M)
	hy3 := make([]fr.Element, M)
	copy(hy1, hY[0:M])
	copy(hy2, hY[M:2*M])
	copy(hy3, hY[2*M:3*M])

	// 使用 Y 轴 Lagrange SRS（Vy）对 HY 各部分进行承诺。
	hy1Lag := canonicalToLagrange(hy1, &pk.DomainY)
	hy2Lag := canonicalToLagrange(hy2, &pk.DomainY)
	hy3Lag := canonicalToLagrange(hy3, &pk.DomainY)

	proof.Hy[0], err = commitY(hy1Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	proof.Hy[1], err = commitY(hy2Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}
	proof.Hy[2], err = commitY(hy3Lag, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// 将 BatchedProofX.H + ClaimedDigests[0..12] + Hy[0..2] 绑定到转录中以派生 beta。
	// （ZShiftedProofX 不参与绑定，与参考协议一致。）
	betaDigests := make([]dkzg.Digest, 0, 1+13+3)
	betaDigests = append(betaDigests, proof.BatchedProofX.H)
	for k := range proof.BatchedProofX.ClaimedDigests {
		betaDigests = append(betaDigests, proof.BatchedProofX.ClaimedDigests[k])
	}
	betaDigests = append(betaDigests, proof.Hy[0], proof.Hy[1], proof.Hy[2])
	beta, err := deriveChallenge(fs, "beta", betaDigests)
	if err != nil {
		return nil, err
	}

	// ── 第六轮：在 β 处的 Y 轴批量开放 ─────────────────────────────────────
	// 折叠 HY：foldedHy = hy1 + β^M·hy2 + β^{2M}·hy3（Lagrange 形式）。
	betaPowM := new(fr.Element)
	{
		bM := new(big.Int).SetInt64(int64(M))
		betaPowM.Exp(beta, bM)
	}
	foldedHyLag := foldQuotientLagrange(hy1Lag, hy2Lag, hy3Lag, *betaPowM)
	foldedHyDig := foldDigests(proof.Hy[0], proof.Hy[1], proof.Hy[2], betaPowM)

	// 为共享选择子/置换多项式构建常数 Y 轴向量（各子节点取值相同）。
	makeConst := func(val fr.Element) []fr.Element {
		v := make([]fr.Element, M)
		for i := range v {
			v[i] = val
		}
		return v
	}

	// 15 个 Y 轴多项式的 Lagrange 向量（各长度 M）。
	// 顺序：[foldedHx, A, B, O, Ql, Qr, Qm, Qo, Qk, S1, S2, S3, Z, ZS, foldedHy]
	yPolys := [][]fr.Element{
		fhxVec,
		aVec, bVec, oVec,
		makeConst(qlAlpha), makeConst(qrAlpha), makeConst(qmAlpha),
		makeConst(qoAlpha), makeConst(qkAlpha),
		makeConst(s1Alpha), makeConst(s2Alpha), makeConst(s3Alpha),
		zVec, zsVec,
		foldedHyLag,
	}

	// 15 个 Y 轴多项式的承诺（comVF_k）。
	// k=0..12：来自 BatchedProofX.ClaimedDigests
	// k=13：ZShiftedProofX.ComVF
	// k=14：foldedHyDig
	yComVFs := make([]dkzg.Digest, 15)
	copy(yComVFs[:13], proof.BatchedProofX.ClaimedDigests)
	yComVFs[13] = proof.ZShiftedProofX.ComVF
	yComVFs[14] = foldedHyDig

	proof.BatchedProofY, err = dkzg.BatchOpenY(yComVFs, yPolys, beta, pk.DKZGSRS)
	if err != nil {
		return nil, err
	}

	// 从 BatchedProofY 中提取声明求值值。
	// 顺序：[foldedHx=0, A=1, B=2, O=3, Ql=4, Qr=5, Qm=6, Qo=7, Qk=8, S1=9, S2=10, S3=11, Z=12, ZS=13, foldedHy=14]
	proof.ClaimedHx = proof.BatchedProofY.ClaimedValues[0]
	proof.ClaimedA = proof.BatchedProofY.ClaimedValues[1]
	proof.ClaimedB = proof.BatchedProofY.ClaimedValues[2]
	proof.ClaimedO = proof.BatchedProofY.ClaimedValues[3]
	proof.ClaimedQl = proof.BatchedProofY.ClaimedValues[4]
	proof.ClaimedQr = proof.BatchedProofY.ClaimedValues[5]
	proof.ClaimedQm = proof.BatchedProofY.ClaimedValues[6]
	proof.ClaimedQo = proof.BatchedProofY.ClaimedValues[7]
	proof.ClaimedQk = proof.BatchedProofY.ClaimedValues[8]
	proof.ClaimedS1 = proof.BatchedProofY.ClaimedValues[9]
	proof.ClaimedS2 = proof.BatchedProofY.ClaimedValues[10]
	proof.ClaimedS3 = proof.BatchedProofY.ClaimedValues[11]
	proof.ClaimedZ = proof.BatchedProofY.ClaimedValues[12]
	proof.ClaimedZS = proof.BatchedProofY.ClaimedValues[13]
	proof.ClaimedHy = proof.BatchedProofY.ClaimedValues[14]

	// 检查 Y 轴约束（调试用）。
	if err := checkConstraintY(alpha, beta, proof, eta, gamma, lambda, pk); err != nil {
		return nil, err
	}

	_ = alphaPowTBig
	return proof, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ────────────────────────────────────────────────────────────────────────────

// cosetEval 在 domainL 的陪集上对规范系数多项式进行求值。
// 输入规范多项式长度须 ≤ domainL.Cardinality；不足部分以零填充。
// 返回 DIF 比特逆序的求值结果。
func cosetEval(canonical []fr.Element, domainL *fft.Domain) []fr.Element {
	out := padTo(canonical, int(domainL.Cardinality))
	domainL.FFT(out, fft.DIF, fft.OnCoset())
	return out
}

// cosetPoints 返回陪集点 u·ω_{4T}^j（j=0..4T-1，与 cosetEval 的 DIF 比特逆序对齐）。
// 以 DIF 比特逆序返回，与 cosetEval 输出索引一致。
func cosetPoints(domainL *fft.Domain) []fr.Element {
	N := int(domainL.Cardinality)
	pts := make([]fr.Element, N)
	u := domainL.FrMultiplicativeGen
	gen := domainL.Generator
	pts[0].Set(&u)
	for j := 1; j < N; j++ {
		pts[j].Mul(&pts[j-1], &gen)
	}
	// 应用 DIF 比特逆序。
	fft.BitReverse(pts)
	return pts
}

// l0OnCoset 在 domainL 的陪集上计算 L_0(X) = (X^T-1)/(T·(X-1))。
// 返回值与 cosetEval 输出顺序相同（DIF 比特逆序）。
func l0OnCoset(domainL *fft.Domain, T int) []fr.Element {
	N := int(domainL.Cardinality)
	// 以自然序获取陪集点（尚未比特逆序）。
	u := domainL.FrMultiplicativeGen
	gen := domainL.Generator

	pts := make([]fr.Element, N)
	pts[0].Set(&u)
	for j := 1; j < N; j++ {
		pts[j].Mul(&pts[j-1], &gen)
	}

	var one fr.Element
	one.SetOne()
	TInv := new(fr.Element)
	TInv.SetUint64(uint64(T)).Inverse(TInv)
	sizeExp := new(big.Int).SetInt64(int64(T))

	res := make([]fr.Element, N)
	dens := make([]fr.Element, N)
	for j := 0; j < N; j++ {
		var xT fr.Element
		xT.Exp(pts[j], sizeExp)
		xT.Sub(&xT, &one)
		res[j].Set(&xT) // 分子：X^T - 1

		var d fr.Element
		d.Sub(&pts[j], &one)
		dens[j].Set(&d) // 分母：X - 1
	}
	dens = fr.BatchInvert(dens)
	for j := 0; j < N; j++ {
		res[j].Mul(&res[j], &dens[j]).Mul(&res[j], TInv)
	}

	// 应用 DIF 比特逆序，使结果与 cosetEval 输出对齐。
	fft.BitReverse(res)
	return res
}

// foldDigests 计算 com0 + aPowN·com1 + aPowN²·com2（G1 标量乘法）。
func foldDigests(com0, com1, com2 dkzg.Digest, aPowN *fr.Element) dkzg.Digest {
	var aBig big.Int
	aPowN.BigInt(&aBig)

	var d dkzg.Digest
	d = com2
	d.ScalarMultiplication(&d, &aBig)
	d.Add(&d, &com1)
	d.ScalarMultiplication(&d, &aBig)
	d.Add(&d, &com0)
	return d
}

// foldQuotientLagrange 逐点折叠：hy1Lag + betaPowM·hy2Lag + betaPowM²·hy3Lag。
func foldQuotientLagrange(hy1, hy2, hy3 []fr.Element, betaPowM fr.Element) []fr.Element {
	M := len(hy1)
	out := make([]fr.Element, M)
	for j := 0; j < M; j++ {
		out[j].Mul(&hy3[j], &betaPowM)
		out[j].Add(&out[j], &hy2[j])
		out[j].Mul(&out[j], &betaPowM)
		out[j].Add(&out[j], &hy1[j])
	}
	return out
}

// computeLagrange0 计算 L_0(α) = (α^T-1)/(T·(α-1))。
func computeLagrange0(alpha fr.Element, cardinalityInv fr.Element) fr.Element {
	var one, numer, denom, result fr.Element
	one.SetOne()

	numer.Exp(alpha, new(big.Int).SetUint64(0)) // 下方覆盖
	// 调用方传入 cardinalityInv = 1/T。
	// 由 cardinalityInv = 1/T 可得 T = 1/cardinalityInv。
	// L_0(α) = (α^T-1) / (T(α-1))
	// 需要 T：cardinalityInv = 1/T → T = 1/cardinalityInv。
	var T fr.Element
	T.Inverse(&cardinalityInv) // T
	var TBig big.Int
	T.BigInt(&TBig)

	numer.Exp(alpha, &TBig)
	numer.Sub(&numer, &one)

	denom.Sub(&alpha, &one)
	denom.Inverse(&denom)

	result.Mul(&numer, &denom)
	result.Mul(&result, &cardinalityInv) // × (1/T)
	return result
}

// bindPublicData 将公开数据写入 Fiat-Shamir 转录。
func bindPublicData(fs *fiatshamir.Transcript, challenge string, vk *VerifyingKey, publicInputs [][]fr.Element) error {
	// 绑定选择子和置换多项式的承诺。
	for _, com := range []dkzg.Digest{vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3} {
		b := com.Bytes()
		if err := fs.Bind(challenge, b[:]); err != nil {
			return err
		}
	}
	// 绑定公开输入。
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

// deriveChallenge 在绑定提供的承诺后，从 Fiat-Shamir 转录中导出挑战值。
func deriveChallenge(fs *fiatshamir.Transcript, label string, digests []dkzg.Digest) (fr.Element, error) {
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

// ────────────────────────────────────────────────────────────────────────────
// computeZLagrange 以 Lagrange 形式计算置换累加器 Z。
// ────────────────────────────────────────────────────────────────────────────

func computeZLagrange(l, r, o []fr.Element, pk *ProvingKey, eta, gamma fr.Element) ([]fr.Element, error) {
	T := int(pk.DomainX.Cardinality)
	id := getIDSmallDomain(&pk.DomainX)

	z := make([]fr.Element, T)
	gInv := make([]fr.Element, T)
	z[0].SetOne()
	gInv[0].SetOne()

	for i := 0; i < T-1; i++ {
		var f0, f1, f2 fr.Element
		f0.Mul(&eta, &id[i]).Add(&f0, &l[i]).Add(&f0, &gamma)
		f1.Mul(&eta, &id[T+i]).Add(&f1, &r[i]).Add(&f1, &gamma)
		f2.Mul(&eta, &id[2*T+i]).Add(&f2, &o[i]).Add(&f2, &gamma)
		f0.Mul(&f0, &f1).Mul(&f0, &f2)
		z[i+1] = f0

		var g0, g1, g2 fr.Element
		g0.Mul(&eta, &pk.S1[i]).Add(&g0, &l[i]).Add(&g0, &gamma)
		g1.Mul(&eta, &pk.S2[i]).Add(&g1, &r[i]).Add(&g1, &gamma)
		g2.Mul(&eta, &pk.S3[i]).Add(&g2, &o[i]).Add(&g2, &gamma)
		g0.Mul(&g0, &g1).Mul(&g0, &g2)
		gInv[i+1] = g0
	}

	gInv = fr.BatchInvert(gInv)
	for i := 1; i < T; i++ {
		z[i].Mul(&z[i], &z[i-1]).Mul(&z[i], &gInv[i])
	}

	// 验证 Z 的第 T 次累积值闭合为 1。
	// z[T-1] 应满足 z[T-1]*f[T-1]/g[T-1] = z[0] = 1
	// （由约束系统自身保证，此处仅验证 z[0]=1）。
	// 注意：z[T] 等于 z[T-1]*f[T-1]/g[T-1]，应为 1；
	// 不存储 z[T]，由约束 L_0(X)(Z(X)-1) 保证。
	return z, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 代数约束验证（调试用）
// ────────────────────────────────────────────────────────────────────────────

// checkConstraintX 验证每个子节点 k 的 X 轴约束方程：
//
//	gate_k(α) + λ·L0(α)·(Z_k(α)-1) + λ²·(Zs_k·G_k - Z_k·F_k) = (α^T-1)·Hx_k(α)
func checkConstraintX(
	alpha, ql, qr, qm, qo, qk, s1, s2, s3, u fr.Element,
	aVec, bVec, oVec, zVec, zsVec, fhxVec []fr.Element,
	eta, gamma, lambda fr.Element, pk *ProvingKey,
) error {
	T := int(pk.DomainX.Cardinality)
	M := len(aVec)

	l0 := computeLagrange0(alpha, pk.DomainX.CardinalityInv)
	var vanishX fr.Element
	vanishX.Exp(alpha, new(big.Int).SetInt64(int64(T)))
	var one fr.Element
	one.SetOne()
	vanishX.Sub(&vanishX, &one)

	var uSq fr.Element
	uSq.Square(&u)

	for k := 0; k < M; k++ {
		a, b, o := aVec[k], bVec[k], oVec[k]
		z, zs := zVec[k], zsVec[k]
		hx := fhxVec[k]

		// 门约束。
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

		// 边界约束。
		var boundary fr.Element
		{
			var t fr.Element
			t.Sub(&z, &one)
			boundary.Mul(&l0, &t)
		}

		// 置换约束。
		var idA, idB, idC fr.Element
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

		// 左端（LHS）。
		var lhs fr.Element
		{
			var tmp fr.Element
			tmp.Mul(&lambda, &boundary).Add(&tmp, &perm)
			lhs.Mul(&lambda, &tmp).Add(&lhs, &gate)
		}

		// 右端（RHS）。
		var rhs fr.Element
		rhs.Mul(&vanishX, &hx)

		if !lhs.Equal(&rhs) {
			return fmt.Errorf("piano: 子节点 %d 的 X 轴约束不满足：lhs=%s rhs=%s",
				k, lhs.String(), rhs.String())
		}
	}
	return nil
}

// checkConstraintY 在 (β, α) 处验证 Y 轴代数约束：
//
//	G(β,α) + λ·P0(β,α) + λ²·P1(β,α) = (α^T-1)·Hx(β,α) + (β^M-1)·Hy(β)
func checkConstraintY(alpha, beta fr.Element, proof *Proof, eta, gamma, lambda fr.Element, pk *ProvingKey) error {
	T := int(pk.DomainX.Cardinality)
	M := int(pk.DomainY.Cardinality)

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

	// 门约束。
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

	// 边界约束。
	l0 := computeLagrange0(alpha, pk.DomainX.CardinalityInv)
	var boundary fr.Element
	{
		var t fr.Element
		t.Sub(&z, &one)
		boundary.Mul(&l0, &t)
	}

	// 置换约束。
	u := pk.DomainX.FrMultiplicativeGen
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

	// 左端（LHS）= gate + λ·boundary + λ²·perm。
	var lhs fr.Element
	{
		var tmp fr.Element
		tmp.Mul(&lambda, &boundary).Add(&tmp, &perm)
		lhs.Mul(&lambda, &tmp).Add(&lhs, &gate)
	}

	// 右端（RHS）= (α^T-1)·Hx + (β^M-1)·Hy。
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
		return fmt.Errorf("piano: Y 轴约束不满足：lhs=%s rhs=%s", lhs.String(), rhs.String())
	}
	return nil
}
