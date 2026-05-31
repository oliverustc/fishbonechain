package solgen_test

import (
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/oliverustc/bpiano/piano"
	"github.com/oliverustc/bpiano/solgen"
)

// generateBothContracts generates BPianoVerifierGen.sol and PianoVerifierGen.sol
// into sol/src/ so that forge build succeeds regardless of which test runs first.
func generateBothContracts(t *testing.T, vk *piano.VerifyingKey) string {
	t.Helper()
	solDir := findSolDir(t)

	bpianoSrc, err := solgen.ExportBPianoVerifier(vk, "BPianoVerifierGen")
	if err != nil {
		t.Fatalf("ExportBPianoVerifier: %v", err)
	}
	if err := os.WriteFile(filepath.Join(solDir, "src", "BPianoVerifierGen.sol"), []byte(bpianoSrc), 0644); err != nil {
		t.Fatalf("write BPianoVerifierGen.sol: %v", err)
	}

	pianoSrc, err := solgen.ExportPianoVerifier(vk, "PianoVerifierGen")
	if err != nil {
		t.Fatalf("ExportPianoVerifier: %v", err)
	}
	if err := os.WriteFile(filepath.Join(solDir, "src", "PianoVerifierGen.sol"), []byte(pianoSrc), 0644); err != nil {
		t.Fatalf("write PianoVerifierGen.sol: %v", err)
	}

	return solDir
}

// TestExportBPianoVerifier generates a hardcoded BPianoVerifierGen.sol contract,
// compiles it with forge build, then runs the BPianoVerifierGenTest forge tests.
func TestExportBPianoVerifier(t *testing.T) {
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	ci, _ := buildTestCircuit(T, M)
	_, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	solDir := generateBothContracts(t, vk)
	t.Logf("BPianoVerifierGen.sol and PianoVerifierGen.sol written to %s/src/", solDir)

	// Compile
	buildCmd := exec.Command("forge", "build")
	buildCmd.Dir = solDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	t.Logf("forge build output:\n%s", string(buildOut))
	if buildErr != nil {
		t.Fatalf("forge build failed: %v", buildErr)
	}

	// Run forge tests for the generated contract
	testCmd := exec.Command("forge", "test", "--match-contract", "BPianoVerifierGenTest", "-vv")
	testCmd.Dir = solDir
	testOut, testErr := testCmd.CombinedOutput()
	t.Logf("forge test output:\n%s", string(testOut))
	if testErr != nil {
		t.Fatalf("forge test BPianoVerifierGenTest failed: %v", testErr)
	}
}

// TestExportPianoVerifier generates a hardcoded PianoVerifierGen.sol contract,
// compiles it with forge build, then runs the PianoVerifierGenTest forge tests.
func TestExportPianoVerifier(t *testing.T) {
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	ci, _ := buildTestCircuit(T, M)
	_, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	solDir := generateBothContracts(t, vk)
	t.Logf("BPianoVerifierGen.sol and PianoVerifierGen.sol written to %s/src/", solDir)

	// Compile
	buildCmd := exec.Command("forge", "build")
	buildCmd.Dir = solDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	t.Logf("forge build output:\n%s", string(buildOut))
	if buildErr != nil {
		t.Fatalf("forge build failed: %v", buildErr)
	}

	// Run forge tests for the generated contract
	testCmd := exec.Command("forge", "test", "--match-contract", "PianoVerifierGenTest", "-vv")
	testCmd.Dir = solDir
	testOut, testErr := testCmd.CombinedOutput()
	t.Logf("forge test output:\n%s", string(testOut))
	if testErr != nil {
		t.Fatalf("forge test PianoVerifierGenTest failed: %v", testErr)
	}
}
