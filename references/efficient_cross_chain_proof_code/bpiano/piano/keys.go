// Package piano 实现了 Piano 协议：一种数据并行的 PLONK 变体，
// 其中 M 个子电路共享相同结构（选择子、置换），但持有不同的 witness。
// 承诺使用兄弟包 dkzg 中的分布式 KZG（DKZG）方案。
//
// 本包独立于 gnark 的电路编译器，调用方需预先计算好选择子、witness 和置换向量，
// 以 X 轴 FFT 域上的 Lagrange（求值）形式提供。
package piano

import (
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
	"github.com/oliverustc/bpiano/dkzg"
)

// ────────────────────────────────────────────────────────────────────────────
// 电路描述
// ────────────────────────────────────────────────────────────────────────────

// CircuitInfo 描述共享的电路结构（选择子 + 置换）。
// 所有切片长度均为 T = DomainX.Cardinality，以 Lagrange 形式存储
// （即第 j 项是多项式在 ω_X^j 处的取值）。
//
// 门等式（第 j 行）：Ql[j]·L + Qr[j]·R + Qm[j]·L·R + Qo[j]·O + Qk[j] = 0
//
// 置换：Permutation[i]（i ∈ [0, 3T)）编码置换 σ，使得：
//
//	S1[j] = evaluationIDSmallDomain[Permutation[j]]
//	S2[j] = evaluationIDSmallDomain[Permutation[T+j]]
//	S3[j] = evaluationIDSmallDomain[Permutation[2T+j]]
//
// 其中 evaluationIDSmallDomain = [ω^0..ω^{T-1} | u·ω^0.. | u²·ω^0..]。
type CircuitInfo struct {
	// 选择子多项式，Lagrange 形式，各长度为 T。
	Ql, Qr, Qm, Qo, Qk []fr.Element
	// 连线置换，长度为 3T。第 i 项给出槽 i 的目标位置
	// （槽编号：0..T-1 = L 线，T..2T-1 = R 线，2T..3T-1 = O 线）。
	Permutation []int64
	// NbPublicInputs 是公开输入行数（L 列最前面若干行，
	// 其中 Ql[i]=-1，Qk[i]=public_input[i]，其余为 0）。
	NbPublicInputs int
}

// WitnessInstance 保存单个子节点的 Lagrange 形式 witness。
// L、R、O 的长度均须等于 X 轴域的大小 T。
type WitnessInstance struct {
	L, R, O []fr.Element
}

// ────────────────────────────────────────────────────────────────────────────
// 密钥
// ────────────────────────────────────────────────────────────────────────────

// ProvingKey 包含生成 Piano 证明所需的全部数据。
type ProvingKey struct {
	Vk *VerifyingKey

	// X 轴 FFT 域：DomainX（大小 T），DomainXL（大小 4T，用于商多项式）。
	DomainX  fft.Domain
	DomainXL fft.Domain

	// Y 轴 FFT 域：DomainY（大小 M），DomainYL（大小 4M，用于商多项式）。
	DomainY  fft.Domain
	DomainYL fft.Domain

	// 选择子多项式，Lagrange 形式（长度 T，各子节点共享）。
	Ql, Qr, Qm, Qo, Qk []fr.Element

	// 置换多项式 S1、S2、S3，Lagrange 形式（长度 T）。
	// S_k[j] = evaluationIDSmallDomain[Permutation[k*T + j]]。
	S1, S2, S3 []fr.Element

	// 原始置换索引（长度 3T）。
	Permutation []int64

	// DKZG SRS，同时用于 X 轴和 Y 轴承诺。
	DKZGSRS *dkzg.SRS
}

// VerifyingKey 包含验证 Piano 证明所需的数据。
type VerifyingKey struct {
	SizeX          uint64     // T：X 轴域大小
	SizeY          uint64     // M：Y 轴域大小（子节点数）
	NbPublicInputs int        // 公开输入数量
	GeneratorX     fr.Element // ω_X：T 次本原单位根
	GeneratorY     fr.Element // ω_Y：M 次本原单位根
	CosetShift     fr.Element // u = DomainX 的乘法生成元（陪集偏移）

	DKZGSRS *dkzg.SRS

	// 共享多项式的 DKZG 全局承诺。
	Ql, Qr, Qm, Qo, Qk dkzg.Digest
	S1, S2, S3          dkzg.Digest
}

// ────────────────────────────────────────────────────────────────────────────
// 证明
// ────────────────────────────────────────────────────────────────────────────

// Proof 是由 Prove 生成、由 Verify 接受的 Piano 证明。
type Proof struct {
	// === 承诺 ===

	// witness 多项式 A、B、O 的全局承诺（X 轴 DKZG）。
	LRO [3]dkzg.Digest

	// 置换累加器 Z 的全局承诺。
	Z dkzg.Digest

	// X 轴商多项式三份 H_X,0 / H_X,1 / H_X,2 的全局承诺。
	Hx [3]dkzg.Digest

	// Y 轴商多项式三份 H_Y,0 / H_Y,1 / H_Y,2 的承诺。
	Hy [3]dkzg.Digest

	// === X 轴批量开放证明（在 α 处）===

	// BatchedProofX 是对 13 个多项式的 Fiat-Shamir 折叠 X 轴证明：
	//   [foldedHx, A, B, O, Ql, Qr, Qm, Qo, Qk, S1, S2, S3, Z]
	// H 是折叠后的单个商多项式；ClaimedDigests[k] = comVF_k。
	BatchedProofX dkzg.BatchedProofX

	// ZShiftedProofX 是 Z 在 ω_X·α 处的聚合 X 轴开放证明。
	ZShiftedProofX dkzg.AggregatedProofX

	// === Y 轴批量开放证明（在 β 处）===

	// BatchedProofY 是对 15 个多项式的 Fiat-Shamir 折叠 Y 轴证明：
	//   [foldedHx, A, B, O, Ql, Qr, Qm, Qo, Qk, S1, S2, S3, Z, ZS, foldedHy]
	// H 是折叠后的单个商多项式；ClaimedValues[k] 是在 β 处的标量求值。
	BatchedProofY dkzg.BatchedProofY

	// === (β, α) 处的声明求值值 ===

	ClaimedA  fr.Element // A(β, α)
	ClaimedB  fr.Element // B(β, α)
	ClaimedO  fr.Element // O(β, α)
	ClaimedZ  fr.Element // Z(β, α)
	ClaimedZS fr.Element // Z(β, ω_X·α)
	ClaimedHx fr.Element // foldedHx(β, α)
	ClaimedHy fr.Element // foldedHy(β)

	// 选择子/置换多项式在 α 处的求值（标量，各子节点相同）。
	ClaimedQl fr.Element
	ClaimedQr fr.Element
	ClaimedQm fr.Element
	ClaimedQo fr.Element
	ClaimedQk fr.Element
	ClaimedS1 fr.Element
	ClaimedS2 fr.Element
	ClaimedS3 fr.Element
}

// negG1 返回 G1 仿射点的取反。
func negG1(p bn254.G1Affine) bn254.G1Affine {
	p.Neg(&p)
	return p
}
