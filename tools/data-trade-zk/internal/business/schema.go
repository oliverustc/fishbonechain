package business

import (
	"crypto/sha256"
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

// ComputeMaskedValueHash returns SHA256(masked_value || salt) as a 0x-prefixed hex string.
func (w RangeWitness) ComputeMaskedValueHash() string {
	masked := w.RawValue + w.MaskDelta
	salt, err := hex.DecodeString(strings.TrimPrefix(w.SaltHex, "0x"))
	if err != nil {
		return ""
	}
	var data []byte
	// masked_value as 8-byte LE
	var le [8]byte
	binaryPutUint64(le[:], masked)
	data = append(data, le[:]...)
	data = append(data, salt...)
	sum := sha256.Sum256(data)
	return "0x" + hex.EncodeToString(sum[:])
}

func binaryPutUint64(buf []byte, v uint64) {
	_ = buf[7]
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
	buf[4] = byte(v >> 32)
	buf[5] = byte(v >> 40)
	buf[6] = byte(v >> 48)
	buf[7] = byte(v >> 56)
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
		expected := w.ComputeMaskedValueHash()
		if expected == "" {
			return fmt.Errorf("cannot compute masked_value_hash")
		}
		if w.MaskedValueHash != expected {
			return fmt.Errorf("masked_value_hash mismatch: got %s, expected %s", w.MaskedValueHash, expected)
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
	// Auto-compute if not provided
	if w.MaskedValueHash == "" {
		w.MaskedValueHash = w.ComputeMaskedValueHash()
	}
	return w, nil
}
