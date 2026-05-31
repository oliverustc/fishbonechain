package piano

// 本文件将包内部辅助函数导出，供 bpiano 包使用。
// 这些函数是对未导出实现的轻量级包装。

import (
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
	"github.com/oliverustc/bpiano/dkzg"
)

// LagrangeToCanonical 将 Lagrange 求值向量转换为规范系数形式。
func LagrangeToCanonical(lagrange []fr.Element, domain *fft.Domain) []fr.Element {
	return lagrangeToCanonical(lagrange, domain)
}

// CanonicalToLagrange 将规范系数转换为 Lagrange 求值向量。
func CanonicalToLagrange(canonical []fr.Element, domain *fft.Domain) []fr.Element {
	return canonicalToLagrange(canonical, domain)
}

// CosetEval 在 domainL 的陪集上对规范形式多项式进行求值（DIF 比特逆序输出）。
func CosetEval(canonical []fr.Element, domainL *fft.Domain) []fr.Element {
	return cosetEval(canonical, domainL)
}

// CosetPoints 返回 DIF 比特逆序排列的陪集点。
func CosetPoints(domainL *fft.Domain) []fr.Element {
	return cosetPoints(domainL)
}

// L0OnCoset 在 domainL 的陪集上计算 L_0(X)（DIF 比特逆序排列）。
func L0OnCoset(domainL *fft.Domain, T int) []fr.Element {
	return l0OnCoset(domainL, T)
}

// VanishingOnCoset 在陪集上计算 X^smallSize - 1（自然序输出）。
func VanishingOnCoset(domainL *fft.Domain, smallSize uint64) []fr.Element {
	return vanishingOnCoset(domainL, smallSize)
}

// ShiftCanonical 返回 p(ω·X) 的规范系数。
func ShiftCanonical(canonical []fr.Element, omega fr.Element) []fr.Element {
	return shiftCanonical(canonical, omega)
}

// FoldQuotient 计算 h1 + alphaPowN·h2 + alphaPowN²·h3（规范形式）。
func FoldQuotient(h1, h2, h3 []fr.Element, alphaPowN fr.Element) []fr.Element {
	return foldQuotient(h1, h2, h3, alphaPowN)
}

// FoldQuotientLagrange 逐点折叠 hy1 + betaPowM·hy2 + betaPowM²·hy3。
func FoldQuotientLagrange(hy1, hy2, hy3 []fr.Element, betaPowM fr.Element) []fr.Element {
	return foldQuotientLagrange(hy1, hy2, hy3, betaPowM)
}

// FoldDigests 计算 com0 + aPowN·com1 + aPowN²·com2（G1 标量乘法）。
func FoldDigests(com0, com1, com2 dkzg.Digest, aPowN *fr.Element) dkzg.Digest {
	return foldDigests(com0, com1, com2, aPowN)
}

// EvalPolyLagrange 在任意点处对 Lagrange 形式多项式进行求值。
func EvalPolyLagrange(evals []fr.Element, alpha, gen fr.Element) fr.Element {
	return evalPolyLagrange(evals, alpha, gen)
}

// EvalPolyCanonical 用 Horner 方法在 alpha 处对规范形式多项式进行求值。
func EvalPolyCanonical(poly []fr.Element, alpha fr.Element) fr.Element {
	return evalPolyCanonical(poly, alpha)
}

// ComputeLagrange0 计算 L_0(α) = (α^T-1)/(T·(α-1))。
func ComputeLagrange0(alpha fr.Element, cardinalityInv fr.Element) fr.Element {
	return computeLagrange0(alpha, cardinalityInv)
}

// CommitY 对给定 Y 轴域上的 Lagrange 求值向量进行承诺。
func CommitY(lagrangeEvals []fr.Element, srs *dkzg.SRS) (dkzg.Digest, error) {
	return commitY(lagrangeEvals, srs)
}

// ComputeZLagrange 以 Lagrange 形式计算置换累加器 Z。
func ComputeZLagrange(l, r, o []fr.Element, pk *ProvingKey, eta, gamma fr.Element) ([]fr.Element, error) {
	return computeZLagrange(l, r, o, pk, eta, gamma)
}
