package solgen

import (
	"crypto/sha256"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/piano"
)

// AggCalldataResult 包含 Solidity AggBPianoVerifier.verify() 所需的全部输入。
type AggCalldataResult struct {
	Proofs     []*bpiano.CompressedProof
	ComQXTotal [G1Size]byte // EVM 非压缩格式（64 字节）
	Pi1Total   [G1Size]byte
	ZTG2       [G2Size]byte // [Z_T(τ_X)]₂，依赖共享 α
	TauYBetaG2 [G2Size]byte // [τ_Y - β]₂，依赖共享 β

	// 调试字段（Solidity 链上重新推导，此处供对齐验证）
	SharedAlpha fr.Element
	SharedBeta  fr.Element
}

// GenerateAggCalldata 从 AggregatedProof 生成 Solidity 验证所需的 calldata。
//
// 步骤：
//  1. 从所有证明的 Hx 重推共享 α
//  2. 从共享 α 及各证明的 ComQX 等重推共享 β
//  3. 链下计算 ZTG2 = [Z_T(τ_X)]₂ 和 TauYBetaG2 = [τ_Y-β]₂（EVM 无法做 G2 标量乘）
func GenerateAggCalldata(
	agg *bpiano.AggregatedProof,
	vk *piano.VerifyingKey,
) (*AggCalldataResult, error) {
	proofs := agg.Proofs

	// ── 1. 重推共享 α ─────────────────────────────────────────────────────────
	sharedAlpha := aggDeriveSharedAlpha(proofs)

	// ── 2. 重推共享 β ─────────────────────────────────────────────────────────
	sharedBeta := aggDeriveSharedBeta(sharedAlpha, proofs)

	// ── 3. 计算 alphaShifted = α · ω_X ───────────────────────────────────────
	var alphaShifted fr.Element
	alphaShifted.Mul(&sharedAlpha, &vk.GeneratorX)

	// ── 4. 链下计算 G2 点 ─────────────────────────────────────────────────────
	srs := vk.DKZGSRS
	zTG2 := computeZTG2(sharedAlpha, alphaShifted, srs)
	tauYBetaG2 := computeTauYMinusBetaG2(sharedBeta, srs)

	return &AggCalldataResult{
		Proofs:      proofs,
		ComQXTotal:  G1Bytes(agg.ComQXTotal),
		Pi1Total:    G1Bytes(agg.Pi1Total),
		ZTG2:        G2Bytes(zTG2),
		TauYBetaG2:  G2Bytes(tauYBetaG2),
		SharedAlpha: sharedAlpha,
		SharedBeta:  sharedBeta,
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 内部：共享挑战重推（与 bpiano.VerifyBatch 格式完全一致）
// ────────────────────────────────────────────────────────────────────────────

// aggDeriveSharedAlpha 从所有证明的 Hx[0..2] 重推共享 α。
// 格式：SHA256("coord-alpha" || Hx^{(0)}[0..2] || ... || Hx^{(K-1)}[0..2])
func aggDeriveSharedAlpha(proofs []*bpiano.CompressedProof) fr.Element {
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

// aggDeriveSharedBeta 从共享 α 及各证明的 ComQX/ComVFAlpha/ComVFZS 重推共享 β。
// 格式：SHA256("coord-beta" || alpha || ComQX^{(0)} || ComVFAlpha^{(0)} || ComVFZS^{(0)} || ...)
func aggDeriveSharedBeta(sharedAlpha fr.Element, proofs []*bpiano.CompressedProof) fr.Element {
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
