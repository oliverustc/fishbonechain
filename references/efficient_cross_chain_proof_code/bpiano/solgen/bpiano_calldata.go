package solgen

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/piano"
)

// BPianoCalldata 包含 BPiano Solidity 验证合约所需的全部输入。
//
// Solidity 合约将链上重放 Fiat-Shamir 挑战并执行代数约束检验；
// 但 EVM 缺少 G2 标量乘预编译，因此 ZTG2 和 TauYBetaG2 必须由 Go 侧预计算。
type BPianoCalldata struct {
	// ── 证明 G1 承诺（各 64 字节，EVM 非压缩格式）────────────────────────────
	LRO         [3][G1Size]byte // com_A, com_B, com_O
	Z           [G1Size]byte   // com_Z
	Hx          [3][G1Size]byte // com_{H_X,0..2}
	ComQX       [G1Size]byte   // Shplonk X 轴商承诺
	ComVFAlpha  [G1Size]byte   // witness Y 轴承诺（α 处）
	ComVFZS     [G1Size]byte   // witness Y 轴承诺（ω·α 处）
	ComGY       [G1Size]byte   // G_Y 聚合承诺
	Pi1AggH     [G1Size]byte   // Y 轴开放商承诺

	// ── 证明标量求值（各 32 字节大端序）──────────────────────────────────────
	EvalA, EvalB, EvalO                        [FrSize]byte
	EvalZ, EvalZS                              [FrSize]byte
	EvalHx, EvalHy                             [FrSize]byte
	EvalQl, EvalQr, EvalQm, EvalQo, EvalQk    [FrSize]byte
	EvalS1, EvalS2, EvalS3                     [FrSize]byte

	// ── 公开输入（各 32 字节大端序）──────────────────────────────────────────
	// PublicInputs[i][j] = 第 i 个子电路第 j 个公开输入
	PublicInputs [][FrSize]byte
	// PublicInputsPerInstance 是每个子电路的公开输入数量（用于 Solidity 切片边界）。
	PublicInputsPerInstance int

	// ── 链下预计算的 G2 点（各 128 字节，EVM ecPairing 格式）────────────────
	// ZTG2 = [Z_T(τ_X)]₂ = G2[2] - (α+ωα)·G2[1] + α·ωα·G2[0]
	ZTG2 [G2Size]byte
	// TauYBetaG2 = [τ_Y - β]₂ = G2Y[1] - β·G2Y[0]
	TauYBetaG2 [G2Size]byte

	// ── 中间挑战（供调试 / Solidity FS 对齐使用）─────────────────────────────
	Challenges BPianoChallenges
}

// BPianoChallenges 保存 Fiat-Shamir 转录推导出的全部挑战值。
// Solidity 验证器需在链上重新派生这些值；此处用于测试对齐。
type BPianoChallenges struct {
	Gamma, Eta, Lambda, Alpha, Nu, Beta, Mu fr.Element
	AlphaShifted                            fr.Element // ω_X · α
	Rho                                     fr.Element // 链下哈希派生的随机挑战
}

// BPianoCalldataResult 包含 Solidity 调用所需的打包字节以及调试字段。
type BPianoCalldataResult struct {
	Calldata BPianoCalldata
	// ABI 打包：proof_g1s (12×64) || proof_scalars (15×32) || public_inputs (n×32)
	//         || zTG2 (128) || tauYBetaG2 (128)
	Packed []byte
}

// GenerateBPianoCalldata 从 BPiano 压缩证明生成 Solidity 验证合约所需的 calldata。
//
// 该函数：
//  1. 重放与 bpiano.VerifyCompressed 完全相同的 Fiat-Shamir 转录以得到 α、β；
//  2. 在 Go 侧计算 [Z_T(τ_X)]₂ 和 [τ_Y - β]₂（EVM 无 G2 标量乘预编译）；
//  3. 将所有字段序列化为 EVM 兼容格式并打包为 ABI calldata。
func GenerateBPianoCalldata(
	proof *bpiano.CompressedProof,
	vk *piano.VerifyingKey,
	publicInputs [][]fr.Element,
) (*BPianoCalldataResult, error) {
	T := vk.SizeX
	srs := vk.DKZGSRS

	// ── 1. 重放 Fiat-Shamir 挑战（与 bpiano/verify.go 完全一致）──────────────
	challenges, err := replayBPianoFS(proof, vk, publicInputs, T)
	if err != nil {
		return nil, fmt.Errorf("solgen: FS 重放失败：%w", err)
	}

	// ── 2. 计算预编译 G2 点 ──────────────────────────────────────────────────
	// ZTG2 = G2[2] - (α+ωα)·G2[1] + α·ωα·G2[0]
	zTG2 := computeZTG2(challenges.Alpha, challenges.AlphaShifted, srs)
	// TauYBetaG2 = G2Y[1] - β·G2Y[0]
	tauYBetaG2 := computeTauYMinusBetaG2(challenges.Beta, srs)

	// ── 3. 序列化证明字段 ─────────────────────────────────────────────────────
	cd := BPianoCalldata{}
	cd.Challenges = challenges

	for i := range proof.LRO {
		cd.LRO[i] = G1Bytes(proof.LRO[i])
	}
	cd.Z = G1Bytes(proof.Z)
	for i := range proof.Hx {
		cd.Hx[i] = G1Bytes(proof.Hx[i])
	}
	cd.ComQX = G1Bytes(proof.ComQX)
	cd.ComVFAlpha = G1Bytes(proof.ComVFAlpha)
	cd.ComVFZS = G1Bytes(proof.ComVFZS)
	cd.ComGY = G1Bytes(proof.ComGY)
	cd.Pi1AggH = G1Bytes(proof.Pi1AggH)

	cd.EvalA = FrBytes(proof.EvalA)
	cd.EvalB = FrBytes(proof.EvalB)
	cd.EvalO = FrBytes(proof.EvalO)
	cd.EvalZ = FrBytes(proof.EvalZ)
	cd.EvalZS = FrBytes(proof.EvalZS)
	cd.EvalHx = FrBytes(proof.EvalHx)
	cd.EvalHy = FrBytes(proof.EvalHy)
	cd.EvalQl = FrBytes(proof.EvalQl)
	cd.EvalQr = FrBytes(proof.EvalQr)
	cd.EvalQm = FrBytes(proof.EvalQm)
	cd.EvalQo = FrBytes(proof.EvalQo)
	cd.EvalQk = FrBytes(proof.EvalQk)
	cd.EvalS1 = FrBytes(proof.EvalS1)
	cd.EvalS2 = FrBytes(proof.EvalS2)
	cd.EvalS3 = FrBytes(proof.EvalS3)

	// 公开输入：展平为一维数组（每个子电路行按顺序追加）。
	if len(publicInputs) > 0 {
		cd.PublicInputsPerInstance = len(publicInputs[0])
		for _, row := range publicInputs {
			for _, v := range row {
				cd.PublicInputs = append(cd.PublicInputs, FrBytes(v))
			}
		}
	}

	cd.ZTG2 = G2Bytes(zTG2)
	cd.TauYBetaG2 = G2Bytes(tauYBetaG2)

	// ── 4. 打包为 ABI calldata ────────────────────────────────────────────────
	packed := packBPianoCalldata(&cd)

	return &BPianoCalldataResult{Calldata: cd, Packed: packed}, nil
}

// packBPianoCalldata 将 BPianoCalldata 线性打包为字节切片。
//
// 布局（无函数选择器前缀，适合 call(data) 直接使用）：
//
//	[0  .. 767]  12 个 G1 点（LRO[3], Z, Hx[3], ComQX, ComVFAlpha, ComVFZS, ComGY, Pi1AggH）
//	[768 ..1247] 15 个 Fr 标量求值
//	[1248..1247+n×32] n 个公开输入标量
//	[..  ..+128] ZTG2（G2 点）
//	[..  ..+128] TauYBetaG2（G2 点）
func packBPianoCalldata(cd *BPianoCalldata) []byte {
	g1Points := []([G1Size]byte){
		cd.LRO[0], cd.LRO[1], cd.LRO[2],
		cd.Z,
		cd.Hx[0], cd.Hx[1], cd.Hx[2],
		cd.ComQX, cd.ComVFAlpha, cd.ComVFZS,
		cd.ComGY, cd.Pi1AggH,
	}
	scalars := []([FrSize]byte){
		cd.EvalA, cd.EvalB, cd.EvalO,
		cd.EvalZ, cd.EvalZS,
		cd.EvalHx, cd.EvalHy,
		cd.EvalQl, cd.EvalQr, cd.EvalQm, cd.EvalQo, cd.EvalQk,
		cd.EvalS1, cd.EvalS2, cd.EvalS3,
	}

	nPI := len(cd.PublicInputs)
	total := len(g1Points)*G1Size + len(scalars)*FrSize + nPI*FrSize + 2*G2Size
	buf := make([]byte, 0, total)

	for _, p := range g1Points {
		buf = append(buf, p[:]...)
	}
	for _, s := range scalars {
		buf = append(buf, s[:]...)
	}
	for _, pi := range cd.PublicInputs {
		buf = append(buf, pi[:]...)
	}
	buf = append(buf, cd.ZTG2[:]...)
	buf = append(buf, cd.TauYBetaG2[:]...)

	return buf
}

// ────────────────────────────────────────────────────────────────────────────
// Fiat-Shamir 重放
// ────────────────────────────────────────────────────────────────────────────

// replayBPianoFS 重放与 bpiano.VerifyCompressed 完全相同的 FS 转录，
// 返回所有挑战值。
func replayBPianoFS(
	proof *bpiano.CompressedProof,
	vk *piano.VerifyingKey,
	publicInputs [][]fr.Element,
	T uint64,
) (BPianoChallenges, error) {
	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc,
		"gamma", "eta", "lambda", "alpha", "nu", "beta", "mu")

	// gamma：绑定 VK 选择子/置换承诺 + 公开输入 + LRO 承诺
	if err := bindVKAndPI(fs, "gamma", vk, publicInputs); err != nil {
		return BPianoChallenges{}, err
	}
	gamma, err := deriveChallenge(fs, "gamma",
		[]dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
	if err != nil {
		return BPianoChallenges{}, err
	}

	// eta：无额外绑定
	eta, err := deriveChallenge(fs, "eta", nil)
	if err != nil {
		return BPianoChallenges{}, err
	}

	// lambda：绑定 Z 承诺
	lambda, err := deriveChallenge(fs, "lambda", []dkzg.Digest{proof.Z})
	if err != nil {
		return BPianoChallenges{}, err
	}

	// alpha：绑定 Hx[0..2]
	alpha, err := deriveChallenge(fs, "alpha",
		[]dkzg.Digest{proof.Hx[0], proof.Hx[1], proof.Hx[2]})
	if err != nil {
		return BPianoChallenges{}, err
	}

	// 计算 foldedHxDig = Hx[0] + αᵀ·Hx[1] + α^{2T}·Hx[2]（G1 线性组合）
	alphaPowT := new(fr.Element).Exp(alpha, new(big.Int).SetUint64(T))
	foldedHxDig := foldDigestsG1(proof.Hx[0], proof.Hx[1], proof.Hx[2], alphaPowT)

	// nu：绑定 foldedHxDig, LRO[0..2], Z
	nu, err := deriveChallenge(fs, "nu",
		[]dkzg.Digest{foldedHxDig, proof.LRO[0], proof.LRO[1], proof.LRO[2], proof.Z})
	if err != nil {
		return BPianoChallenges{}, err
	}

	// beta：绑定 ComQX, ComVFAlpha, ComVFZS
	beta, err := deriveChallenge(fs, "beta",
		[]dkzg.Digest{proof.ComQX, proof.ComVFAlpha, proof.ComVFZS})
	if err != nil {
		return BPianoChallenges{}, err
	}

	// mu：无额外绑定
	mu, err := deriveChallenge(fs, "mu", nil)
	if err != nil {
		return BPianoChallenges{}, err
	}

	alphaShifted := new(fr.Element).Mul(&alpha, &vk.GeneratorX)

	// ρ：链下 SHA-256 哈希（非 FS 转录）
	rho := deriveRho(proof)

	return BPianoChallenges{
		Gamma:        gamma,
		Eta:          eta,
		Lambda:       lambda,
		Alpha:        alpha,
		Nu:           nu,
		Beta:         beta,
		Mu:           mu,
		AlphaShifted: *alphaShifted,
		Rho:          rho,
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// G2 预计算
// ────────────────────────────────────────────────────────────────────────────

// computeZTG2 计算 [Z_T(τ_X)]₂ = G2[2] - (α+ωα)·G2[1] + α·ωα·G2[0]。
// 此运算需要 G2 标量乘法，EVM 不支持，故必须链下完成。
func computeZTG2(alpha, alphaShifted fr.Element, srs *dkzg.SRS) bn254.G2Affine {
	var coeff1, coeff0 fr.Element
	coeff1.Add(&alpha, &alphaShifted)    // α + ωα
	coeff0.Mul(&alpha, &alphaShifted)    // α · ωα

	var result bn254.G2Jac
	result.FromAffine(&srs.G2[2])

	var tmp bn254.G2Jac
	var b big.Int

	// - (α+ωα)·G2[1]
	tmp.FromAffine(&srs.G2[1])
	coeff1.BigInt(&b)
	tmp.ScalarMultiplication(&tmp, &b)
	result.SubAssign(&tmp)

	// + α·ωα·G2[0]
	tmp.FromAffine(&srs.G2[0])
	coeff0.BigInt(&b)
	tmp.ScalarMultiplication(&tmp, &b)
	result.AddAssign(&tmp)

	var aff bn254.G2Affine
	aff.FromJacobian(&result)
	return aff
}

// computeTauYMinusBetaG2 计算 [τ_Y - β]₂ = G2Y[1] - β·G2Y[0]。
func computeTauYMinusBetaG2(beta fr.Element, srs *dkzg.SRS) bn254.G2Affine {
	var result bn254.G2Jac
	result.FromAffine(&srs.G2Y[1])

	var tmp bn254.G2Jac
	tmp.FromAffine(&srs.G2Y[0])
	var b big.Int
	beta.BigInt(&b)
	tmp.ScalarMultiplication(&tmp, &b)
	result.SubAssign(&tmp)

	var aff bn254.G2Affine
	aff.FromJacobian(&result)
	return aff
}

// ────────────────────────────────────────────────────────────────────────────
// 内部辅助
// ────────────────────────────────────────────────────────────────────────────

// bindVKAndPI 将 VK 中的选择子/置换承诺及公开输入绑定到 FS 转录（对应 bindPublicDataBP）。
func bindVKAndPI(fs *fiatshamir.Transcript, label string, vk *piano.VerifyingKey, publicInputs [][]fr.Element) error {
	for _, com := range []dkzg.Digest{vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3} {
		b := com.Bytes()
		if err := fs.Bind(label, b[:]); err != nil {
			return err
		}
	}
	for _, row := range publicInputs {
		for _, v := range row {
			b := v.Bytes()
			if err := fs.Bind(label, b[:]); err != nil {
				return err
			}
		}
	}
	return nil
}

// deriveChallenge 绑定一批 G1 承诺后从 FS 转录导出挑战（对应 deriveChallengeBP）。
func deriveChallenge(fs *fiatshamir.Transcript, label string, digests []dkzg.Digest) (fr.Element, error) {
	for _, d := range digests {
		b := d.Bytes()
		if err := fs.Bind(label, b[:]); err != nil {
			return fr.Element{}, err
		}
	}
	raw, err := fs.ComputeChallenge(label)
	if err != nil {
		return fr.Element{}, err
	}
	var ch fr.Element
	ch.SetBytes(raw)
	return ch, nil
}

// foldDigestsG1 计算 d0 + aPowN·d1 + aPowN²·d2（G1 标量乘 + 加），
// 对应 piano.FoldDigests（但此处直接调用以避免包循环依赖复杂性，逻辑完全一致）。
func foldDigestsG1(d0, d1, d2 dkzg.Digest, aPowN *fr.Element) dkzg.Digest {
	var aPow2 fr.Element
	aPow2.Mul(aPowN, aPowN)

	var b0, b1 big.Int
	aPowN.BigInt(&b0)
	aPow2.BigInt(&b1)

	var s1, s2 bn254.G1Affine
	s1.ScalarMultiplication(&d1, &b0)
	s2.ScalarMultiplication(&d2, &b1)

	var result bn254.G1Jac
	var d0Jac, s1Jac, s2Jac bn254.G1Jac
	d0Jac.FromAffine(&d0)
	s1Jac.FromAffine(&s1)
	s2Jac.FromAffine(&s2)
	result.Set(&d0Jac)
	result.AddAssign(&s1Jac)
	result.AddAssign(&s2Jac)

	var out dkzg.Digest
	out.FromJacobian(&result)
	return out
}

// deriveRho 对关键证明承诺和求值进行哈希，生成随机挑战 ρ（对应 bpiano.deriveRhoBP）。
func deriveRho(proof *bpiano.CompressedProof) fr.Element {
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
	var rho fr.Element
	rho.SetBytes(h.Sum(nil))
	return rho
}
