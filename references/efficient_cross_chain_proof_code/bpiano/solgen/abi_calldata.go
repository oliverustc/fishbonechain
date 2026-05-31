package solgen

// abi_calldata.go 将 Piano / BPiano 证明编码为 Solidity verify() 的 ABI calldata。
//
// 编码格式：
//   - 4 字节函数选择器
//   - ABI 编码参数（静态 struct 内联，动态 uint256[] 带 offset + length + data）
//
// 不依赖 go-ethereum，使用手动 ABI spec 编码。

import (
	"encoding/binary"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/piano"
)

// BPiano verify(CompressedProof,G2Point,G2Point,uint256[]) 函数选择器。
// keccak256("verify(...)") 前 4 字节，由 forge inspect 计算得到。
var bpianoVerifySelector = [4]byte{0xd3, 0x69, 0x1e, 0xba}

// Piano verify(PianoProof,G2Point,uint256[]) 函数选择器。
var pianoVerifySelector = [4]byte{0x7b, 0xe4, 0x50, 0x49}

// EncodeBPianoVerifyCalldata 将 BPiano 压缩证明编码为 verify() 的 ABI calldata。
//
// 返回：函数选择器（4B）+ ABI 编码参数，可直接用于 eth_sendTransaction.data。
//
// 签名：verify(CompressedProof calldata proof,
//
//	Pairing.G2Point calldata zTG2,
//	Pairing.G2Point calldata tauYBetaG2,
//	uint256[] calldata publicInputsFlat)
//
// 内部先调用 GenerateBPianoCalldata 重放 FS 挑战并预计算 G2 点。
func EncodeBPianoVerifyCalldata(
	proof *bpiano.CompressedProof,
	vk *piano.VerifyingKey,
	publicInputs [][]fr.Element,
) ([]byte, error) {
	res, err := GenerateBPianoCalldata(proof, vk, publicInputs)
	if err != nil {
		return nil, err
	}
	cd := &res.Calldata

	// ABI 布局（相对于选择器之后，即参数编码起始位置）：
	//   [0    .. 1247] CompressedProof（12 G1 × 64B + 15 Fr × 32B = 1248B）
	//   [1248 .. 1375] zTG2（G2Point = 4 × 32B = 128B）
	//   [1376 .. 1503] tauYBetaG2（128B）
	//   [1504 .. 1535] offset for publicInputsFlat（= 1536，32B）
	//   [1536 .. 1567] length of publicInputsFlat（32B）
	//   [1568 ..     ] publicInputsFlat 元素（每个 32B）
	const headSize = 1248 + 128 + 128 + 32 // = 1536

	nPI := len(cd.PublicInputs)
	buf := make([]byte, 0, 4+headSize+32+nPI*32)

	// 函数选择器
	buf = append(buf, bpianoVerifySelector[:]...)

	// CompressedProof：12 G1 点
	for _, p := range [...][G1Size]byte{
		cd.LRO[0], cd.LRO[1], cd.LRO[2],
		cd.Z,
		cd.Hx[0], cd.Hx[1], cd.Hx[2],
		cd.ComQX, cd.ComVFAlpha, cd.ComVFZS,
		cd.ComGY, cd.Pi1AggH,
	} {
		buf = append(buf, p[:]...)
	}
	// CompressedProof：15 Fr 标量求值
	for _, s := range [...][FrSize]byte{
		cd.EvalA, cd.EvalB, cd.EvalO,
		cd.EvalZ, cd.EvalZS,
		cd.EvalHx, cd.EvalHy,
		cd.EvalQl, cd.EvalQr, cd.EvalQm, cd.EvalQo, cd.EvalQk,
		cd.EvalS1, cd.EvalS2, cd.EvalS3,
	} {
		buf = append(buf, s[:]...)
	}

	// G2Point zTG2（xIm||xRe||yIm||yRe，各 32B）
	buf = append(buf, cd.ZTG2[:]...)

	// G2Point tauYBetaG2
	buf = append(buf, cd.TauYBetaG2[:]...)

	// offset for uint256[]（从参数编码起始位置计算）
	buf = append(buf, uint256BE(headSize)...)

	// 动态数组 tail：length + 元素
	buf = append(buf, uint256BE(nPI)...)
	for _, pi := range cd.PublicInputs {
		buf = append(buf, pi[:]...)
	}

	return buf, nil
}

// EncodePianoVerifyCalldata 将 Piano 证明编码为 verify() 的 ABI calldata。
//
// 返回：函数选择器（4B）+ ABI 编码参数。
//
// 签名：verify(PianoProof calldata proof,
//
//	Pairing.G2Point calldata tauYBetaG2,
//	uint256[] calldata publicInputsFlat)
func EncodePianoVerifyCalldata(
	proof *piano.Proof,
	vk *piano.VerifyingKey,
	publicInputs [][]fr.Element,
) ([]byte, error) {
	res, err := GeneratePianoCalldata(proof, vk, publicInputs)
	if err != nil {
		return nil, err
	}
	cd := res.Calldata

	// ABI 布局：
	//   [0    .. 2687] PianoProof（27 G1 × 64B + 30 Fr × 32B = 1728 + 960 = 2688B）
	//   [2688 .. 2815] tauYBetaG2（128B）
	//   [2816 .. 2847] offset for publicInputsFlat（= 2848，32B）
	//   [2848 .. 2879] length（32B）
	//   [2880 ..     ] 元素（每个 32B）
	const headSize = 2688 + 128 + 32 // = 2848

	nPI := len(cd.PublicInputs)
	buf := make([]byte, 0, 4+headSize+32+nPI*32)

	// 函数选择器
	buf = append(buf, pianoVerifySelector[:]...)

	// PianoProof：27 G1 点（顺序与 Solidity struct 字段一致）
	g1Fields := [...][G1Size]byte{
		cd.LRO[0], cd.LRO[1], cd.LRO[2],
		cd.Z,
		cd.Hx[0], cd.Hx[1], cd.Hx[2],
		cd.Hy[0], cd.Hy[1], cd.Hy[2],
		cd.BatchXH,
		cd.ClaimedDigs[0], cd.ClaimedDigs[1], cd.ClaimedDigs[2],
		cd.ClaimedDigs[3], cd.ClaimedDigs[4], cd.ClaimedDigs[5],
		cd.ClaimedDigs[6], cd.ClaimedDigs[7], cd.ClaimedDigs[8],
		cd.ClaimedDigs[9], cd.ClaimedDigs[10], cd.ClaimedDigs[11],
		cd.ClaimedDigs[12],
		cd.ZsH, cd.ZsComVF,
		cd.BatchYH,
	}
	for _, p := range g1Fields {
		buf = append(buf, p[:]...)
	}

	// PianoProof：15 Fr 求值 + 15 BatchYVals
	for _, s := range [...][FrSize]byte{
		cd.EvalA, cd.EvalB, cd.EvalO,
		cd.EvalZ, cd.EvalZS,
		cd.EvalHx, cd.EvalHy,
		cd.EvalQl, cd.EvalQr, cd.EvalQm, cd.EvalQo, cd.EvalQk,
		cd.EvalS1, cd.EvalS2, cd.EvalS3,
	} {
		buf = append(buf, s[:]...)
	}
	for _, s := range cd.BatchYVals {
		buf = append(buf, s[:]...)
	}

	// G2Point tauYBetaG2
	buf = append(buf, cd.TauYBetaG2[:]...)

	// offset for uint256[]
	buf = append(buf, uint256BE(headSize)...)

	// 动态数组 tail：length + 元素
	buf = append(buf, uint256BE(nPI)...)
	for _, pi := range cd.PublicInputs {
		buf = append(buf, pi[:]...)
	}

	return buf, nil
}

// uint256BE 将整数编码为 32 字节大端序（ABI uint256）。
func uint256BE(v int) []byte {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], uint64(v))
	return b[:]
}
