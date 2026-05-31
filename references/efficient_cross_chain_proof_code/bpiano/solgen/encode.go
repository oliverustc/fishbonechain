// Package solgen 提供将 BN254 曲线上的密码学对象序列化为
// EVM Solidity 合约所需格式的工具函数。
//
// EVM BN254 预编译合约（EIP-196/197）使用的字节格式：
//   - G1 点：64 字节，x || y，各 32 字节大端序
//   - G2 点：128 字节，x.imag || x.real || y.imag || y.real，各 32 字节大端序
//   - Fr 元素：32 字节大端序
package solgen

import (
	"encoding/binary"
	"math/big"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

const (
	// G1Size 是 EVM G1 仿射点的字节长度（非压缩格式）。
	G1Size = 64
	// G2Size 是 EVM G2 仿射点的字节长度（非压缩格式）。
	G2Size = 128
	// FrSize 是 BN254 标量域元素的字节长度。
	FrSize = 32
)

// G1Bytes 将 G1Affine 点转换为 EVM ecMul/ecAdd/ecPairing 预编译所需的
// 64 字节非压缩格式：x（32 字节大端序）|| y（32 字节大端序）。
//
// 无穷远点编码为全零 64 字节。
func G1Bytes(p bn254.G1Affine) [G1Size]byte {
	raw := p.RawBytes() // gnark-crypto 返回 [64]byte：[0:32]=X, [32:64]=Y
	// mUncompressed = 0x00，不影响坐标值；直接返回即可。
	// 无穷远点时 RawBytes 设置 res[0]=mUncompressed=0，实为全零，符合 EVM 约定。
	return raw
}

// G2Bytes 将 G2Affine 点转换为 EVM ecPairing 预编译所需的
// 128 字节非压缩格式：
//
//	x.A1（虚部，32 字节）|| x.A0（实部，32 字节）|| y.A1（虚部，32 字节）|| y.A0（实部，32 字节）
//
// gnark-crypto RawBytes() 的存储顺序与 EVM EIP-197 一致，
// 故直接使用（mUncompressed = 0x00，不影响数据位）。
func G2Bytes(p bn254.G2Affine) [G2Size]byte {
	raw := p.RawBytes() // [128]byte：[0:32]=X.A1, [32:64]=X.A0, [64:96]=Y.A1, [96:128]=Y.A0
	return raw
}

// FrBytes 将 fr.Element 转换为 32 字节大端序表示，用于 Solidity 合约的标量参数。
func FrBytes(e fr.Element) [FrSize]byte {
	return e.Bytes() // gnark-crypto 已返回大端序 [32]byte
}

// FrFromBig 将 *big.Int 编码为 32 字节大端序（截断到 32 字节）。
// 输入须在 [0, p) 范围内，否则结果无意义。
func FrFromBig(b *big.Int) [FrSize]byte {
	var res [FrSize]byte
	bBytes := b.Bytes()
	if len(bBytes) > FrSize {
		bBytes = bBytes[len(bBytes)-FrSize:]
	}
	copy(res[FrSize-len(bBytes):], bBytes)
	return res
}

// Uint256Bytes 将 uint64 编码为 32 字节大端序（ABI uint256）。
func Uint256Bytes(v uint64) [32]byte {
	var res [32]byte
	binary.BigEndian.PutUint64(res[24:], v)
	return res
}

// EncodeG1s 将多个 G1 点顺序写入字节切片。
func EncodeG1s(points []bn254.G1Affine) []byte {
	out := make([]byte, len(points)*G1Size)
	for i, p := range points {
		b := G1Bytes(p)
		copy(out[i*G1Size:], b[:])
	}
	return out
}

// EncodeG2s 将多个 G2 点顺序写入字节切片。
func EncodeG2s(points []bn254.G2Affine) []byte {
	out := make([]byte, len(points)*G2Size)
	for i, p := range points {
		b := G2Bytes(p)
		copy(out[i*G2Size:], b[:])
	}
	return out
}

// EncodeFrs 将多个域元素顺序写入字节切片。
func EncodeFrs(elems []fr.Element) []byte {
	out := make([]byte, len(elems)*FrSize)
	for i, e := range elems {
		b := FrBytes(e)
		copy(out[i*FrSize:], b[:])
	}
	return out
}
