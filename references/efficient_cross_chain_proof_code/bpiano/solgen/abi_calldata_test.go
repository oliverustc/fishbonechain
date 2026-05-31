package solgen_test

import (
	"encoding/hex"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/piano"
	"github.com/oliverustc/bpiano/solgen"
)

// TestEncodeBPianoVerifyCalldata 验证 BPiano ABI calldata 编码的结构正确性并执行端到端 Forge 测试。
func TestEncodeBPianoVerifyCalldata(t *testing.T) {
	// 构建电路并生成证明
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)
	ci, witnesses := buildTestCircuit(T, M)
	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}

	// 编码 ABI calldata
	calldata, err := solgen.EncodeBPianoVerifyCalldata(proof, vk, nil)
	if err != nil {
		t.Fatalf("EncodeBPianoVerifyCalldata: %v", err)
	}

	// 检查函数选择器
	if len(calldata) < 4 {
		t.Fatal("calldata 太短")
	}
	wantSelector := [4]byte{0xd3, 0x69, 0x1e, 0xba}
	gotSelector := [4]byte(calldata[:4])
	if gotSelector != wantSelector {
		t.Errorf("选择器 got=%x want=%x", gotSelector, wantSelector)
	}

	// 检查长度：4 + 1248 + 128 + 128 + 32 + 32 + 0*32（无公开输入）
	// = 4 + 1568 = 1572（nPI=0 时）
	wantLen := 4 + 1248 + 128 + 128 + 32 + 32
	if len(calldata) != wantLen {
		t.Errorf("calldata 长度 got=%d want=%d", len(calldata), wantLen)
	}

	t.Logf("BPiano calldata 长度: %d bytes（无公开输入）", len(calldata))
	t.Logf("前 8 字节: %s", hex.EncodeToString(calldata[:8]))

	// 写入十六进制文件，供 Forge 测试使用
	solDir := findSolDir(t)
	hexPath := filepath.Join(solDir, "test", "bpiano_calldata.hex")
	if err := os.WriteFile(hexPath, []byte(hex.EncodeToString(calldata)), 0644); err != nil {
		t.Fatalf("写入 calldata hex 文件: %v", err)
	}
	t.Logf("BPiano calldata 已写入 %s", hexPath)

	// 运行 Forge 端到端测试（利用已有的 fixture.json 间接验证）
	cmd := exec.Command("forge", "test", "--match-contract", "BPianoVerifierTest", "-vv")
	cmd.Dir = solDir
	out, runErr := cmd.CombinedOutput()
	t.Logf("forge test:\n%s", string(out))
	if runErr != nil {
		t.Fatalf("forge test 失败: %v", runErr)
	}
}

// TestEncodePianoVerifyCalldata 验证 Piano ABI calldata 编码的结构正确性并执行端到端 Forge 测试。
func TestEncodePianoVerifyCalldata(t *testing.T) {
	// 构建电路并生成证明
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)
	ci, witnesses := buildTestCircuit(T, M)
	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	proof, err := piano.Prove(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	// 编码 ABI calldata
	calldata, err := solgen.EncodePianoVerifyCalldata(proof, vk, nil)
	if err != nil {
		t.Fatalf("EncodePianoVerifyCalldata: %v", err)
	}

	// 检查函数选择器
	wantSelector := [4]byte{0x7b, 0xe4, 0x50, 0x49}
	gotSelector := [4]byte(calldata[:4])
	if gotSelector != wantSelector {
		t.Errorf("选择器 got=%x want=%x", gotSelector, wantSelector)
	}

	// 检查长度：4 + 2688 + 128 + 32 + 32 + 0*32（无公开输入）
	// = 4 + 2880 = 2884
	wantLen := 4 + 2688 + 128 + 32 + 32
	if len(calldata) != wantLen {
		t.Errorf("calldata 长度 got=%d want=%d", len(calldata), wantLen)
	}

	t.Logf("Piano calldata 长度: %d bytes（无公开输入）", len(calldata))
	t.Logf("前 8 字节: %s", hex.EncodeToString(calldata[:8]))

	// 写入十六进制文件，供 Forge 测试使用
	solDir := findSolDir(t)
	hexPath := filepath.Join(solDir, "test", "piano_calldata.hex")
	if err := os.WriteFile(hexPath, []byte(hex.EncodeToString(calldata)), 0644); err != nil {
		t.Fatalf("写入 calldata hex 文件: %v", err)
	}
	t.Logf("Piano calldata 已写入 %s", hexPath)

	// 运行 Forge 端到端测试（利用已有的 fixture_piano.json 间接验证）
	cmd := exec.Command("forge", "test", "--match-contract", "PianoVerifierTest", "-vv")
	cmd.Dir = solDir
	out, runErr := cmd.CombinedOutput()
	t.Logf("forge test:\n%s", string(out))
	if runErr != nil {
		t.Fatalf("forge test 失败: %v", runErr)
	}
}
