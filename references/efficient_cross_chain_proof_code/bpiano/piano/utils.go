package piano

import (
	"errors"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
	"github.com/oliverustc/bpiano/dkzg"
)

// ────────────────────────────────────────────────────────────────────────────
// 恒等置换域
// ────────────────────────────────────────────────────────────────────────────

// getIDSmallDomain 返回长度为 3T 的"恒等置换"求值向量，
// 覆盖 <ω_X> 的三个陪集：
//
//	[ω^0, ω^1, …, ω^{T-1}, u·ω^0, …, u·ω^{T-1}, u²·ω^0, …, u²·ω^{T-1}]
//
// 其中 u = domain.FrMultiplicativeGen。
func getIDSmallDomain(domain *fft.Domain) []fr.Element {
	T := int(domain.Cardinality)
	res := make([]fr.Element, 3*T)

	res[0].SetOne()
	res[T].Set(&domain.FrMultiplicativeGen)
	res[2*T].Square(&domain.FrMultiplicativeGen)

	for i := 1; i < T; i++ {
		res[i].Mul(&res[i-1], &domain.Generator)
		res[T+i].Mul(&res[T+i-1], &domain.Generator)
		res[2*T+i].Mul(&res[2*T+i-1], &domain.Generator)
	}
	return res
}

// ────────────────────────────────────────────────────────────────────────────
// Lagrange ↔ 规范系数转换
// ────────────────────────────────────────────────────────────────────────────

// lagrangeToCanonical 通过 IFFT 将多项式从 Lagrange（求值）形式转换为规范系数形式。
// 返回新切片，不修改输入。
func lagrangeToCanonical(lagrange []fr.Element, domain *fft.Domain) []fr.Element {
	out := make([]fr.Element, len(lagrange))
	copy(out, lagrange)
	domain.FFTInverse(out, fft.DIF)
	fft.BitReverse(out)
	return out
}

// canonicalToLagrange 通过 FFT 将多项式从规范系数形式转换为 Lagrange（求值）形式。
// 返回新切片，不修改输入。
func canonicalToLagrange(canonical []fr.Element, domain *fft.Domain) []fr.Element {
	out := make([]fr.Element, len(canonical))
	copy(out, canonical)
	fft.BitReverse(out)
	domain.FFT(out, fft.DIT)
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// 多项式求值辅助函数
// ────────────────────────────────────────────────────────────────────────────

// evalPolyLagrange 用标准公式在任意点 alpha 处对 Lagrange 系数多项式进行求值：
//
//	p(α) = (α^N - 1) / N · Σ_j ω^j · evals[j] / (α - ω^j)
//
// evals[j] = p(ω^j)，gen 是域的生成元（ω^N = 1）。
// 若 alpha 等于某个单位根 ω^j，则直接返回 evals[j]。
func evalPolyLagrange(evals []fr.Element, alpha, gen fr.Element) fr.Element {
	N := uint64(len(evals))

	// 检查 alpha 是否为单位根 ω^j。
	var cur fr.Element
	cur.SetOne()
	for j := uint64(0); j < N; j++ {
		if cur.Equal(&alpha) {
			return evals[j]
		}
		cur.Mul(&cur, &gen)
	}

	// 分子：α^N - 1。
	var alphaN, one fr.Element
	one.SetOne()
	alphaN.Exp(alpha, new(big.Int).SetUint64(N))
	alphaN.Sub(&alphaN, &one)

	// 构造分母：α - ω^j（每个 j 一个）。
	denoms := make([]fr.Element, N)
	cur.SetOne()
	for j := uint64(0); j < N; j++ {
		denoms[j].Sub(&alpha, &cur)
		cur.Mul(&cur, &gen)
	}
	denoms = fr.BatchInvert(denoms)

	// Σ_j ω^j · evals[j] / (α - ω^j)  ·  (α^N-1) / N。
	var nInv, omegaJ, sum fr.Element
	nInv.SetUint64(N).Inverse(&nInv)
	omegaJ.SetOne()

	for j := uint64(0); j < N; j++ {
		var term fr.Element
		term.Mul(&omegaJ, &evals[j])
		term.Mul(&term, &denoms[j])
		sum.Add(&sum, &term)
		omegaJ.Mul(&omegaJ, &gen)
	}

	sum.Mul(&sum, &alphaN)
	sum.Mul(&sum, &nInv)
	return sum
}

// evalPolyCanonical 用 Horner 方法在 alpha 处对规范形式多项式求值。
// poly[0] 为常数项。
func evalPolyCanonical(poly []fr.Element, alpha fr.Element) fr.Element {
	var res fr.Element
	for i := len(poly) - 1; i >= 0; i-- {
		res.Mul(&res, &alpha).Add(&res, &poly[i])
	}
	return res
}

// ────────────────────────────────────────────────────────────────────────────
// 移位：由 p(X) 的规范系数计算 p(ω·X) 的规范系数
// ────────────────────────────────────────────────────────────────────────────

// shiftCanonical 返回 p(omega·X) 的新规范系数。
// 若 p(X) = Σ_k c_k X^k，则 p(ω·X) = Σ_k c_k·ω^k · X^k。
func shiftCanonical(canonical []fr.Element, omega fr.Element) []fr.Element {
	out := make([]fr.Element, len(canonical))
	var omegaK fr.Element
	omegaK.SetOne()
	for k := range out {
		out[k].Mul(&canonical[k], &omegaK)
		omegaK.Mul(&omegaK, &omega)
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// 陪集消失多项式
// ────────────────────────────────────────────────────────────────────────────

// vanishingOnCoset 在 domainL（大域）的陪集上逐点计算 X^smallSize - 1。
// 陪集为 u·<ω_{4T}>，其中 u = domainL.FrMultiplicativeGen。
//
// 结果长度为 domainL.Cardinality，以自然序（j=0..4T-1）返回。
// 若调用方需要与 DIF OnCoset FFT 的比特逆序输出对齐，须在使用前调用 fft.BitReverse。
func vanishingOnCoset(domainL *fft.Domain, smallSize uint64) []fr.Element {
	bigN := domainL.Cardinality
	// 陪集生成元 u。
	u := domainL.FrMultiplicativeGen
	// 大域的生成元。
	genL := domainL.Generator

	res := make([]fr.Element, bigN)
	// 自然序第 j 点：u · ω_{4T}^j。
	var pt fr.Element
	pt.Set(&u)
	var one fr.Element
	one.SetOne()
	sizeExp := new(big.Int).SetUint64(smallSize)
	for j := uint64(0); j < bigN; j++ {
		var v fr.Element
		v.Exp(pt, sizeExp)
		v.Sub(&v, &one)
		res[j] = v
		pt.Mul(&pt, &genL)
	}
	return res
}

// ────────────────────────────────────────────────────────────────────────────
// Y 轴承诺（使用 DKZG SRS 的 Vy 元素）
// ────────────────────────────────────────────────────────────────────────────

// commitY 对 Y 轴域（长度 M）上的 Lagrange 求值向量进行承诺，
// 使用 Vy SRS 元素：
//
//	com = Σ_i lagrangeEvals[i] · Vy[i]
func commitY(lagrangeEvals []fr.Element, srs *dkzg.SRS) (dkzg.Digest, error) {
	M := len(lagrangeEvals)
	if M != len(srs.Vy) {
		return dkzg.Digest{}, errBadSize
	}

	points := srs.Vy
	scalars := lagrangeEvals

	// 多标量乘法（MSM）。
	var msmResult bn254.G1Affine
	bigScalars := make([]fr.Element, M)
	copy(bigScalars, scalars)
	if _, err := msmResult.MultiExp(points, bigScalars, ecc.MultiExpConfig{}); err != nil {
		return dkzg.Digest{}, err
	}
	return msmResult, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 商多项式辅助函数
// ────────────────────────────────────────────────────────────────────────────

// foldQuotient 计算 h1 + alphaPowN·h2 + alphaPowN²·h3（规范形式）。
// alphaPowN = α^T，三个输入均为长度 T 的规范形式多项式。
// 使用 Horner 方法：res = (h3·alphaPowN + h2)·alphaPowN + h1。
func foldQuotient(h1, h2, h3 []fr.Element, alphaPowN fr.Element) []fr.Element {
	T := len(h1)
	out := make([]fr.Element, T)
	for j := 0; j < T; j++ {
		out[j].Mul(&h3[j], &alphaPowN)
		out[j].Add(&out[j], &h2[j])
		out[j].Mul(&out[j], &alphaPowN)
		out[j].Add(&out[j], &h1[j])
	}
	return out
}

// padTo 通过在末尾追加零将切片填充到长度 n。
func padTo(s []fr.Element, n int) []fr.Element {
	out := make([]fr.Element, n)
	copy(out, s)
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// 错误定义
// ────────────────────────────────────────────────────────────────────────────

var (
	errBadSize      = errors.New("piano: 大小不匹配")
	errZNotOne      = errors.New("piano: Z(T) ≠ 1（约束未满足）")
	errVerification = errors.New("piano: 证明验证失败")
)
