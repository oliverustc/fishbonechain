package dynamic

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// ── Dataset ──────────────────────────────────────────────────────────

type Dataset struct {
	Version       int      `json:"version"`
	DatasetID     string   `json:"dataset_id"`
	Description   string   `json:"description"`
	SchemaVersion int      `json:"schema_version"`
	Records       []Record `json:"records"`
}

type Record struct {
	RecordID string           `json:"record_id"`
	Fields   map[string]Field `json:"fields"`
}

type Field struct {
	Type      string `json:"type"`
	Value     uint64 `json:"value"`
	SaltHex   string `json:"salt_hex"`
	MaskDelta uint64 `json:"mask_delta"`
}

// ── Request ──────────────────────────────────────────────────────────

type Request struct {
	Version        int    `json:"version"`
	ConstraintKind string `json:"constraint_kind"`
	RequestHash    string `json:"request_hash"`
	DatasetID      string `json:"dataset_id"`
	RecordID       string `json:"record_id"`
	FieldName      string                `json:"field_name"`
	Range          RangeConstraint       `json:"range"`
	Constraints    []RangeFieldConstraint `json:"constraints,omitempty"`
}

type RangeConstraint struct {
	MinValue uint64 `json:"min_value"`
	MaxValue uint64 `json:"max_value"`
}

type RangeFieldConstraint struct {
	FieldName string          `json:"field_name"`
	Range     RangeConstraint `json:"range"`
}

// ── Validation ───────────────────────────────────────────────────────

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

func validHex32(v string) bool {
	raw := strings.TrimPrefix(strings.ToLower(v), "0x")
	b, err := hex.DecodeString(raw)
	return err == nil && len(b) == 32
}

func ValidateDataset(d Dataset) error {
	if d.Version != 1 {
		return fmt.Errorf("dataset version must be 1, got %d", d.Version)
	}
	if d.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1, got %d", d.SchemaVersion)
	}
	if strings.TrimSpace(d.DatasetID) == "" {
		return fmt.Errorf("dataset_id must not be empty")
	}
	if !isASCII(d.DatasetID) {
		return fmt.Errorf("dataset_id must be ASCII")
	}
	if len(d.Records) == 0 {
		return fmt.Errorf("at least 1 record required")
	}
	seenRecords := map[string]bool{}
	for _, r := range d.Records {
		if strings.TrimSpace(r.RecordID) == "" {
			return fmt.Errorf("record_id must not be empty")
		}
		if !isASCII(r.RecordID) {
			return fmt.Errorf("record_id must be ASCII: %q", r.RecordID)
		}
		if seenRecords[r.RecordID] {
			return fmt.Errorf("duplicate record_id: %q", r.RecordID)
		}
		seenRecords[r.RecordID] = true
		if len(r.Fields) == 0 {
			return fmt.Errorf("record %q must have at least 1 field", r.RecordID)
		}
		seenFields := map[string]bool{}
		for fn, f := range r.Fields {
			if strings.TrimSpace(fn) == "" {
				return fmt.Errorf("field name must not be empty in record %q", r.RecordID)
			}
			if !isASCII(fn) {
				return fmt.Errorf("field name must be ASCII: %q", fn)
			}
			if seenFields[fn] {
				return fmt.Errorf("duplicate field name %q in record %q", fn, r.RecordID)
			}
			seenFields[fn] = true
			if f.Type != "uint64" {
				return fmt.Errorf("field %q type must be uint64 in Stage 8, got %q", fn, f.Type)
			}
			if !validHex32(f.SaltHex) {
				return fmt.Errorf("field %q salt_hex must be 32-byte hex", fn)
			}
		}
	}
	return nil
}

func ValidateRequest(r Request) error {
	if r.Version != 1 {
		return fmt.Errorf("request version must be 1, got %d", r.Version)
	}
	if r.ConstraintKind != "range" && r.ConstraintKind != "multi_range" {
		return fmt.Errorf("constraint_kind must be range or multi_range, got %q", r.ConstraintKind)
	}
	if !validHex32(r.RequestHash) {
		return fmt.Errorf("request_hash must be 32-byte hex")
	}
	if strings.TrimSpace(r.DatasetID) == "" {
		return fmt.Errorf("dataset_id must not be empty")
	}
	if !isASCII(r.DatasetID) {
		return fmt.Errorf("dataset_id must be ASCII")
	}
	if strings.TrimSpace(r.RecordID) == "" {
		return fmt.Errorf("record_id must not be empty")
	}
	if !isASCII(r.RecordID) {
		return fmt.Errorf("record_id must be ASCII")
	}

	if r.ConstraintKind == "range" {
		if len(r.Constraints) > 0 {
			return fmt.Errorf("constraints must be empty for single range request")
		}
		if strings.TrimSpace(r.FieldName) == "" {
			return fmt.Errorf("field_name must not be empty")
		}
		if !isASCII(r.FieldName) {
			return fmt.Errorf("field_name must be ASCII")
		}
		if r.Range.MinValue > r.Range.MaxValue {
			return fmt.Errorf("min_value must be <= max_value")
		}
		return nil
	}

	// multi_range
	if r.FieldName != "" {
		return fmt.Errorf("field_name must be empty for multi_range request")
	}
	if r.Range.MinValue != 0 || r.Range.MaxValue != 0 {
		return fmt.Errorf("top-level range must be omitted for multi_range request")
	}
	if len(r.Constraints) < 2 {
		return fmt.Errorf("multi_range requires at least 2 constraints, got %d", len(r.Constraints))
	}
	if len(r.Constraints) > 4 {
		return fmt.Errorf("multi_range supports at most 4 constraints, got %d", len(r.Constraints))
	}
	seenFields := map[string]bool{}
	for _, c := range r.Constraints {
		if strings.TrimSpace(c.FieldName) == "" {
			return fmt.Errorf("constraint field_name must not be empty")
		}
		if !isASCII(c.FieldName) {
			return fmt.Errorf("constraint field_name must be ASCII: %q", c.FieldName)
		}
		if c.Range.MinValue > c.Range.MaxValue {
			return fmt.Errorf("constraint %q: min_value must be <= max_value", c.FieldName)
		}
		if seenFields[c.FieldName] {
			return fmt.Errorf("duplicate field_name in constraints: %q", c.FieldName)
		}
		seenFields[c.FieldName] = true
	}
	return nil
}

// ── JSON helpers with uint64 overflow guard ─────────────────────────

// uint64Strict parses a uint64 with overflow detection.
func uint64Strict(n json.Number) (uint64, error) {
	f, err := n.Float64()
	if err != nil {
		return 0, fmt.Errorf("invalid number: %w", err)
	}
	if f < 0 || f > float64(math.MaxUint64) {
		return 0, fmt.Errorf("value %s out of uint64 range", n.String())
	}
	v, err := n.Int64()
	if err != nil {
		// n.Int64 fails for values > math.MaxInt64; use Uint64 path.
		// json.Number does not have Uint64 directly.
		// Parse manually via string.
		var u uint64
		if _, scanErr := fmt.Sscanf(n.String(), "%d", &u); scanErr != nil {
			return 0, fmt.Errorf("cannot parse uint64: %w", scanErr)
		}
		return u, nil
	}
	return uint64(v), nil
}

// readDatasetJSON reads a dataset with uint64 overflow detection.
func readDatasetJSON(path string) (Dataset, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return Dataset{}, "", err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	// First: decode into raw to get string representation for re-validation.
	var raw map[string]interface{}
	if err := dec.Decode(&raw); err != nil {
		return Dataset{}, "", err
	}

	// Second: decode the same file fresh into the typed struct.
	f2, err := os.Open(path)
	if err != nil {
		return Dataset{}, "", err
	}
	defer f2.Close()
	dec2 := json.NewDecoder(f2)
	dec2.UseNumber()
	var ds Dataset
	if err := dec2.Decode(&ds); err != nil {
		return ds, "", err
	}

	// Re-validate each uint64 field value against math.MaxUint64.
	records, _ := raw["records"].([]interface{})
	for ri, r := range records {
		rm, _ := r.(map[string]interface{})
		fields, _ := rm["fields"].(map[string]interface{})
		for fn, fv := range fields {
			fm, _ := fv.(map[string]interface{})
			if v, ok := fm["value"].(json.Number); ok {
				if _, err := uint64Strict(v); err != nil {
					return ds, "", fmt.Errorf("record %q field %q: value %w", ds.Records[ri].RecordID, fn, err)
				}
			}
			if mv, ok := fm["mask_delta"].(json.Number); ok {
				if _, err := uint64Strict(mv); err != nil {
					return ds, "", fmt.Errorf("record %q field %q: mask_delta %w", ds.Records[ri].RecordID, fn, err)
				}
			}
		}
	}

	return ds, "", nil
}

// ReadDataset reads and validates a dataset JSON file.
func ReadDataset(path string) (Dataset, error) {
	ds, _, err := readDatasetJSON(path)
	if err != nil {
		return ds, err
	}
	if err := ValidateDataset(ds); err != nil {
		return ds, err
	}
	return ds, nil
}

// ReadRequest reads and validates a request JSON file.
func ReadRequest(path string) (Request, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Request{}, err
	}
	var r Request
	if err := json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	if err := ValidateRequest(r); err != nil {
		return r, err
	}
	return r, nil
}
