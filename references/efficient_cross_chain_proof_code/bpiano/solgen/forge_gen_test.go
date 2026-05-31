// Package solgen_test 包含端到端集成测试：
// 1. Go 生成 BPiano proof 及 calldata
// 2. 将参数导出为 Forge 测试所需的 JSON fixture
// 3. 执行 forge test 验证 Solidity 合约
package solgen_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/piano"
	"github.com/oliverustc/bpiano/solgen"
)

// Fixture 是写入磁盘供 Forge 测试读取的 JSON 文件格式。
// 所有数值以十六进制字符串（无 0x）表示，与 Solidity vm.parseUint 兼容。
type Fixture struct {
	// VK
	VK VKJson `json:"vk"`
	// Proof
	Proof ProofJson `json:"proof"`
	// 预计算 G2 点（十六进制，128 字节）
	ZTG2       [4]string `json:"zTG2"`       // [xIm, xRe, yIm, yRe]
	TauYBetaG2 [4]string `json:"tauYBetaG2"` // [xIm, xRe, yIm, yRe]
	// 公开输入（十六进制数组）
	PublicInputs []string `json:"publicInputs"`
	// Go 计算的 FS 挑战（用于 Solidity 调试对比）
	Challenges ChallengesJson `json:"challenges"`
}

type ChallengesJson struct {
	Gamma, Eta, Lambda, Alpha, Nu, Beta, Mu, Rho string
}

type G1Json [2]string  // [x, y]
type G2Json [4]string  // [xIm, xRe, yIm, yRe]

type VKJson struct {
	Ql, Qr, Qm, Qo, Qk G1Json
	S1, S2, S3          G1Json
	G2_0, G2_1, G2Y_0   G2Json
	SizeX, SizeY        string
	GeneratorX          string
	CosetShift          string
	NbPublicInputs      string
}

type ProofJson struct {
	LRO0, LRO1, LRO2                         G1Json
	Z                                         G1Json
	Hx0, Hx1, Hx2                            G1Json
	ComQX, ComVFAlpha, ComVFZS, ComGY, Pi1AggH G1Json
	// 15 标量
	EvalA, EvalB, EvalO, EvalZ, EvalZS, EvalHx, EvalHy string
	EvalQl, EvalQr, EvalQm, EvalQo, EvalQk              string
	EvalS1, EvalS2, EvalS3                               string
}

func bigHex(b []byte) string {
	return fmt.Sprintf("0x%064x", new(big.Int).SetBytes(b))
}

func frHex(b [32]byte) string {
	return bigHex(b[:])
}

func g1Json(b [solgen.G1Size]byte) G1Json {
	return G1Json{bigHex(b[0:32]), bigHex(b[32:64])}
}

func g2Json(b [solgen.G2Size]byte) G2Json {
	return G2Json{
		bigHex(b[0:32]),   // xIm
		bigHex(b[32:64]),  // xRe
		bigHex(b[64:96]),  // yIm
		bigHex(b[96:128]), // yRe
	}
}

// TestForgeE2E 端到端测试：
//  1. 构建电路、生成 proof
//  2. 调用 GenerateBPianoCalldata 提取参数
//  3. 写入 JSON fixture 到 sol/test/fixture.json
//  4. 执行 forge test BPianoVerifierTest
func TestForgeE2E(t *testing.T) {
	// ── 1. 搭建电路并生成 proof ───────────────────────────────────────────────
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
	if err := bpiano.VerifyCompressed(proof, vk, nil); err != nil {
		t.Fatalf("VerifyCompressed: %v", err)
	}

	// ── 2. 生成 calldata ──────────────────────────────────────────────────────
	res, err := solgen.GenerateBPianoCalldata(proof, vk, nil)
	if err != nil {
		t.Fatalf("GenerateBPianoCalldata: %v", err)
	}
	cd := res.Calldata
	vs := solgen.ExtractVKSolidity(vk)

	// ── 3. 构建 JSON fixture ──────────────────────────────────────────────────
	ch := res.Calldata.Challenges
	fix := Fixture{
		ZTG2:         g2Json(cd.ZTG2),
		TauYBetaG2:   g2Json(cd.TauYBetaG2),
		PublicInputs: []string{},
		Challenges: ChallengesJson{
			Gamma:  frHex(solgen.FrBytes(ch.Gamma)),
			Eta:    frHex(solgen.FrBytes(ch.Eta)),
			Lambda: frHex(solgen.FrBytes(ch.Lambda)),
			Alpha:  frHex(solgen.FrBytes(ch.Alpha)),
			Nu:     frHex(solgen.FrBytes(ch.Nu)),
			Beta:   frHex(solgen.FrBytes(ch.Beta)),
			Mu:     frHex(solgen.FrBytes(ch.Mu)),
			Rho:    frHex(solgen.FrBytes(ch.Rho)),
		},
	}

	// VK
	fix.VK = VKJson{
		Ql:             g1Json(vs.Ql),
		Qr:             g1Json(vs.Qr),
		Qm:             g1Json(vs.Qm),
		Qo:             g1Json(vs.Qo),
		Qk:             g1Json(vs.Qk),
		S1:             g1Json(vs.S1),
		S2:             g1Json(vs.S2),
		S3:             g1Json(vs.S3),
		G2_0:           g2Json(vs.G2_0),
		G2_1:           g2Json(vs.G2_1),
		G2Y_0:          g2Json(vs.G2Y_0),
		SizeX:          bigHex(vs.SizeX[:]),
		SizeY:          bigHex(vs.SizeY[:]),
		GeneratorX:     frHex(vs.GeneratorX),
		CosetShift:     frHex(vs.CosetShift),
		NbPublicInputs: fmt.Sprintf("0x%064x", 0),
	}

	// Proof
	fix.Proof = ProofJson{
		LRO0: g1Json(cd.LRO[0]), LRO1: g1Json(cd.LRO[1]), LRO2: g1Json(cd.LRO[2]),
		Z:    g1Json(cd.Z),
		Hx0:  g1Json(cd.Hx[0]), Hx1: g1Json(cd.Hx[1]), Hx2: g1Json(cd.Hx[2]),
		ComQX:      g1Json(cd.ComQX),
		ComVFAlpha: g1Json(cd.ComVFAlpha),
		ComVFZS:    g1Json(cd.ComVFZS),
		ComGY:      g1Json(cd.ComGY),
		Pi1AggH:    g1Json(cd.Pi1AggH),
		EvalA:  frHex(cd.EvalA),  EvalB:  frHex(cd.EvalB),  EvalO:  frHex(cd.EvalO),
		EvalZ:  frHex(cd.EvalZ),  EvalZS: frHex(cd.EvalZS),
		EvalHx: frHex(cd.EvalHx), EvalHy: frHex(cd.EvalHy),
		EvalQl: frHex(cd.EvalQl), EvalQr: frHex(cd.EvalQr), EvalQm: frHex(cd.EvalQm),
		EvalQo: frHex(cd.EvalQo), EvalQk: frHex(cd.EvalQk),
		EvalS1: frHex(cd.EvalS1), EvalS2: frHex(cd.EvalS2), EvalS3: frHex(cd.EvalS3),
	}

	// 写入 JSON
	solDir := findSolDir(t)
	fixPath := filepath.Join(solDir, "test", "fixture.json")
	if err := os.MkdirAll(filepath.Dir(fixPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fixBytes, _ := json.MarshalIndent(fix, "", "  ")
	if err := os.WriteFile(fixPath, fixBytes, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Logf("fixture written to %s", fixPath)

	// ── 4. 执行 forge test ─────────────────────────────────────────────────────
	cmd := exec.Command("forge", "test", "--match-contract", "BPianoVerifierTest", "-vv")
	cmd.Dir = solDir
	out, runErr := cmd.CombinedOutput()
	t.Logf("forge test output:\n%s", string(out))
	if runErr != nil {
		t.Fatalf("forge test failed: %v", runErr)
	}
}

// findSolDir 查找 sol/ 目录（相对于测试文件）。
func findSolDir(t *testing.T) string {
	t.Helper()
	// solgen/ 目录的上级是 bpiano/，sol/ 与 solgen/ 平级
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(wd, "..", "sol")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	t.Fatal("sol/ directory not found")
	return ""
}

// TestForgeE2EPublicInputs 验证含公开输入的端到端流程。
func TestForgeE2EPublicInputs(t *testing.T) {
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	// 含 1 个公开输入的电路
	ql := make([]fr.Element, T)
	ql[0].SetInt64(-1)
	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	perm := piano.BuildPermutation(lro, 3*T, T)
	qk := make([]fr.Element, T)
	var pubVal fr.Element
	pubVal.SetInt64(42)
	qk[0].Set(&pubVal)

	ci := piano.CircuitInfo{
		Ql: ql, Qr: cloneZero(T),
		Qm: cloneZero(T), Qo: cloneZero(T),
		Qk: qk, Permutation: perm,
		NbPublicInputs: 1,
	}
	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		L := make([]fr.Element, T)
		L[0].Set(&pubVal)
		witnesses[i] = piano.WitnessInstance{L: L, R: cloneZero(T), O: cloneZero(T)}
	}
	pubInputs := make([][]fr.Element, M)
	for i := range pubInputs {
		pubInputs[i] = []fr.Element{pubVal}
	}

	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	proof, err := bpiano.Compress(pk, witnesses, pubInputs)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if err := bpiano.VerifyCompressed(proof, vk, pubInputs); err != nil {
		t.Fatalf("VerifyCompressed: %v", err)
	}

	res, err := solgen.GenerateBPianoCalldata(proof, vk, pubInputs)
	if err != nil {
		t.Fatalf("GenerateBPianoCalldata: %v", err)
	}
	cd := res.Calldata
	vs := solgen.ExtractVKSolidity(vk)

	fix := Fixture{
		ZTG2:       g2Json(cd.ZTG2),
		TauYBetaG2: g2Json(cd.TauYBetaG2),
	}
	// 公开输入
	for _, b := range cd.PublicInputs {
		fix.PublicInputs = append(fix.PublicInputs, frHex(b))
	}

	fix.VK = VKJson{
		Ql: g1Json(vs.Ql), Qr: g1Json(vs.Qr), Qm: g1Json(vs.Qm),
		Qo: g1Json(vs.Qo), Qk: g1Json(vs.Qk),
		S1: g1Json(vs.S1), S2: g1Json(vs.S2), S3: g1Json(vs.S3),
		G2_0: g2Json(vs.G2_0), G2_1: g2Json(vs.G2_1), G2Y_0: g2Json(vs.G2Y_0),
		SizeX: bigHex(vs.SizeX[:]),
		SizeY: bigHex(vs.SizeY[:]),
		GeneratorX: frHex(vs.GeneratorX),
		CosetShift: frHex(vs.CosetShift),
		NbPublicInputs: fmt.Sprintf("0x%064x", 1),
	}
	fix.Proof = ProofJson{
		LRO0: g1Json(cd.LRO[0]), LRO1: g1Json(cd.LRO[1]), LRO2: g1Json(cd.LRO[2]),
		Z:   g1Json(cd.Z),
		Hx0: g1Json(cd.Hx[0]), Hx1: g1Json(cd.Hx[1]), Hx2: g1Json(cd.Hx[2]),
		ComQX: g1Json(cd.ComQX), ComVFAlpha: g1Json(cd.ComVFAlpha),
		ComVFZS: g1Json(cd.ComVFZS), ComGY: g1Json(cd.ComGY), Pi1AggH: g1Json(cd.Pi1AggH),
		EvalA: frHex(cd.EvalA), EvalB: frHex(cd.EvalB), EvalO: frHex(cd.EvalO),
		EvalZ: frHex(cd.EvalZ), EvalZS: frHex(cd.EvalZS),
		EvalHx: frHex(cd.EvalHx), EvalHy: frHex(cd.EvalHy),
		EvalQl: frHex(cd.EvalQl), EvalQr: frHex(cd.EvalQr), EvalQm: frHex(cd.EvalQm),
		EvalQo: frHex(cd.EvalQo), EvalQk: frHex(cd.EvalQk),
		EvalS1: frHex(cd.EvalS1), EvalS2: frHex(cd.EvalS2), EvalS3: frHex(cd.EvalS3),
	}

	solDir := findSolDir(t)
	fixPath := filepath.Join(solDir, "test", "fixture_pi.json")
	fixBytes, _ := json.MarshalIndent(fix, "", "  ")
	if err := os.WriteFile(fixPath, fixBytes, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Logf("PI fixture written to %s", fixPath)

	cmd := exec.Command("forge", "test", "--match-contract", "BPianoVerifierPITest", "-vv")
	cmd.Dir = solDir
	out, runErr := cmd.CombinedOutput()
	t.Logf("forge test output:\n%s", string(out))
	if runErr != nil {
		t.Fatalf("forge test (PI) failed: %v", runErr)
	}
}
