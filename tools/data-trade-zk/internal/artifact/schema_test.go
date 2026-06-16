package artifact

import "testing"

func TestComputeProofDigestIsStable(t *testing.T) {
	p := ProofArtifact{
		Version:            1,
		ProofSystem:        "gnark-groth16-bn254",
		ProofSystemCode:    1,
		ConstraintKind:     "range",
		ConstraintKindCode: 1,
		RODepth:            10,
		RequestHash:        "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:          7,
		RoundIndex:         2,
		VKHash:             "0x2222222222222222222222222222222222222222222222222222222222222222",
		CHProofHash:        "0x3333333333333333333333333333333333333333333333333333333333333333",
		ROProofHash:        "0x4444444444444444444444444444444444444444444444444444444444444444",
		PublicInputHash:    "0x5555555555555555555555555555555555555555555555555555555555555555",
		BusinessInputHash:  "0x6666666666666666666666666666666666666666666666666666666666666666",
	}
	got, err := p.ComputeProofDigest()
	if err != nil {
		t.Fatalf("compute digest: %v", err)
	}
	if got == "" || got[:2] != "0x" || len(got) != 66 {
		t.Fatalf("bad digest format: %q", got)
	}
	p.ProofDigest = got
	if err := p.Validate(); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}
}

func TestValidateRejectsDigestMismatch(t *testing.T) {
	p := ProofArtifact{
		Version:            1,
		ProofSystem:        "gnark-groth16-bn254",
		ProofSystemCode:    1,
		ConstraintKind:     "range",
		ConstraintKindCode: 1,
		RODepth:            10,
		RequestHash:        "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:          7,
		RoundIndex:         2,
		VKHash:             "0x2222222222222222222222222222222222222222222222222222222222222222",
		CHProofHash:        "0x3333333333333333333333333333333333333333333333333333333333333333",
		ROProofHash:        "0x4444444444444444444444444444444444444444444444444444444444444444",
		PublicInputHash:    "0x5555555555555555555555555555555555555555555555555555555555555555",
		BusinessInputHash:  "0x6666666666666666666666666666666666666666666666666666666666666666",
		ProofDigest:        "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected digest mismatch")
	}
}

func TestValidateRejectsInvalidProofSystem(t *testing.T) {
	p := ProofArtifact{
		Version:            1,
		ProofSystem:        "invalid-system",
		ProofSystemCode:    1,
		ConstraintKind:     "range",
		ConstraintKindCode: 1,
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected rejection of invalid proof system")
	}
}

func TestBusinessProofDigestIncludesBusinessInputHash(t *testing.T) {
	p := ProofArtifact{
		Version:            1,
		ProofSystem:        "gnark-groth16-bn254",
		ProofSystemCode:    1,
		ConstraintKind:     "range",
		ConstraintKindCode: 1,
		RODepth:            10,
		RequestHash:        "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:          1,
		RoundIndex:         0,
		VKHash:             "0x2222222222222222222222222222222222222222222222222222222222222222",
		CHProofHash:        "0x3333333333333333333333333333333333333333333333333333333333333333",
		ROProofHash:        "0x4444444444444444444444444444444444444444444444444444444444444444",
		PublicInputHash:    "0x5555555555555555555555555555555555555555555555555555555555555555",
		BusinessInputHash:  "0x6666666666666666666666666666666666666666666666666666666666666666",
	}
	// Digest changes when business_input_hash changes (pallet now includes it)
	a, err := p.ComputeProofDigest()
	if err != nil {
		t.Fatalf("digest a: %v", err)
	}
	p.BusinessInputHash = "0x7777777777777777777777777777777777777777777777777777777777777777"
	b, err := p.ComputeProofDigest()
	if err != nil {
		t.Fatalf("digest b: %v", err)
	}
	if a == b {
		t.Fatalf("business input hash must affect proof digest")
	}
}
