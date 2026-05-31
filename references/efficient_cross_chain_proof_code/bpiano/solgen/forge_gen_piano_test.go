package solgen_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/oliverustc/bpiano/piano"
	"github.com/oliverustc/bpiano/solgen"
)

// PianoFixture 是写入磁盘供 Forge 测试读取的 JSON 文件格式（Piano 版）。
type PianoFixture struct {
	VK           VKJson           `json:"vk"`
	Proof        PianoProofJson   `json:"proof"`
	TauYBetaG2   [4]string        `json:"tauYBetaG2"`
	PublicInputs []string         `json:"publicInputs"`
	Challenges   PianoChallengesJson `json:"challenges"`
}

type PianoChallengesJson struct {
	Gamma, Eta, Lambda, Alpha, Beta string
}

type PianoProofJson struct {
	LRO0, LRO1, LRO2 G1Json
	Z                 G1Json
	Hx0, Hx1, Hx2    G1Json
	Hy0, Hy1, Hy2    G1Json
	BatchXH           G1Json
	// BatchedProofX.ClaimedDigests[0..12]
	CD0, CD1, CD2, CD3, CD4, CD5, CD6  G1Json
	CD7, CD8, CD9, CD10, CD11, CD12    G1Json
	ZsH, ZsComVF                       G1Json
	BatchYH                            G1Json
	// Fr 求值
	EvalA, EvalB, EvalO, EvalZ, EvalZS, EvalHx, EvalHy string
	EvalQl, EvalQr, EvalQm, EvalQo, EvalQk              string
	EvalS1, EvalS2, EvalS3                               string
	// BatchedProofY.ClaimedValues[0..14]
	BatchYVal0, BatchYVal1, BatchYVal2, BatchYVal3, BatchYVal4   string
	BatchYVal5, BatchYVal6, BatchYVal7, BatchYVal8, BatchYVal9   string
	BatchYVal10, BatchYVal11, BatchYVal12, BatchYVal13, BatchYVal14 string
}

// TestForgeE2EPiano 端到端测试：
//  1. 构建电路、生成 Piano proof
//  2. 调用 GeneratePianoCalldata 提取参数
//  3. 写入 JSON fixture 到 sol/test/fixture_piano.json
//  4. 执行 forge test PianoVerifierTest
func TestForgeE2EPiano(t *testing.T) {
	// ── 1. 搭建电路并生成 proof ───────────────────────────────────────────────
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
	if err := piano.Verify(vk, proof, nil); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	// ── 2. 生成 calldata ──────────────────────────────────────────────────────
	res, err := solgen.GeneratePianoCalldata(proof, vk, nil)
	if err != nil {
		t.Fatalf("GeneratePianoCalldata: %v", err)
	}
	cd := res.Calldata
	vs := res.VK

	// ── 3. 构建 JSON fixture ──────────────────────────────────────────────────
	ch := cd.Challenges
	fix := PianoFixture{
		TauYBetaG2:   g2Json(cd.TauYBetaG2),
		PublicInputs: []string{},
		Challenges: PianoChallengesJson{
			Gamma:  frHex(solgen.FrBytes(ch.Gamma)),
			Eta:    frHex(solgen.FrBytes(ch.Eta)),
			Lambda: frHex(solgen.FrBytes(ch.Lambda)),
			Alpha:  frHex(solgen.FrBytes(ch.Alpha)),
			Beta:   frHex(solgen.FrBytes(ch.Beta)),
		},
	}

	// VK
	fix.VK = VKJson{
		Ql: g1Json(vs.Ql), Qr: g1Json(vs.Qr), Qm: g1Json(vs.Qm),
		Qo: g1Json(vs.Qo), Qk: g1Json(vs.Qk),
		S1: g1Json(vs.S1), S2: g1Json(vs.S2), S3: g1Json(vs.S3),
		G2_0: g2Json(vs.G2_0), G2_1: g2Json(vs.G2_1), G2Y_0: g2Json(vs.G2Y_0),
		SizeX:          bigHex(vs.SizeX[:]),
		SizeY:          bigHex(vs.SizeY[:]),
		GeneratorX:     frHex(vs.GeneratorX),
		CosetShift:     frHex(vs.CosetShift),
		NbPublicInputs: fmt.Sprintf("0x%064x", 0),
	}

	// Proof
	fix.Proof = PianoProofJson{
		LRO0: g1Json(cd.LRO[0]), LRO1: g1Json(cd.LRO[1]), LRO2: g1Json(cd.LRO[2]),
		Z:    g1Json(cd.Z),
		Hx0:  g1Json(cd.Hx[0]), Hx1: g1Json(cd.Hx[1]), Hx2: g1Json(cd.Hx[2]),
		Hy0:  g1Json(cd.Hy[0]), Hy1: g1Json(cd.Hy[1]), Hy2: g1Json(cd.Hy[2]),
		BatchXH: g1Json(cd.BatchXH),
		CD0:  g1Json(cd.ClaimedDigs[0]),
		CD1:  g1Json(cd.ClaimedDigs[1]),
		CD2:  g1Json(cd.ClaimedDigs[2]),
		CD3:  g1Json(cd.ClaimedDigs[3]),
		CD4:  g1Json(cd.ClaimedDigs[4]),
		CD5:  g1Json(cd.ClaimedDigs[5]),
		CD6:  g1Json(cd.ClaimedDigs[6]),
		CD7:  g1Json(cd.ClaimedDigs[7]),
		CD8:  g1Json(cd.ClaimedDigs[8]),
		CD9:  g1Json(cd.ClaimedDigs[9]),
		CD10: g1Json(cd.ClaimedDigs[10]),
		CD11: g1Json(cd.ClaimedDigs[11]),
		CD12: g1Json(cd.ClaimedDigs[12]),
		ZsH:     g1Json(cd.ZsH),
		ZsComVF: g1Json(cd.ZsComVF),
		BatchYH: g1Json(cd.BatchYH),
		EvalA:   frHex(cd.EvalA), EvalB:  frHex(cd.EvalB), EvalO:  frHex(cd.EvalO),
		EvalZ:   frHex(cd.EvalZ), EvalZS: frHex(cd.EvalZS),
		EvalHx:  frHex(cd.EvalHx), EvalHy: frHex(cd.EvalHy),
		EvalQl:  frHex(cd.EvalQl), EvalQr: frHex(cd.EvalQr), EvalQm: frHex(cd.EvalQm),
		EvalQo:  frHex(cd.EvalQo), EvalQk: frHex(cd.EvalQk),
		EvalS1:  frHex(cd.EvalS1), EvalS2: frHex(cd.EvalS2), EvalS3: frHex(cd.EvalS3),
		BatchYVal0:  frHex(cd.BatchYVals[0]),
		BatchYVal1:  frHex(cd.BatchYVals[1]),
		BatchYVal2:  frHex(cd.BatchYVals[2]),
		BatchYVal3:  frHex(cd.BatchYVals[3]),
		BatchYVal4:  frHex(cd.BatchYVals[4]),
		BatchYVal5:  frHex(cd.BatchYVals[5]),
		BatchYVal6:  frHex(cd.BatchYVals[6]),
		BatchYVal7:  frHex(cd.BatchYVals[7]),
		BatchYVal8:  frHex(cd.BatchYVals[8]),
		BatchYVal9:  frHex(cd.BatchYVals[9]),
		BatchYVal10: frHex(cd.BatchYVals[10]),
		BatchYVal11: frHex(cd.BatchYVals[11]),
		BatchYVal12: frHex(cd.BatchYVals[12]),
		BatchYVal13: frHex(cd.BatchYVals[13]),
		BatchYVal14: frHex(cd.BatchYVals[14]),
	}

	// 写入 JSON
	solDir := findSolDir(t)
	fixPath := filepath.Join(solDir, "test", "fixture_piano.json")
	if err := os.MkdirAll(filepath.Dir(fixPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fixBytes, _ := json.MarshalIndent(fix, "", "  ")
	if err := os.WriteFile(fixPath, fixBytes, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Logf("Piano fixture written to %s", fixPath)

	// ── 4. 执行 forge test ─────────────────────────────────────────────────────
	cmd := exec.Command("forge", "test", "--match-contract", "PianoVerifierTest", "-vv")
	cmd.Dir = solDir
	out, runErr := cmd.CombinedOutput()
	t.Logf("forge test output:\n%s", string(out))
	if runErr != nil {
		t.Fatalf("forge test failed: %v", runErr)
	}
}
