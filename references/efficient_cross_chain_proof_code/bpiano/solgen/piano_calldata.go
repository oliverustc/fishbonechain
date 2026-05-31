package solgen

import (
	"crypto/sha256"
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/piano"
)

// PianoCalldata 包含 Piano Solidity 验证合约所需的全部输入。
type PianoCalldata struct {
	// ── 证明 G1 承诺（各 64 字节，EVM 非压缩格式）
	LRO         [3][G1Size]byte  // com_A, com_B, com_O
	Z           [G1Size]byte     // com_Z
	Hx          [3][G1Size]byte  // com_{H_X,0..2}
	Hy          [3][G1Size]byte  // com_{H_Y,0..2}
	BatchXH     [G1Size]byte     // BatchedProofX.H
	ClaimedDigs [13][G1Size]byte // BatchedProofX.ClaimedDigests
	ZsH         [G1Size]byte     // ZShiftedProofX.H
	ZsComVF     [G1Size]byte     // ZShiftedProofX.ComVF
	BatchYH     [G1Size]byte     // BatchedProofY.H

	// ── 证明标量求值（各 32 字节大端序）
	EvalA, EvalB, EvalO                     [FrSize]byte
	EvalZ, EvalZS                           [FrSize]byte
	EvalHx, EvalHy                          [FrSize]byte
	EvalQl, EvalQr, EvalQm, EvalQo, EvalQk [FrSize]byte
	EvalS1, EvalS2, EvalS3                  [FrSize]byte
	BatchYVals                              [15][FrSize]byte // BatchedProofY.ClaimedValues

	// ── 公开输入（各 32 字节大端序）
	PublicInputs []  [FrSize]byte

	// ── 链下预计算的 G2 点（128 字节）
	// TauYBetaG2 = G2Y[1] - β·G2Y[0]
	TauYBetaG2 [G2Size]byte

	// ── 中间挑战（调试用）
	Challenges PianoChallenges
}

// PianoChallenges 保存 Fiat-Shamir 转录推导出的全部挑战值。
type PianoChallenges struct {
	Gamma, Eta, Lambda, Alpha, Beta fr.Element
	AlphaShifted                    fr.Element // ω_X · α
}

// PianoCalldataResult 包含 Solidity 调用所需的数据以及调试字段。
type PianoCalldataResult struct {
	Calldata *PianoCalldata
	VK       *VKSolidity // reuse from vk_calldata.go
}

// GeneratePianoCalldata 从 Piano 证明生成 Solidity 验证合约所需的 calldata。
//
// 该函数：
//  1. 重放与 piano.Verify 完全相同的 Fiat-Shamir 转录以得到 α、β；
//  2. 在 Go 侧计算 [τ_Y - β]₂（EVM 无 G2 标量乘预编译）；
//  3. 将所有字段序列化为 EVM 兼容格式。
func GeneratePianoCalldata(
	proof *piano.Proof,
	vk *piano.VerifyingKey,
	publicInputs [][]fr.Element,
) (*PianoCalldataResult, error) {
	srs := vk.DKZGSRS

	// ── 1. 重放 Fiat-Shamir 挑战（与 piano.Verify 完全一致）──────────────────
	challenges, err := replayPianoFS(proof, vk, publicInputs)
	if err != nil {
		return nil, fmt.Errorf("solgen: Piano FS 重放失败：%w", err)
	}

	// ── 2. 计算预编译 G2 点 ────────────────────────────────────────────────
	// TauYBetaG2 = G2Y[1] - β·G2Y[0]
	tauYBetaG2 := computeTauYMinusBetaG2(challenges.Beta, srs)

	// ── 3. 序列化证明字段 ─────────────────────────────────────────────────────
	cd := &PianoCalldata{}
	cd.Challenges = challenges

	for i := range proof.LRO {
		cd.LRO[i] = G1Bytes(proof.LRO[i])
	}
	cd.Z = G1Bytes(proof.Z)
	for i := range proof.Hx {
		cd.Hx[i] = G1Bytes(proof.Hx[i])
	}
	for i := range proof.Hy {
		cd.Hy[i] = G1Bytes(proof.Hy[i])
	}
	cd.BatchXH = G1Bytes(proof.BatchedProofX.H)
	for i := range proof.BatchedProofX.ClaimedDigests {
		cd.ClaimedDigs[i] = G1Bytes(proof.BatchedProofX.ClaimedDigests[i])
	}
	cd.ZsH = G1Bytes(proof.ZShiftedProofX.H)
	cd.ZsComVF = G1Bytes(proof.ZShiftedProofX.ComVF)
	cd.BatchYH = G1Bytes(proof.BatchedProofY.H)

	// Fr 标量求值
	cd.EvalA = FrBytes(proof.ClaimedA)
	cd.EvalB = FrBytes(proof.ClaimedB)
	cd.EvalO = FrBytes(proof.ClaimedO)
	cd.EvalZ = FrBytes(proof.ClaimedZ)
	cd.EvalZS = FrBytes(proof.ClaimedZS)
	cd.EvalHx = FrBytes(proof.ClaimedHx)
	cd.EvalHy = FrBytes(proof.ClaimedHy)
	cd.EvalQl = FrBytes(proof.ClaimedQl)
	cd.EvalQr = FrBytes(proof.ClaimedQr)
	cd.EvalQm = FrBytes(proof.ClaimedQm)
	cd.EvalQo = FrBytes(proof.ClaimedQo)
	cd.EvalQk = FrBytes(proof.ClaimedQk)
	cd.EvalS1 = FrBytes(proof.ClaimedS1)
	cd.EvalS2 = FrBytes(proof.ClaimedS2)
	cd.EvalS3 = FrBytes(proof.ClaimedS3)

	// BatchedProofY.ClaimedValues[15]
	for i := 0; i < 15; i++ {
		cd.BatchYVals[i] = FrBytes(proof.BatchedProofY.ClaimedValues[i])
	}

	// 公开输入：展平
	if len(publicInputs) > 0 {
		for _, row := range publicInputs {
			for _, v := range row {
				cd.PublicInputs = append(cd.PublicInputs, FrBytes(v))
			}
		}
	}

	cd.TauYBetaG2 = G2Bytes(tauYBetaG2)

	return &PianoCalldataResult{
		Calldata: cd,
		VK:       ExtractVKSolidity(vk),
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Fiat-Shamir 重放
// ────────────────────────────────────────────────────────────────────────────

// replayPianoFS 重放与 piano.Verify 完全相同的 FS 转录，返回所有挑战值。
func replayPianoFS(
	proof *piano.Proof,
	vk *piano.VerifyingKey,
	publicInputs [][]fr.Element,
) (PianoChallenges, error) {
	hFunc := sha256.New()
	fs := fiatshamir.NewTranscript(hFunc, "gamma", "eta", "lambda", "alpha", "beta")

	// gamma：绑定 VK 选择子/置换承诺 + 公开输入 + LRO 承诺
	if err := bindVKAndPI(fs, "gamma", vk, publicInputs); err != nil {
		return PianoChallenges{}, err
	}
	gamma, err := deriveChallenge(fs, "gamma",
		[]dkzg.Digest{proof.LRO[0], proof.LRO[1], proof.LRO[2]})
	if err != nil {
		return PianoChallenges{}, err
	}

	// eta：无额外绑定
	eta, err := deriveChallenge(fs, "eta", nil)
	if err != nil {
		return PianoChallenges{}, err
	}

	// lambda：绑定 Z 承诺
	lambda, err := deriveChallenge(fs, "lambda", []dkzg.Digest{proof.Z})
	if err != nil {
		return PianoChallenges{}, err
	}

	// alpha：绑定 Hx[0..2]
	alpha, err := deriveChallenge(fs, "alpha",
		[]dkzg.Digest{proof.Hx[0], proof.Hx[1], proof.Hx[2]})
	if err != nil {
		return PianoChallenges{}, err
	}

	// beta：绑定 BatchedProofX.H + ClaimedDigests[0..12] + Hy[0..2]
	betaDigests := make([]dkzg.Digest, 0, 1+13+3)
	betaDigests = append(betaDigests, proof.BatchedProofX.H)
	for k := range proof.BatchedProofX.ClaimedDigests {
		betaDigests = append(betaDigests, proof.BatchedProofX.ClaimedDigests[k])
	}
	betaDigests = append(betaDigests, proof.Hy[0], proof.Hy[1], proof.Hy[2])
	beta, err := deriveChallenge(fs, "beta", betaDigests)
	if err != nil {
		return PianoChallenges{}, err
	}

	var alphaShifted fr.Element
	alphaShifted.Mul(&alpha, &vk.GeneratorX)

	return PianoChallenges{
		Gamma:        gamma,
		Eta:          eta,
		Lambda:       lambda,
		Alpha:        alpha,
		Beta:         beta,
		AlphaShifted: alphaShifted,
	}, nil
}

