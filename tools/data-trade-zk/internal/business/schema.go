package business

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type RangeWitness struct {
	RequestHash     string `json:"request_hash"`
	SessionID       uint32 `json:"session_id"`
	RoundIndex      uint32 `json:"round_index"`
	RawValue        uint64 `json:"raw_value"`
	MinValue        uint64 `json:"min_value"`
	MaxValue        uint64 `json:"max_value"`
	MaskDelta       uint64 `json:"mask_delta"`
	SaltHex         string `json:"salt_hex"`
	MaskedValueHash string `json:"masked_value_hash,omitempty"`
}

// ComputeMaskedValueHash returns MiMC(masked_value, salt) as a 0x-prefixed hex string.
// Stage 2.2: migrated from SHA256 to MiMC for gnark circuit compatibility.
// NOTE: This function cannot compute the real MiMC hash without importing gnark
// (which would create a circular dependency). The actual MiMC computation is
// done in gnarkadapter.GenerateBusinessRangeFixture. The fixture JSON should
// provide masked_value_hash directly; Validate() checks it is valid hex format.
// ReadRangeWitness() does NOT auto-compute it.
func (w RangeWitness) ComputeMaskedValueHash() string {
	return ""
}

// IsMaskedValueHashProvided returns true if the witness has a non-empty hash.
func (w RangeWitness) IsMaskedValueHashProvided() bool {
	return w.MaskedValueHash != ""
}

func validHex32(value string) bool {
	raw := strings.TrimPrefix(strings.ToLower(value), "0x")
	b, err := hex.DecodeString(raw)
	return err == nil && len(b) == 32
}

func (w RangeWitness) Validate() error {
	if !validHex32(w.RequestHash) {
		return fmt.Errorf("request_hash must be 32-byte hex")
	}
	if !validHex32(w.SaltHex) {
		return fmt.Errorf("salt_hex must be 32-byte hex")
	}
	if w.MinValue > w.MaxValue {
		return fmt.Errorf("min_value must be <= max_value")
	}
	if w.RawValue < w.MinValue || w.RawValue > w.MaxValue {
		return fmt.Errorf("raw_value outside requested range")
	}
	if w.MaskedValueHash != "" {
		if !validHex32(w.MaskedValueHash) {
			return fmt.Errorf("masked_value_hash must be 32-byte hex if provided")
		}
	}
	return nil
}

func ReadRangeWitness(path string) (RangeWitness, error) {
	var w RangeWitness
	b, err := os.ReadFile(path)
	if err != nil {
		return w, err
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return w, err
	}
	if err := w.Validate(); err != nil {
		return w, err
	}
	return w, nil
}
