package bpiano_test

import (
	"crypto/sha256"
	"math/big"
	"testing"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/piano"
)

// buildDiffWitnessCircuit：T=4，M=2，平凡约束，但各子节点的 L[0] 不同。
// 确保 Y 轴多项式非常数（j=0 处 aVec[0]=1，aVec[1]=2）。
func buildDiffWitnessCircuit(T, M int) (piano.CircuitInfo, []piano.WitnessInstance) {
	zero := make([]fr.Element, T)
	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	perm := piano.BuildPermutation(lro, 3*T, T)
	ci := piano.CircuitInfo{
		Ql: cloneSlice(zero), Qr: cloneSlice(zero),
		Qm: cloneSlice(zero), Qo: cloneSlice(zero),
		Qk: cloneSlice(zero), Permutation: perm,
	}
	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		L := make([]fr.Element, T)
		R := make([]fr.Element, T)
		O := make([]fr.Element, T)
		L[0].SetInt64(int64(i + 1)) // 子节点 0：L[0]=1，子节点 1：L[0]=2
		witnesses[i] = piano.WitnessInstance{L: L, R: R, O: O}
	}
	return ci, witnesses
}

// replayFS 重放 verify.go 中的 Fiat-Shamir 推导，以获得 α、ν、β、μ、ρ。
func replayFS(t *testing.T, proof *bpiano.CompressedProof, vk *piano.VerifyingKey) (
	alpha, nu, beta, mu, rho fr.Element,
) {
	t.Helper()
	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc, "gamma", "eta", "lambda", "alpha", "nu", "beta", "mu")

	// 绑定公开数据（此处无公开输入）
	for _, com := range []dkzg.Digest{vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3} {
		b := com.Bytes()
		if err := fs.Bind("gamma", b[:]); err != nil {
			t.Fatal(err)
		}
	}
	bind := func(label string, digests []dkzg.Digest) fr.Element {
		for _, d := range digests {
			b := d.Bytes()
			if err := fs.Bind(label, b[:]); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := fs.ComputeChallenge(label)
		if err != nil {
			t.Fatal(err)
		}
		var c fr.Element
		c.SetBytes(raw)
		return c
	}

	bind("gamma", []dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
	bind("eta", nil)
	bind("lambda", []dkzg.Digest{proof.Z})

	T := vk.SizeX
	alpha = bind("alpha", []dkzg.Digest{proof.Hx[0], proof.Hx[1], proof.Hx[2]})

	alphaPowT := new(fr.Element)
	alphaPowT.Exp(alpha, new(big.Int).SetUint64(T))
	foldedHxDig := piano.FoldDigests(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)

	nu = bind("nu", []dkzg.Digest{foldedHxDig, proof.LRO[0], proof.LRO[1], proof.LRO[2], proof.Z})
	beta = bind("beta", []dkzg.Digest{proof.ComQX, proof.ComVFAlpha, proof.ComVFZS})
	mu = bind("mu", nil)

	// ρ = Hash(ComQX, ComGY, Pi1AggH, 7 个求值)
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
	rho.SetBytes(h.Sum(nil))
	return
}

func mulG1ByFr(p bn254.G1Affine, s fr.Element) bn254.G1Affine {
	var b big.Int
	s.BigInt(&b)
	var r bn254.G1Affine
	r.ScalarMultiplication(&p, &b)
	return r
}

func mulG2ByFr(p bn254.G2Affine, s fr.Element) bn254.G2Affine {
	var b big.Int
	s.BigInt(&b)
	var r bn254.G2Affine
	r.ScalarMultiplication(&p, &b)
	return r
}

func subG2(a, b bn254.G2Affine) bn254.G2Affine {
	var r bn254.G2Affine
	r.Sub(&a, &b)
	return r
}

func addG2(a, b bn254.G2Affine) bn254.G2Affine {
	var r bn254.G2Affine
	r.Add(&a, &b)
	return r
}

// TestDebugSplitXY 分别独立执行 X 轴和 Y 轴配对验证。
// 可精确定位 4 配对方程中哪一侧出现故障。
func TestDebugSplitXY(t *testing.T) {
	T, M := 4, 2
	ci, witnesses := buildDiffWitnessCircuit(T, M)
	pk, vk := setup(t, T, M, ci)

	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}

	srs := vk.DKZGSRS
	_, _, g1Gen, _ := bn254.Generators()

	alpha, nu, beta, mu, _ := replayFS(t, proof, vk)
	alphaShifted := new(fr.Element).Mul(&alpha, &vk.GeneratorX)

	T64 := vk.SizeX
	alphaPowT := new(fr.Element).Exp(alpha, new(big.Int).SetUint64(T64))
	foldedHxDig := piano.FoldDigests(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)

	// ── Y 轴 KZG 验证 ────────────────────────────────────────────────────────
	// e(ComGY - GY(β)·g1, G2Y[0]) = e(Pi1AggH, G2Y[1] - β·G2Y[0])
	// 即 4 个配对之积 = 1：
	//   e(ComGY, G2Y[0]) · e(-GY(β)·g1, G2Y[0]) · e(-Pi1AggH, G2Y[1]-β·G2Y[0]) = 1

	// 构造 GY(β) = Σ_k μ^k * EvalK
	muPow := make([]fr.Element, 7)
	muPow[0].SetOne()
	for k := 1; k < 7; k++ {
		muPow[k].Mul(&muPow[k-1], &mu)
	}
	yEvals := []fr.Element{
		proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO,
		proof.EvalZ, proof.EvalZS, proof.EvalHy,
	}
	t.Logf("mu=%s beta=%s alpha=%s", mu.String(), beta.String(), alpha.String())
	for k, e := range yEvals {
		t.Logf("  yEvals[%d]=%s", k, e.String())
	}

	var gYBeta fr.Element
	for k, e := range yEvals {
		var term fr.Element
		term.Mul(&muPow[k], &e)
		gYBeta.Add(&gYBeta, &term)
	}
	t.Logf("GY(β)=%s", gYBeta.String())

	// [τ_Y - β]_2 = G2Y[1] - β·G2Y[0]
	tauYMinusBeta := subG2(srs.G2Y[1], mulG2ByFr(srs.G2Y[0], beta))

	// ComGY - GY(β)·g₁
	gYBetaG1 := mulG1ByFr(g1Gen, gYBeta)
	var comGYMinusZ bn254.G1Affine
	comGYMinusZ.Sub(&proof.ComGY, &gYBetaG1)

	okY, err := bn254.PairingCheck(
		[]bn254.G1Affine{comGYMinusZ, negAff(proof.Pi1AggH)},
		[]bn254.G2Affine{srs.G2Y[0], tauYMinusBeta},
	)
	if err != nil {
		t.Fatalf("Y 轴配对错误：%v", err)
	}
	t.Logf("Y 轴 KZG 验证：%v", okY)

	// ── X 轴 Shplonk 验证 ─────────────────────────────────────────────────────
	// e(ComQX, ZTG2) = e(C1, [τ_X-ωα]_2) · e(C2, [τ_X-α]_2)  （与 verify.go 约定一致）

	nuPow := make([]fr.Element, 14)
	nuPow[0].SetOne()
	for k := 1; k < 14; k++ {
		nuPow[k].Mul(&nuPow[k-1], &nu)
	}

	sAlphaComs := []dkzg.Digest{
		foldedHxDig, proof.LRO[0], proof.LRO[1], proof.LRO[2], proof.Z,
		vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3,
	}
	sAlphaEvals := []fr.Element{
		proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO, proof.EvalZ,
		proof.EvalQl, proof.EvalQr, proof.EvalQm, proof.EvalQo, proof.EvalQk,
		proof.EvalS1, proof.EvalS2, proof.EvalS3,
	}

	c1 := computeCAff(sAlphaComs, sAlphaEvals, nuPow[:13], g1Gen)
	c2 := computeCAff(
		[]dkzg.Digest{proof.Z},
		[]fr.Element{proof.EvalZS},
		[]fr.Element{nuPow[13]},
		g1Gen,
	)

	// 对双变量 DKZG 校正 C1 和 C2：witness 多项式修正。
	// c1 += evalAlphaCorr·g1 - ComVFAlpha
	// c2 += ν^13·evalZS·g1   - ν^13·ComVFZS
	{
		var evalAlphaCorr fr.Element
		for k, e := range []fr.Element{
			proof.EvalHx, proof.EvalA, proof.EvalB, proof.EvalO, proof.EvalZ,
		} {
			var tmp fr.Element
			tmp.Mul(&nuPow[k], &e)
			evalAlphaCorr.Add(&evalAlphaCorr, &tmp)
		}
		c1corrG1 := mulG1ByFr(g1Gen, evalAlphaCorr)
		var c1Jac bn254.G1Jac
		c1Jac.FromAffine(&c1)
		var corrJac bn254.G1Jac
		corrJac.FromAffine(&c1corrG1)
		c1Jac.AddAssign(&corrJac)
		var vfAlphaJac bn254.G1Jac
		vfAlphaJac.FromAffine(&proof.ComVFAlpha)
		c1Jac.SubAssign(&vfAlphaJac)
		c1.FromJacobian(&c1Jac)

		var nuZSEvalZS fr.Element
		nuZSEvalZS.Mul(&nuPow[13], &proof.EvalZS)
		c2corrG1 := mulG1ByFr(g1Gen, nuZSEvalZS)
		var c2Jac bn254.G1Jac
		c2Jac.FromAffine(&c2)
		var corr2Jac bn254.G1Jac
		corr2Jac.FromAffine(&c2corrG1)
		c2Jac.AddAssign(&corr2Jac)
		var vfZSJac bn254.G1Jac
		vfZSJac.FromAffine(&proof.ComVFZS)
		var b big.Int
		nuPow[13].BigInt(&b)
		vfZSJac.ScalarMultiplication(&vfZSJac, &b)
		c2Jac.SubAssign(&vfZSJac)
		c2.FromJacobian(&c2Jac)
	}

	// [Z_T(τ_X)]_2 = G2[2] - (α+ωα)·G2[1] + α·ωα·G2[0]
	var coeff1, coeff0 fr.Element
	coeff1.Add(&alpha, alphaShifted)
	coeff0.Mul(&alpha, alphaShifted)
	zTG2 := addG2(subG2(srs.G2[2], mulG2ByFr(srs.G2[1], coeff1)), mulG2ByFr(srs.G2[0], coeff0))

	// [τ_X - α]_2 = G2[1] - α·G2[0]
	tauXMinusAlpha := subG2(srs.G2[1], mulG2ByFr(srs.G2[0], alpha))

	// [τ_X - ω_X·α]_2 = G2[1] - ω_X·α·G2[0]
	tauXMinusAlphaS := subG2(srs.G2[1], mulG2ByFr(srs.G2[0], *alphaShifted))

	okX, err := bn254.PairingCheck(
		[]bn254.G1Affine{proof.ComQX, negAff(c1), negAff(c2)},
		[]bn254.G2Affine{zTG2, tauXMinusAlphaS, tauXMinusAlpha}, // C1↔[τ_X-ωα]，C2↔[τ_X-α]
	)
	if err != nil {
		t.Fatalf("X 轴配对错误：%v", err)
	}
	t.Logf("X 轴 Shplonk 验证：%v", okX)

	if !okX && !okY {
		t.Error("X 轴和 Y 轴验证均失败")
	} else if !okX {
		t.Error("X 轴 Shplonk 验证失败 — 问题出在 compress.go 的 Shplonk 商（ComQX）")
	} else if !okY {
		t.Error("Y 轴 KZG 验证失败 — 问题出在 compress.go 的 G_Y 承诺或商多项式")
	} else {
		t.Log("X 轴和 Y 轴独立验证均通过 — 问题出在两者的组合方式（D_lin/D_tau）")
	}
}

func negAff(p bn254.G1Affine) bn254.G1Affine {
	var r bn254.G1Affine
	r.Neg(&p)
	return r
}

func computeCAff(coms []dkzg.Digest, evals []fr.Element, nuPow []fr.Element, g1 bn254.G1Affine) bn254.G1Affine {
	var result bn254.G1Jac
	for k := range coms {
		var comJac bn254.G1Jac
		comJac.FromAffine(&coms[k])
		var evalG1 bn254.G1Jac
		evalG1.FromAffine(&g1)
		var b big.Int
		evals[k].BigInt(&b)
		evalG1.ScalarMultiplication(&evalG1, &b)
		comJac.SubAssign(&evalG1)
		nuPow[k].BigInt(&b)
		comJac.ScalarMultiplication(&comJac, &b)
		result.AddAssign(&comJac)
	}
	var aff bn254.G1Affine
	aff.FromJacobian(&result)
	return aff
}
