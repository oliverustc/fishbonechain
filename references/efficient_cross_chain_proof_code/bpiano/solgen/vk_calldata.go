package solgen

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/piano"
)

// VKSolidity 包含将 VerifyingKey 部署到 BPianoVerifier 合约所需的全部参数。
type VKSolidity struct {
	// G1 承诺（各 64 字节）
	Ql, Qr, Qm, Qo, Qk [G1Size]byte
	S1, S2, S3          [G1Size]byte
	// G2 点（各 128 字节）
	G2_0  [G2Size]byte // srs.G2[0]
	G2_1  [G2Size]byte // srs.G2[1]
	G2Y_0 [G2Size]byte // srs.G2Y[0]
	// 域参数（32 字节大端序）
	SizeX      [32]byte // T
	SizeY      [32]byte // M
	GeneratorX [FrSize]byte
	CosetShift [FrSize]byte
	NbPublicInputs uint64
}

// ExtractVKSolidity 从 piano.VerifyingKey 中提取 Solidity 合约所需的 VK 字段。
func ExtractVKSolidity(vk *piano.VerifyingKey) *VKSolidity {
	out := &VKSolidity{}

	out.Ql = G1Bytes(vk.Ql)
	out.Qr = G1Bytes(vk.Qr)
	out.Qm = G1Bytes(vk.Qm)
	out.Qo = G1Bytes(vk.Qo)
	out.Qk = G1Bytes(vk.Qk)
	out.S1 = G1Bytes(vk.S1)
	out.S2 = G1Bytes(vk.S2)
	out.S3 = G1Bytes(vk.S3)

	out.G2_0  = G2Bytes(vk.DKZGSRS.G2[0])
	out.G2_1  = G2Bytes(vk.DKZGSRS.G2[1])
	out.G2Y_0 = G2Bytes(vk.DKZGSRS.G2Y[0])

	out.SizeX = Uint256Bytes(vk.SizeX)
	out.SizeY = Uint256Bytes(vk.SizeY)

	out.GeneratorX = FrBytes(vk.GeneratorX)
	out.CosetShift = FrBytes(vk.CosetShift)

	out.NbPublicInputs = uint64(vk.NbPublicInputs)
	return out
}

// G1ToSolidity 将 G1 点的 64 字节拆分为 (x, y) 两个 *big.Int，
// 对应 Solidity struct G1Point { uint256 x; uint256 y; }。
func G1ToSolidity(b [G1Size]byte) (x, y *big.Int) {
	x = new(big.Int).SetBytes(b[0:32])
	y = new(big.Int).SetBytes(b[32:64])
	return
}

// G2ToSolidity 将 G2 点的 128 字节拆分为 (xIm, xRe, yIm, yRe) 四个 *big.Int，
// 对应 Solidity struct G2Point { uint256 xIm; uint256 xRe; uint256 yIm; uint256 yRe; }。
func G2ToSolidity(b [G2Size]byte) (xIm, xRe, yIm, yRe *big.Int) {
	xIm = new(big.Int).SetBytes(b[0:32])
	xRe = new(big.Int).SetBytes(b[32:64])
	yIm = new(big.Int).SetBytes(b[64:96])
	yRe = new(big.Int).SetBytes(b[96:128])
	return
}

// FrToSolidity 将 Fr 字节转为 *big.Int（Solidity uint256）。
func FrToSolidity(b [FrSize]byte) *big.Int {
	return new(big.Int).SetBytes(b[:])
}

// FrElementToSolidity 将 fr.Element 转为 *big.Int。
func FrElementToSolidity(e fr.Element) *big.Int {
	b := FrBytes(e)
	return FrToSolidity(b)
}

// G1AffineToSolidity 将 bn254.G1Affine 转为 (x, y) *big.Int。
func G1AffineToSolidity(p bn254.G1Affine) (x, y *big.Int) {
	b := G1Bytes(p)
	return G1ToSolidity(b)
}

// G2AffineToSolidity 将 bn254.G2Affine 转为 (xIm, xRe, yIm, yRe) *big.Int。
func G2AffineToSolidity(p bn254.G2Affine) (xIm, xRe, yIm, yRe *big.Int) {
	b := G2Bytes(p)
	return G2ToSolidity(b)
}

// ────────────────────────────────────────────────────────────────────────────
// Solidity 构造参数生成
// ────────────────────────────────────────────────────────────────────────────

// SolidityVKArgs 将 VK 格式化为可直接嵌入 Foundry 测试或 cast 调用的 Solidity 元组字符串。
// 格式：(G1Point, G1Point, ..., G2Point, ..., uint256, ...)
func SolidityVKArgs(vk *piano.VerifyingKey) string {
	vs := ExtractVKSolidity(vk)
	var sb strings.Builder
	sb.WriteString("(")

	// 8 个 G1 承诺
	for i, b := range [][G1Size]byte{vs.Ql, vs.Qr, vs.Qm, vs.Qo, vs.Qk, vs.S1, vs.S2, vs.S3} {
		x, y := G1ToSolidity(b)
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("(%s,%s)", x.String(), y.String()))
	}
	sb.WriteString(",")

	// 3 个 G2 点
	for i, b := range [][G2Size]byte{vs.G2_0, vs.G2_1, vs.G2Y_0} {
		xIm, xRe, yIm, yRe := G2ToSolidity(b)
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("(%s,%s,%s,%s)", xIm.String(), xRe.String(), yIm.String(), yRe.String()))
	}
	sb.WriteString(",")

	// 域参数
	sb.WriteString(new(big.Int).SetBytes(vs.SizeX[:]).String())
	sb.WriteString(",")
	sb.WriteString(new(big.Int).SetBytes(vs.SizeY[:]).String())
	sb.WriteString(",")
	sb.WriteString(FrToSolidity(vs.GeneratorX).String())
	sb.WriteString(",")
	sb.WriteString(FrToSolidity(vs.CosetShift).String())
	sb.WriteString(",")
	sb.WriteString(fmt.Sprintf("%d", vs.NbPublicInputs))

	sb.WriteString(")")
	return sb.String()
}

// proofG1Hex 将 G1 点编码为 64 字节十六进制（小写，无 0x 前缀）。
func proofG1Hex(b [G1Size]byte) string {
	return hex.EncodeToString(b[:])
}
