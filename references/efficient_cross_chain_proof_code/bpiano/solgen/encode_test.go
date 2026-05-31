package solgen_test

import (
	"math/big"
	"testing"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/solgen"
)

// TestG1BytesGenerator 验证 G1 生成元的 EVM 编码。
// BN254 G1 生成元的坐标（十进制）：
//
//	x = 1
//	y = 2
func TestG1BytesGenerator(t *testing.T) {
	_, _, g1, _ := bn254.Generators()

	b := solgen.G1Bytes(g1)
	if len(b) != solgen.G1Size {
		t.Fatalf("G1Bytes 长度 %d，期望 %d", len(b), solgen.G1Size)
	}

	// 验证 x = 1：前 32 字节应为 0x00...01
	var xBig big.Int
	xBig.SetBytes(b[0:32])
	if xBig.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("G1 生成元 x = %s，期望 1", xBig.String())
	}

	// 验证 y = 2：后 32 字节应为 0x00...02
	var yBig big.Int
	yBig.SetBytes(b[32:64])
	if yBig.Cmp(big.NewInt(2)) != 0 {
		t.Errorf("G1 生成元 y = %s，期望 2", yBig.String())
	}
}

// TestG2BytesGenerator 验证 G2 生成元的 EVM 编码格式。
// 验证内容：前 64 字节为 X（A1||A0），后 64 字节为 Y（A1||A0）。
func TestG2BytesGenerator(t *testing.T) {
	_, _, _, g2 := bn254.Generators()

	b := solgen.G2Bytes(g2)
	if len(b) != solgen.G2Size {
		t.Fatalf("G2Bytes 长度 %d，期望 %d", len(b), solgen.G2Size)
	}

	// 从字节中恢复 Fp 坐标并与原始值对比。
	var xA1, xA0, yA1, yA0 fp.Element
	if err := xA1.SetBytesCanonical(b[0:32]); err != nil {
		t.Fatalf("xA1.SetBytesCanonical: %v", err)
	}
	if err := xA0.SetBytesCanonical(b[32:64]); err != nil {
		t.Fatalf("xA0.SetBytesCanonical: %v", err)
	}
	if err := yA1.SetBytesCanonical(b[64:96]); err != nil {
		t.Fatalf("yA1.SetBytesCanonical: %v", err)
	}
	if err := yA0.SetBytesCanonical(b[96:128]); err != nil {
		t.Fatalf("yA0.SetBytesCanonical: %v", err)
	}

	if !xA1.Equal(&g2.X.A1) {
		t.Errorf("G2.X.A1 编码不匹配")
	}
	if !xA0.Equal(&g2.X.A0) {
		t.Errorf("G2.X.A0 编码不匹配")
	}
	if !yA1.Equal(&g2.Y.A1) {
		t.Errorf("G2.Y.A1 编码不匹配")
	}
	if !yA0.Equal(&g2.Y.A0) {
		t.Errorf("G2.Y.A0 编码不匹配")
	}
}

// TestFrBytes 验证 Fr 元素的 32 字节大端序编码。
func TestFrBytes(t *testing.T) {
	var e fr.Element
	e.SetUint64(12345)

	b := solgen.FrBytes(e)
	if len(b) != solgen.FrSize {
		t.Fatalf("FrBytes 长度 %d，期望 %d", len(b), solgen.FrSize)
	}

	var got big.Int
	got.SetBytes(b[:])
	if got.Cmp(big.NewInt(12345)) != 0 {
		t.Errorf("FrBytes(%d) = %s，期望 12345", 12345, got.String())
	}
}

// TestG1BytesInfinity 验证无穷远点编码为全零。
func TestG1BytesInfinity(t *testing.T) {
	var inf bn254.G1Affine // 零值 = 无穷远点
	b := solgen.G1Bytes(inf)
	for i, v := range b {
		if v != 0 {
			t.Errorf("无穷远点字节[%d] = %d，期望 0", i, v)
			break
		}
	}
}

// TestEncodeG1s 验证批量 G1 编码的总字节数。
func TestEncodeG1s(t *testing.T) {
	_, _, g1, _ := bn254.Generators()

	points := []bn254.G1Affine{g1, g1, g1}
	out := solgen.EncodeG1s(points)
	if len(out) != 3*solgen.G1Size {
		t.Errorf("EncodeG1s 字节数 %d，期望 %d", len(out), 3*solgen.G1Size)
	}
}

// TestUint256Bytes 验证 uint64 的 32 字节 ABI 编码。
func TestUint256Bytes(t *testing.T) {
	b := solgen.Uint256Bytes(256)
	var got big.Int
	got.SetBytes(b[:])
	if got.Cmp(big.NewInt(256)) != 0 {
		t.Errorf("Uint256Bytes(256) = %s，期望 256", got.String())
	}
}
