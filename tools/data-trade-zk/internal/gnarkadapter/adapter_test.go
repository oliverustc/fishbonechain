package gnarkadapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRangeAndROFixtureCanBeGeneratedAndVerified(t *testing.T) {
	dir := t.TempDir()
	out, err := GenerateRangeROFixture(GenerateInput{
		OutDir:      dir,
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RODepth:     10,
	})
	if err != nil {
		t.Fatalf("generate fixture: %v", err)
	}
	if out.Artifact.ProofDigest == "" {
		t.Fatal("missing proof digest")
	}
	if _, err := os.Stat(filepath.Join(dir, "artifact.json")); err != nil {
		t.Fatalf("artifact not written: %v", err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)
	if err := VerifyRangeRO(out.Artifact); err != nil {
		t.Fatalf("verify fixture: %v", err)
	}
}
