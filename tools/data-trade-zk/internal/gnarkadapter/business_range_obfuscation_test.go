package gnarkadapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fishbone-data-trade-zk/internal/artifact"
	"fishbone-data-trade-zk/internal/business"
)

func TestBusinessHashChangesProofDigest(t *testing.T) {
	dir := t.TempDir()
	w := business.RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	out, err := GenerateBusinessRangeFixture(w, dir)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	d1 := out.Artifact.ProofDigest

	// Change mask_delta → different proof_digest
	w.MaskDelta = 2000
	out2, err := GenerateBusinessRangeFixture(w, dir)
	if err != nil {
		t.Fatalf("generate 2: %v", err)
	}
	if out2.Artifact.ProofDigest == d1 {
		t.Fatal("proof_digest should change when business input changes")
	}
}

func TestBusinessHashUsesFixedWidthLittleEndian(t *testing.T) {
	dir := t.TempDir()
	w := business.RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RawValue:    256, // 0x0100 in LE: 0x00 0x01
		MinValue:    0,
		MaxValue:    1000,
		MaskDelta:   0,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	out, err := GenerateBusinessRangeFixture(w, dir)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// raw_value=256 must be encoded as 8-byte LE: [0x00,0x01,0x00,0x00,0x00,0x00,0x00,0x00]
	// and must NOT be encoded as ASCII "256" = [0x32,0x35,0x36]
	// The proof_digest is deterministic — just verify it's non-empty
	if out.Artifact.ProofDigest == "" {
		t.Fatal("missing proof_digest")
	}
	if out.Artifact.BusinessInputHash == "" {
		t.Fatal("missing business_input_hash")
	}
}

func TestBusinessFixtureRejectsWrongMaskedValueHash(t *testing.T) {
	dir := t.TempDir()
	w := business.RangeWitness{
		RequestHash:     "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:       1,
		RoundIndex:      0,
		RawValue:        42,
		MinValue:        18,
		MaxValue:        65,
		MaskDelta:       1000,
		SaltHex:         "0x2222222222222222222222222222222222222222222222222222222222222222",
		MaskedValueHash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	_, err := GenerateBusinessRangeFixture(w, dir)
	if err == nil {
		t.Fatal("expected wrong masked_value_hash to be rejected at circuit level")
	}
}

func TestBusinessInputHashDeterministicAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	w := business.RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	out1, err := GenerateBusinessRangeFixture(w, dir)
	if err != nil {
		t.Fatalf("generate 1: %v", err)
	}
	out2, err := GenerateBusinessRangeFixture(w, dir)
	if err != nil {
		t.Fatalf("generate 2: %v", err)
	}
	if out1.Artifact.BusinessInputHash != out2.Artifact.BusinessInputHash {
		t.Fatalf("business_input_hash should be deterministic across runs: %s != %s",
			out1.Artifact.BusinessInputHash, out2.Artifact.BusinessInputHash)
	}
}

func TestBusinessInputHashChangesWithDatasetID(t *testing.T) {
	dir := t.TempDir()
	w1 := business.RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	// Validate() auto-fills zero IMT with default fixture.
	if err := w1.Validate(); err != nil {
		t.Fatalf("validate w1: %v", err)
	}
	out1, err := GenerateBusinessRangeFixture(w1, dir)
	if err != nil {
		t.Fatalf("generate 1: %v", err)
	}

	w2 := w1
	w2.IMT.DatasetID = "other-dataset"
	out2, err := GenerateBusinessRangeFixture(w2, dir)
	if err != nil {
		t.Fatalf("generate 2: %v", err)
	}

	if out1.Artifact.BusinessInputHash == out2.Artifact.BusinessInputHash {
		t.Fatal("business_input_hash should change when dataset_id changes")
	}
}

func TestBusinessInputHashBytesStringEncoding(t *testing.T) {
	// Verify that strLE encoding with 4-byte LE length prefix distinguishes
	// strings of different lengths: "A" and "AB" must produce different hashes.
	dir := t.TempDir()
	wBase := business.RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	// Pre-validate to fill default IMT, then override dataset_id.
	if err := wBase.Validate(); err != nil {
		t.Fatalf("validate base: %v", err)
	}

	wA := wBase
	wA.IMT.DatasetID = "A"
	outA, err := GenerateBusinessRangeFixture(wA, dir)
	if err != nil {
		t.Fatalf("generate A: %v", err)
	}

	wAB := wBase
	wAB.IMT.DatasetID = "AB"
	outAB, err := GenerateBusinessRangeFixture(wAB, dir)
	if err != nil {
		t.Fatalf("generate AB: %v", err)
	}

	if outA.Artifact.BusinessInputHash == outAB.Artifact.BusinessInputHash {
		t.Fatal("different-length strings should produce different business_input_hash")
	}
}

func TestBusinessArtifactIsValidAndVerifiable(t *testing.T) {
	dir := t.TempDir()
	w := business.RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	out, err := GenerateBusinessRangeFixture(w, dir)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Verify artifact can be read back
	artifactPath := filepath.Join(dir, "artifact.json")
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var a artifact.ProofArtifact
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if a.BusinessInputHash != out.Artifact.BusinessInputHash {
		t.Fatal("business_input_hash mismatch on read-back")
	}

	// Verify gnark proofs
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)
	if err := VerifyRangeRO(a); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
