package bpiano

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/piano"
)

// Fr 是 fr.Element 的别名，方便调用方无需直接导入 gnark-crypto。
type Fr = fr.Element

// AggregatedProof 是 K 个共享 α、β 的压缩证明经聚合后的批量证明（对应论文 §4.3.2）。
//
// 验证路径：CoordinateChallenges → AggregateProofs → VerifyBatch（常数 4 次 pairing）。
type AggregatedProof struct {
	K      int
	Proofs []*CompressedProof // K 份完整压缩证明（含承诺与求值，验证端重建聚合量时使用）

	// §4.3.2 定义的聚合 KZG 证明（由 AggregateProofs 计算）
	ComQXTotal dkzg.Digest // com_{Q_X,total} = Σ r_k · com_{Q_X}^{(k)}
	Pi1Total   dkzg.Digest // π_{1,total}     = Σ r_k · π_{1,agg}^{(k)}
}

// BatchProof 是 AggregatedProof 的别名，保持与旧接口的兼容性。
type BatchProof = AggregatedProof

// ────────────────────────────────────────────────────────────────────────────
// CoordinateChallenges（§4.3.1 两轮挑战协调）
// ────────────────────────────────────────────────────────────────────────────

// CoordinateChallenges 为 K 对 (pk, witnesses) 生成 K 个共享 α/β 的压缩证明。
//
// 协议流程：
//
//	阶段一：K 个 CompressStage1 → 汇总 Hx^{(k)} → 推导共享 α
//	阶段二：K 个 CompressStage2（注入 sharedAlpha） → 汇总 ComQX 等 → 推导共享 β
//	阶段三：K 个 CompressStage3（注入 sharedBeta）  → K 个 CompressedProof
func CoordinateChallenges(
	pks []*piano.ProvingKey,
	witnessSlices [][]piano.WitnessInstance,
	publicInputs [][][]fr.Element,
) ([]*CompressedProof, error) {
	K := len(pks)
	if len(witnessSlices) != K || len(publicInputs) != K {
		return nil, fmt.Errorf("bpiano: 输入长度不匹配（pks=%d witnessSlices=%d publicInputs=%d）",
			K, len(witnessSlices), len(publicInputs))
	}

	// ── 阶段一：并行执行 CompressStage1 ──────────────────────────────────────
	states1 := make([]*ProveState1, K)
	for k := 0; k < K; k++ {
		s, err := CompressStage1(pks[k], witnessSlices[k], publicInputs[k])
		if err != nil {
			return nil, fmt.Errorf("bpiano: stage1[%d]: %w", k, err)
		}
		states1[k] = s
	}

	// 派生共享 α
	sharedAlpha := deriveSharedAlpha(states1)

	// ── 阶段二：并行执行 CompressStage2 ──────────────────────────────────────
	states2 := make([]*ProveState2, K)
	for k := 0; k < K; k++ {
		s, err := CompressStage2(states1[k], sharedAlpha)
		if err != nil {
			return nil, fmt.Errorf("bpiano: stage2[%d]: %w", k, err)
		}
		states2[k] = s
	}

	// 派生共享 β
	sharedBeta := deriveSharedBeta(sharedAlpha, states2)

	// ── 阶段三：并行执行 CompressStage3 ──────────────────────────────────────
	proofs := make([]*CompressedProof, K)
	for k := 0; k < K; k++ {
		p, err := CompressStage3(states2[k], sharedBeta)
		if err != nil {
			return nil, fmt.Errorf("bpiano: stage3[%d]: %w", k, err)
		}
		proofs[k] = p
	}
	return proofs, nil
}

// ────────────────────────────────────────────────────────────────────────────
// AggregateProofs（§4.3.2 承诺聚合）
// ────────────────────────────────────────────────────────────────────────────

// AggregateProofs 将 K 个由 CoordinateChallenges 生成的压缩证明聚合为批量证明。
//
// 前提：proofs 中所有证明共享相同的 α 和 β（由 CoordinateChallenges 保证）。
// 计算：
//
//	r_k = SHA256("agg-rk" || k_bytes || serialize(π^{(0)}) || ... || serialize(π^{(K-1)}))
//	com_{Q_X,total} = Σ r_k · com_{Q_X}^{(k)}  （G1 MSM）
//	π_{1,total}     = Σ r_k · π_{1,agg}^{(k)}  （G1 MSM）
func AggregateProofs(proofs []*CompressedProof) (*AggregatedProof, error) {
	K := len(proofs)
	if K == 0 {
		return nil, fmt.Errorf("bpiano: 没有可聚合的证明")
	}

	rk := deriveAggCoeffs(proofs)

	// com_{Q_X,total} = Σ r_k · ComQX^{(k)}
	comQXPoints := make([]dkzg.Digest, K)
	for k, p := range proofs {
		comQXPoints[k] = p.ComQX
	}
	comQXTotal, err := g1MSM(comQXPoints, rk)
	if err != nil {
		return nil, err
	}

	// π_{1,total} = Σ r_k · Pi1AggH^{(k)}
	pi1Points := make([]dkzg.Digest, K)
	for k, p := range proofs {
		pi1Points[k] = p.Pi1AggH
	}
	pi1Total, err := g1MSM(pi1Points, rk)
	if err != nil {
		return nil, err
	}

	return &AggregatedProof{
		K:          K,
		Proofs:     proofs,
		ComQXTotal: comQXTotal,
		Pi1Total:   pi1Total,
	}, nil
}

// Aggregate 是 AggregateProofs 的别名，保持与旧接口的兼容性。
func Aggregate(proofs []*CompressedProof) (*AggregatedProof, error) {
	return AggregateProofs(proofs)
}

// ────────────────────────────────────────────────────────────────────────────
// 共享挑战派生（CoordinateChallenges 内部使用）
// ────────────────────────────────────────────────────────────────────────────

// deriveSharedAlpha 从所有 K 个证明的 Hx^{(k)}[0..2] 联合派生共享 α。
//
// 格式：SHA256("coord-alpha" || Hx^{(0)}[0] || Hx^{(0)}[1] || Hx^{(0)}[2]
//
//	|| Hx^{(1)}[0] || ... || Hx^{(K-1)}[2])
func deriveSharedAlpha(states []*ProveState1) fr.Element {
	h := sha256.New()
	h.Write([]byte("coord-alpha"))
	for _, s := range states {
		for _, d := range s.hx {
			b := d.Bytes()
			h.Write(b[:])
		}
	}
	var alpha fr.Element
	alpha.SetBytes(h.Sum(nil))
	return alpha
}

// deriveSharedBeta 在共享 α 确定后，从所有 K 个证明的
// ComQX^{(k)}/ComVFAlpha^{(k)}/ComVFZS^{(k)} 联合派生共享 β。
//
// 格式：SHA256("coord-beta" || alpha_bytes
//
//	|| ComQX^{(0)} || ComVFAlpha^{(0)} || ComVFZS^{(0)}
//	|| ComQX^{(1)} || ... || ComVFZS^{(K-1)})
func deriveSharedBeta(sharedAlpha fr.Element, states []*ProveState2) fr.Element {
	h := sha256.New()
	h.Write([]byte("coord-beta"))
	b := sharedAlpha.Bytes()
	h.Write(b[:])
	for _, s := range states {
		for _, d := range []dkzg.Digest{s.comQX, s.comVFAlpha, s.comVFZS} {
			b2 := d.Bytes()
			h.Write(b2[:])
		}
	}
	var beta fr.Element
	beta.SetBytes(h.Sum(nil))
	return beta
}

// ────────────────────────────────────────────────────────────────────────────
// 聚合系数派生（AggregateProofs 和 VerifyBatch 均使用相同格式）
// ────────────────────────────────────────────────────────────────────────────

// deriveAggCoeffs 为 K 个压缩证明推导聚合系数 r_0..r_{K-1}。
//
// 格式：r_k = SHA256("agg-rk" || k（4字节大端）|| serialize(π^{(0)}) || ... || serialize(π^{(K-1)}))
func deriveAggCoeffs(proofs []*CompressedProof) []fr.Element {
	K := len(proofs)
	// 一次性序列化所有证明
	var allData []byte
	for _, p := range proofs {
		allData = append(allData, serializeCompressedProof(p)...)
	}
	rk := make([]fr.Element, K)
	for k := 0; k < K; k++ {
		h := sha256.New()
		h.Write([]byte("agg-rk"))
		h.Write([]byte{byte(k >> 24), byte(k >> 16), byte(k >> 8), byte(k)})
		h.Write(allData)
		rk[k].SetBytes(h.Sum(nil))
	}
	return rk
}

// serializeCompressedProof 将 CompressedProof 序列化为字节切片（用于派生聚合系数）。
//
// 格式：LRO[0..2] || Z || Hx[0..2] || ComQX || ComVFAlpha || ComVFZS || ComGY || Pi1AggH
//
//	（各 G1 压缩格式 32 字节）
//	|| EvalA..EvalS3（各 Fr 大端 32 字节）
func serializeCompressedProof(p *CompressedProof) []byte {
	var out []byte
	writeG1 := func(d dkzg.Digest) {
		b := d.Bytes()
		out = append(out, b[:]...)
	}
	writeFr := func(e fr.Element) {
		b := e.Bytes()
		out = append(out, b[:]...)
	}
	for _, d := range p.LRO {
		writeG1(d)
	}
	writeG1(p.Z)
	for _, d := range p.Hx {
		writeG1(d)
	}
	writeG1(p.ComQX)
	writeG1(p.ComVFAlpha)
	writeG1(p.ComVFZS)
	writeG1(p.ComGY)
	writeG1(p.Pi1AggH)
	for _, e := range []fr.Element{
		p.EvalA, p.EvalB, p.EvalO, p.EvalZ, p.EvalZS,
		p.EvalHx, p.EvalHy,
		p.EvalQl, p.EvalQr, p.EvalQm, p.EvalQo, p.EvalQk,
		p.EvalS1, p.EvalS2, p.EvalS3,
	} {
		writeFr(e)
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// G1 辅助函数
// ────────────────────────────────────────────────────────────────────────────

// g1MSM 计算 Σ scalars[k] · points[k]（标量-多基 MSM，结果为 G1Affine）。
func g1MSM(points []dkzg.Digest, scalars []fr.Element) (dkzg.Digest, error) {
	if len(points) != len(scalars) {
		return dkzg.Digest{}, fmt.Errorf("bpiano g1MSM: len(points)=%d ≠ len(scalars)=%d",
			len(points), len(scalars))
	}
	var result bn254.G1Jac
	for k := range points {
		var p bn254.G1Jac
		p.FromAffine(&points[k])
		var b big.Int
		scalars[k].BigInt(&b)
		p.ScalarMultiplication(&p, &b)
		result.AddAssign(&p)
	}
	var aff bn254.G1Affine
	aff.FromJacobian(&result)
	return aff, nil
}
