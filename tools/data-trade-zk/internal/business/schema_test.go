package business

import "testing"

func TestRangeWitnessValidateAcceptsValidSample(t *testing.T) {
	w := RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   7,
		RoundIndex:  2,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	if err := w.Validate(); err != nil {
		t.Fatalf("validate valid witness: %v", err)
	}
}

func TestRangeWitnessValidateRejectsWrongMaskedValueHash(t *testing.T) {
	w := RangeWitness{
		RequestHash:     "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:       7,
		RoundIndex:      2,
		RawValue:        42,
		MinValue:        18,
		MaxValue:        65,
		MaskDelta:       1000,
		SaltHex:         "0x2222222222222222222222222222222222222222222222222222222222222222",
		MaskedValueHash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if err := w.Validate(); err == nil {
		t.Fatalf("expected wrong masked_value_hash to fail")
	}
}

func TestRangeWitnessValidateAcceptsCorrectMaskedValueHash(t *testing.T) {
	w := RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   7,
		RoundIndex:  2,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	correct := w.ComputeMaskedValueHash()
	w.MaskedValueHash = correct
	if err := w.Validate(); err != nil {
		t.Fatalf("correct masked_value_hash rejected: %v", err)
	}
}

func TestRangeWitnessValidateRejectsOutOfRange(t *testing.T) {
	w := RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   7,
		RoundIndex:  2,
		RawValue:    99,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	if err := w.Validate(); err == nil {
		t.Fatalf("expected out-of-range witness to fail")
	}
}
