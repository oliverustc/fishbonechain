// Package solgen_test 包含聚合 BPiano 证明的端到端集成测试：
// 1. Go 生成 K 个 BPiano 压缩证明（共享 α/β）
// 2. 聚合为 AggregatedProof，调用 GenerateAggCalldata 提取参数
// 3. 将参数导出为 Forge 测试所需的 JSON fixture
// 4. 执行 forge test 验证 AggBPianoVerifier 合约
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

// kValuesAgg 是 Agg Gas 测试覆盖的 K 值序列（与 bench 包保持一致）。
var kValuesAgg = []int{2, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

// AggFixture 是写入磁盘供 Forge 聚合验证测试读取的 JSON 文件格式。
// Proofs 字段为数组，支持任意 K 值（K=1..100）。
type AggFixture struct {
	VK         VKJson      `json:"vk"`
	K          int         `json:"K"`
	Proofs     []ProofJson `json:"proofs"`
	ComQXTotal G1Json      `json:"comQXTotal"`
	Pi1Total   G1Json      `json:"pi1Total"`
	ZTG2       G2Json      `json:"zTG2"`
	TauYBetaG2 G2Json      `json:"tauYBetaG2"`
}

// aggProofToJson 将 CompressedProof 转换为 ProofJson。
func aggProofToJson(p *bpiano.CompressedProof) ProofJson {
	return ProofJson{
		LRO0: g1Json(solgen.G1Bytes(p.LRO[0])),
		LRO1: g1Json(solgen.G1Bytes(p.LRO[1])),
		LRO2: g1Json(solgen.G1Bytes(p.LRO[2])),
		Z:    g1Json(solgen.G1Bytes(p.Z)),
		Hx0:  g1Json(solgen.G1Bytes(p.Hx[0])),
		Hx1:  g1Json(solgen.G1Bytes(p.Hx[1])),
		Hx2:  g1Json(solgen.G1Bytes(p.Hx[2])),
		ComQX:      g1Json(solgen.G1Bytes(p.ComQX)),
		ComVFAlpha: g1Json(solgen.G1Bytes(p.ComVFAlpha)),
		ComVFZS:    g1Json(solgen.G1Bytes(p.ComVFZS)),
		ComGY:      g1Json(solgen.G1Bytes(p.ComGY)),
		Pi1AggH:    g1Json(solgen.G1Bytes(p.Pi1AggH)),
		EvalA:  frHex(solgen.FrBytes(p.EvalA)),
		EvalB:  frHex(solgen.FrBytes(p.EvalB)),
		EvalO:  frHex(solgen.FrBytes(p.EvalO)),
		EvalZ:  frHex(solgen.FrBytes(p.EvalZ)),
		EvalZS: frHex(solgen.FrBytes(p.EvalZS)),
		EvalHx: frHex(solgen.FrBytes(p.EvalHx)),
		EvalHy: frHex(solgen.FrBytes(p.EvalHy)),
		EvalQl: frHex(solgen.FrBytes(p.EvalQl)),
		EvalQr: frHex(solgen.FrBytes(p.EvalQr)),
		EvalQm: frHex(solgen.FrBytes(p.EvalQm)),
		EvalQo: frHex(solgen.FrBytes(p.EvalQo)),
		EvalQk: frHex(solgen.FrBytes(p.EvalQk)),
		EvalS1: frHex(solgen.FrBytes(p.EvalS1)),
		EvalS2: frHex(solgen.FrBytes(p.EvalS2)),
		EvalS3: frHex(solgen.FrBytes(p.EvalS3)),
	}
}

// setupAggTest 构建电路并生成 PK/VK（T=8, M=2 小型测试电路）。
// 所有 Agg 测试共享相同的 setup 参数，可复用以节省时间。
func setupAggTest(t *testing.T) (*piano.ProvingKey, *piano.VerifyingKey, []piano.WitnessInstance) {
	t.Helper()
	const T, M = 8, 2
	ci, witnesses := buildTestCircuit(T, M)
	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), big.NewInt(17), big.NewInt(23))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	return pk, vk, witnesses
}

// writeAggFixture 生成 K 个共享 α/β 的压缩证明并将 fixture 写入磁盘。
// 不运行 forge test，供 TestForgeE2E_Agg_AllK 批量生成 fixture 使用。
func writeAggFixture(t *testing.T, K int, pk *piano.ProvingKey, vk *piano.VerifyingKey, witnesses []piano.WitnessInstance) {
	t.Helper()

	// ── 1. 准备 K 组 pks/witnessSlices/publicInputs ──────────────────────────
	pks := make([]*piano.ProvingKey, K)
	witnessSlices := make([][]piano.WitnessInstance, K)
	piSlices := make([][][]fr.Element, K)
	for k := 0; k < K; k++ {
		pks[k] = pk
		witnessSlices[k] = witnesses
		piSlices[k] = nil
	}

	// ── 2. 生成 K 个共享 α/β 的压缩证明 ─────────────────────────────────────
	proofs, err := bpiano.CoordinateChallenges(pks, witnessSlices, piSlices)
	if err != nil {
		t.Fatalf("CoordinateChallenges(K=%d): %v", K, err)
	}

	// ── 3. 聚合证明 ───────────────────────────────────────────────────────────
	agg, err := bpiano.AggregateProofs(proofs)
	if err != nil {
		t.Fatalf("AggregateProofs(K=%d): %v", K, err)
	}

	// ── 4. 生成 calldata ──────────────────────────────────────────────────────
	res, err := solgen.GenerateAggCalldata(agg, vk)
	if err != nil {
		t.Fatalf("GenerateAggCalldata(K=%d): %v", K, err)
	}

	// ── 5. 提取 VK ────────────────────────────────────────────────────────────
	vs := solgen.ExtractVKSolidity(vk)
	vkJson := VKJson{
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

	// ── 6. 构建 AggFixture ────────────────────────────────────────────────────
	proofJsons := make([]ProofJson, K)
	for i, p := range res.Proofs {
		proofJsons[i] = aggProofToJson(p)
	}
	fix := AggFixture{
		VK:         vkJson,
		K:          K,
		Proofs:     proofJsons,
		ComQXTotal: g1Json(res.ComQXTotal),
		Pi1Total:   g1Json(res.Pi1Total),
		ZTG2:       g2Json(res.ZTG2),
		TauYBetaG2: g2Json(res.TauYBetaG2),
	}

	// ── 7. 写入 JSON fixture ──────────────────────────────────────────────────
	solDir := findSolDir(t)
	fixPath := filepath.Join(solDir, "test", fmt.Sprintf("fixture_agg_k%d.json", K))
	if err := os.MkdirAll(filepath.Dir(fixPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fixBytes, _ := json.MarshalIndent(fix, "", "  ")
	if err := os.WriteFile(fixPath, fixBytes, 0644); err != nil {
		t.Fatalf("write fixture K=%d: %v", K, err)
	}
	t.Logf("AggFixture K=%d written to %s", K, fixPath)
}

// runAggForgeE2E 执行 K 个聚合证明的端到端测试（生成 fixture + forge test）。
func runAggForgeE2E(t *testing.T, K int) {
	t.Helper()
	pk, vk, witnesses := setupAggTest(t)
	writeAggFixture(t, K, pk, vk, witnesses)

	solDir := findSolDir(t)
	matchTest := fmt.Sprintf("testVerify_AggK%d", K)
	cmd := exec.Command("forge", "test",
		"--match-contract", "AggBPianoVerifierTest",
		"--match-test", matchTest,
		"-vv")
	cmd.Dir = solDir
	out, runErr := cmd.CombinedOutput()
	t.Logf("forge test output:\n%s", string(out))
	if runErr != nil {
		t.Fatalf("forge test (AggK%d) failed: %v", K, runErr)
	}
}

// TestForgeE2E_Agg_AllK 为 kValuesAgg 中所有 K 值生成 fixture，
// 然后一次性运行 forge test --match-contract AggBPianoVerifierTest。
// Gas 输出行格式："AggK{K} gas: XXXXX"，供 benchAggGas 解析。
func TestForgeE2E_Agg_AllK(t *testing.T) {
	pk, vk, witnesses := setupAggTest(t)

	for _, K := range kValuesAgg {
		writeAggFixture(t, K, pk, vk, witnesses)
	}

	solDir := findSolDir(t)
	cmd := exec.Command("forge", "test",
		"--match-contract", "AggBPianoVerifierTest",
		"-vv")
	cmd.Dir = solDir
	out, runErr := cmd.CombinedOutput()
	t.Logf("forge test output:\n%s", string(out))
	if runErr != nil {
		t.Fatalf("forge test (AggAllK) failed: %v", runErr)
	}
}

// ── 单 K 测试函数（可独立运行）────────────────────────────────────────────────

func TestForgeE2E_Agg_K1(t *testing.T)   { runAggForgeE2E(t, 1) }
func TestForgeE2E_Agg_K2(t *testing.T)   { runAggForgeE2E(t, 2) }
func TestForgeE2E_Agg_K4(t *testing.T)   { runAggForgeE2E(t, 4) }
func TestForgeE2E_Agg_K8(t *testing.T)   { runAggForgeE2E(t, 8) }
func TestForgeE2E_Agg_K10(t *testing.T)  { runAggForgeE2E(t, 10) }
func TestForgeE2E_Agg_K20(t *testing.T)  { runAggForgeE2E(t, 20) }
func TestForgeE2E_Agg_K30(t *testing.T)  { runAggForgeE2E(t, 30) }
func TestForgeE2E_Agg_K40(t *testing.T)  { runAggForgeE2E(t, 40) }
func TestForgeE2E_Agg_K50(t *testing.T)  { runAggForgeE2E(t, 50) }
func TestForgeE2E_Agg_K60(t *testing.T)  { runAggForgeE2E(t, 60) }
func TestForgeE2E_Agg_K70(t *testing.T)  { runAggForgeE2E(t, 70) }
func TestForgeE2E_Agg_K80(t *testing.T)  { runAggForgeE2E(t, 80) }
func TestForgeE2E_Agg_K90(t *testing.T)  { runAggForgeE2E(t, 90) }
func TestForgeE2E_Agg_K100(t *testing.T) { runAggForgeE2E(t, 100) }
