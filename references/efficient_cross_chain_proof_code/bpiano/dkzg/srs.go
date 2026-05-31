// Package dkzg 实现了针对双变量多项式 F(Y, X) = Σ_{i=0}^{M-1} R_i(Y) · f_i(X) 的
// 分布式 KZG（DKZG）承诺方案。
//
// 每个子节点 i 持有一个单变量多项式 f_i(X)，使用 X 轴 Lagrange SRS 进行本地承诺。
// 主节点将各子节点的本地承诺聚合为对 F 的全局承诺，并协调 Y 轴的开放证明。
package dkzg

import (
	"errors"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
)

var (
	ErrMNotPowerOfTwo      = errors.New("dkzg: M 必须是 2 的幂且 >= 2")
	ErrTNotPowerOfTwo      = errors.New("dkzg: T 必须是 2 的幂且 >= 2")
	ErrAlphaIsRootOfUnityX = errors.New("dkzg: 求值点 alpha 是 X 轴的单位根")
	ErrBetaIsRootOfUnityY  = errors.New("dkzg: 求值点 beta 是 Y 轴的单位根")
	ErrInvalidNodeIndex    = errors.New("dkzg: 节点索引越界")
	ErrInvalidPolySize     = errors.New("dkzg: 多项式长度与 SRS 不匹配")
	ErrInvalidEvalsSize    = errors.New("dkzg: 求值向量长度与 M 不匹配")
	ErrVerification        = errors.New("dkzg: pairing 校验失败")
	ErrMismatchedInputs    = errors.New("dkzg: 批量操作的输入数量不一致")
)

// SRS 是 DKZG 方案的结构化参考字符串（Structured Reference String）。
//
// X 轴使用 Lagrange 形式的 SRS：
//
//	Ux[i][j] = g1^{R_i(τ_Y) · L_j(τ_X)}
//
// Y 轴使用承诺基：
//
//	Vy[i] = g1^{R_i(τ_Y)}
//
// 验证使用 G2 元素：
//
//	G2[0] = g2,  G2[1] = g2^{τ_X},  G2[2] = g2^{τ_X²}  （X 轴 pairing）
//	G2Y[0] = g2, G2Y[1] = g2^{τ_Y}                      （Y 轴 pairing）
//
// G2[2] 用于 BPiano Shplonk 两点验证公式：
//
//	[Z_T(τ_X)]_2 = G2[2] - (α+ω·α)·G2[1] + α·ω·α·G2[0]
type SRS struct {
	Ux      [][]bn254.G1Affine // [M][T]，X 轴 Lagrange 承诺基
	Vy      []bn254.G1Affine   // [M]，Y 轴承诺基
	G2      [3]bn254.G2Affine  // [g2, g2^{τ_X}, g2^{τ_X²}]
	G2Y     [2]bn254.G2Affine  // [g2, g2^{τ_Y}]
	DomainX fft.Domain         // X 轴 FFT 域（基数为 T）
	DomainY fft.Domain         // Y 轴 FFT 域（基数为 M）
}

// NewSRS 使用随机陷门 τ_X 和 τ_Y 生成 DKZG SRS。
// M 为子节点数量，T 为子电路规模，两者均须为 2 的幂且 >= 2。
func NewSRS(M, T uint64) (*SRS, error) {
	if M < 2 || M&(M-1) != 0 {
		return nil, ErrMNotPowerOfTwo
	}
	if T < 2 || T&(T-1) != 0 {
		return nil, ErrTNotPowerOfTwo
	}

	// 随机生成陷门。
	var tauX, tauY fr.Element
	if _, err := tauX.SetRandom(); err != nil {
		return nil, err
	}
	if _, err := tauY.SetRandom(); err != nil {
		return nil, err
	}
	var tauXBig, tauYBig big.Int
	tauX.BigInt(&tauXBig)
	tauY.BigInt(&tauYBig)

	srs, err := newSRSFromTau(M, T, &tauXBig, &tauYBig)

	// 使用完毕后立即清零陷门。
	tauX.SetZero()
	tauY.SetZero()
	tauXBig.SetInt64(0)
	tauYBig.SetInt64(0)

	return srs, err
}

// NewTestSRS 使用固定陷门生成 DKZG SRS，仅用于测试。
// 禁止在生产环境中使用。
func NewTestSRS(M, T uint64, tauX, tauY *big.Int) (*SRS, error) {
	if M < 2 || M&(M-1) != 0 {
		return nil, ErrMNotPowerOfTwo
	}
	if T < 2 || T&(T-1) != 0 {
		return nil, ErrTNotPowerOfTwo
	}
	return newSRSFromTau(M, T, tauX, tauY)
}

// newSRSFromTau 根据给定陷门计算所有 SRS 元素。
func newSRSFromTau(M, T uint64, tauX, tauY *big.Int) (*SRS, error) {
	_, _, g1Aff, g2Aff := bn254.Generators()

	srs := &SRS{}

	// 创建 FFT 域。fft.NewDomain 在参数非法时会 panic，
	// 但此处 M 和 T 已经过合法性校验。
	srs.DomainX = *fft.NewDomain(T)
	srs.DomainY = *fft.NewDomain(M)

	// 将陷门转换为域元素。
	var tauXFr, tauYFr fr.Element
	tauXFr.SetBigInt(tauX)
	tauYFr.SetBigInt(tauY)

	// 计算陷门处的 Lagrange 基值：
	// lagX[j] = L_j(τ_X)，lagY[i] = R_i(τ_Y)。
	lagX, err := evalLagrangeBasis(tauXFr, T, srs.DomainX.Generator)
	if err != nil {
		return nil, err
	}
	lagY, err := evalLagrangeBasis(tauYFr, M, srs.DomainY.Generator)
	if err != nil {
		return nil, err
	}

	// 构建 Ux[i][j] = g1^{lagY[i] * lagX[j]}。
	// 先计算全部 M*T 个标量，再做一次大批量标量乘法。
	allScalars := make([]fr.Element, M*T)
	for i := uint64(0); i < M; i++ {
		for j := uint64(0); j < T; j++ {
			allScalars[i*T+j].Mul(&lagY[i], &lagX[j])
		}
	}
	// 转换为非 Montgomery 形式以供 BatchScalarMultiplicationG1 使用。
	scalarsBig := make([]big.Int, M*T)
	for k := range allScalars {
		allScalars[k].BigInt(&scalarsBig[k])
	}
	allG1s := bn254.BatchScalarMultiplicationG1(&g1Aff, toFrSlice(allScalars))

	srs.Ux = make([][]bn254.G1Affine, M)
	for i := uint64(0); i < M; i++ {
		srs.Ux[i] = allG1s[i*T : (i+1)*T]
	}

	// 构建 Vy[i] = g1^{lagY[i]}。
	srs.Vy = bn254.BatchScalarMultiplicationG1(&g1Aff, lagY)

	// 构建 G2 元素。
	srs.G2[0] = g2Aff
	srs.G2[1].ScalarMultiplication(&g2Aff, tauX)
	// G2[2] = g2^{τ_X²}：将 G2[1] 再乘以 τ_X。
	srs.G2[2].ScalarMultiplication(&srs.G2[1], tauX)

	srs.G2Y[0] = g2Aff
	srs.G2Y[1].ScalarMultiplication(&g2Aff, tauY)

	return srs, nil
}

// evalLagrangeBasis 计算 {L_j(alpha)}，j = 0,...,domainSize-1，
// 其中 L_j 是以 gen（domainSize 次本原单位根）为生成元的域上第 j 个 Lagrange 基多项式。
//
// 公式：L_j(alpha) = ω^j · (alpha^N - 1) / (N · (alpha - ω^j))
//
// 若 alpha 是该域的单位根（分母为零），则返回错误。
func evalLagrangeBasis(alpha fr.Element, domainSize uint64, gen fr.Element) ([]fr.Element, error) {
	N := domainSize

	// 计算 alpha^N。
	var alphaN fr.Element
	{
		exp := new(big.Int).SetUint64(N)
		alphaN.Exp(alpha, exp)
	}

	// 分子 = alpha^N - 1。
	var one fr.Element
	one.SetOne()
	var numerator fr.Element
	numerator.Sub(&alphaN, &one)

	// 若 alpha^N == 1，则 alpha 是该域的单位根。
	if numerator.IsZero() {
		return nil, ErrAlphaIsRootOfUnityX
	}

	// 计算域大小的逆元 N_inv = 1/N（在 Fr 中）。
	var NField, NInv fr.Element
	NField.SetUint64(N)
	NInv.Inverse(&NField)

	// numeratorScaled = (alpha^N - 1) / N。
	var numeratorScaled fr.Element
	numeratorScaled.Mul(&numerator, &NInv)

	// 计算分母：denom[j] = alpha - gen^j，同时保存 gen^j 供最终乘法使用。
	denoms := make([]fr.Element, N)
	genPows := make([]fr.Element, N)
	genPow := one // gen^0 = 1
	for j := uint64(0); j < N; j++ {
		genPows[j] = genPow
		denoms[j].Sub(&alpha, &genPow)
		genPow.Mul(&genPow, &gen)
	}

	// 批量求逆（BatchInvert 返回新切片，不修改原切片）。
	denoms = fr.BatchInvert(denoms)

	// L_j(alpha) = gen^j · (alpha^N - 1) / (N · (alpha - gen^j))
	//            = gen^j · numeratorScaled · denoms[j]。
	result := make([]fr.Element, N)
	for j := uint64(0); j < N; j++ {
		result[j].Mul(&numeratorScaled, &denoms[j])
		result[j].Mul(&result[j], &genPows[j])
	}

	return result, nil
}

// toFrSlice 返回切片本身（gnark-crypto 的 BatchScalarMultiplicationG1
// 接受 Montgomery 形式的 fr.Element，无需额外转换）。
func toFrSlice(s []fr.Element) []fr.Element {
	return s
}
