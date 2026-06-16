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

func TestRangeWitnessRejectsInvalidHexMaskedValueHash(t *testing.T) {
	w := RangeWitness{
		RequestHash:     "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:       7,
		RoundIndex:      2,
		RawValue:        42,
		MinValue:        18,
		MaxValue:        65,
		MaskDelta:       1000,
		SaltHex:         "0x2222222222222222222222222222222222222222222222222222222222222222",
		MaskedValueHash: "0xzzzz", // invalid hex
	}
	if err := w.Validate(); err == nil {
		t.Fatalf("expected invalid hex masked_value_hash to fail")
	}
}

func TestRangeWitnessAcceptsEmptyMaskedValueHash(t *testing.T) {
	w := RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   7,
		RoundIndex:  2,
		RawValue:    42,
		MinValue:    18,
		MaxValue:    65,
		MaskDelta:   1000,
		SaltHex:     "0x2222222222222222222222222222222222222222222222222222222222222222",
		// MaskedValueHash empty — validated at circuit level by GenerateBusinessRangeFixture
	}
	if err := w.Validate(); err != nil {
		t.Fatalf("empty masked_value_hash rejected: %v", err)
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
