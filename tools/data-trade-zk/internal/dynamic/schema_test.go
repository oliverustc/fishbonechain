package dynamic

import (
	"math"
	"os"
	"testing"
)

func mkTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestValidateDatasetAcceptsValid(t *testing.T) {
	ds := Dataset{
		Version:       1,
		DatasetID:     "test-ds",
		SchemaVersion: 1,
		Records: []Record{
			{RecordID: "r1", Fields: map[string]Field{
				"temp": {Type: "uint64", Value: 42, SaltHex: "0x2222222222222222222222222222222222222222222222222222222222222222", MaskDelta: 1000},
			}},
		},
	}
	if err := ValidateDataset(ds); err != nil {
		t.Fatalf("valid dataset rejected: %v", err)
	}
}

func TestValidateDatasetRejectsDuplicateRecordID(t *testing.T) {
	ds := Dataset{
		Version:       1,
		DatasetID:     "test",
		SchemaVersion: 1,
		Records: []Record{
			{RecordID: "r1", Fields: map[string]Field{"f": {Type: "uint64", Value: 1, SaltHex: "0x1111111111111111111111111111111111111111111111111111111111111111"}}},
			{RecordID: "r1", Fields: map[string]Field{"f": {Type: "uint64", Value: 2, SaltHex: "0x2222222222222222222222222222222222222222222222222222222222222222"}}},
		},
	}
	if err := ValidateDataset(ds); err == nil {
		t.Fatal("duplicate record_id should reject")
	}
}

func TestValidateDatasetRejectsUnsupportedType(t *testing.T) {
	ds := Dataset{
		Version:       1,
		DatasetID:     "test",
		SchemaVersion: 1,
		Records: []Record{
			{RecordID: "r1", Fields: map[string]Field{"f": {Type: "string", Value: 1, SaltHex: "0x1111111111111111111111111111111111111111111111111111111111111111"}}},
		},
	}
	if err := ValidateDataset(ds); err == nil {
		t.Fatal("non-uint64 type should reject")
	}
}

func TestValidateDatasetRejectsInvalidSalt(t *testing.T) {
	ds := Dataset{
		Version:       1,
		DatasetID:     "test",
		SchemaVersion: 1,
		Records: []Record{
			{RecordID: "r1", Fields: map[string]Field{"f": {Type: "uint64", Value: 1, SaltHex: "0xbad"}}},
		},
	}
	if err := ValidateDataset(ds); err == nil {
		t.Fatal("invalid salt should reject")
	}
}

func TestValidateRequestAcceptsValid(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "range",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		FieldName:      "temp",
		Range:          RangeConstraint{MinValue: 18, MaxValue: 65},
	}
	if err := ValidateRequest(r); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestValidateRequestRejectsNonRange(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "subset",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		FieldName:      "f",
		Range:          RangeConstraint{MinValue: 0, MaxValue: 10},
	}
	if err := ValidateRequest(r); err == nil {
		t.Fatal("non-range constraint_kind should reject")
	}
}

func TestValidateRequestRejectsMinGtMax(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "range",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		FieldName:      "f",
		Range:          RangeConstraint{MinValue: 100, MaxValue: 10},
	}
	if err := ValidateRequest(r); err == nil {
		t.Fatal("min_value > max_value should reject")
	}
}

func TestReadDatasetRejectsOverflowValue(t *testing.T) {
	b := []byte(`{"version":1,"dataset_id":"t","schema_version":1,"records":[{"record_id":"r","fields":{"f":{"type":"uint64","value":18446744073709551616,"salt_hex":"0x1111111111111111111111111111111111111111111111111111111111111111","mask_delta":1}}}]}`)
	p := mkTemp(t, string(b))
	defer os.Remove(p)
	_, err := ReadDataset(p)
	if err == nil {
		t.Fatal("overflow value should reject")
	}
}

func TestReadDatasetRejectsOverflowMaskDelta(t *testing.T) {
	b := []byte(`{"version":1,"dataset_id":"t","schema_version":1,"records":[{"record_id":"r","fields":{"f":{"type":"uint64","value":1,"salt_hex":"0x1111111111111111111111111111111111111111111111111111111111111111","mask_delta":` + maxUint64PlusOne() + `}}}]}`)
	p := mkTemp(t, string(b))
	defer os.Remove(p)
	_, err := ReadDataset(p)
	if err == nil {
		t.Fatal("overflow mask_delta should reject")
	}
	if err != nil && err.Error() == "" {
		t.Fatal("expected non-empty error")
	}
}

func maxUint64PlusOne() string {
	// float64 for math.MaxUint64 + 1 is precise enough.
	return "18446744073709551616"
}

func TestReadDatasetUsesUint64Guard(t *testing.T) {
	// Smoke: a value just below max uint64 should still accept.
	maxVal := uint64(math.MaxUint64)
	b := []byte(`{"version":1,"dataset_id":"t","schema_version":1,"records":[{"record_id":"r","fields":{"f":{"type":"uint64","value":` + formatUint(maxVal) + `,"salt_hex":"0x1111111111111111111111111111111111111111111111111111111111111111","mask_delta":1}}}]}`)
	p := mkTemp(t, string(b))
	defer os.Remove(p)
	_, err := ReadDataset(p)
	if err != nil {
		t.Fatalf("max uint64 value should accept: %v", err)
	}
}

func TestValidateMultiRangeAccepts(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "multi_range",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		Constraints: []RangeFieldConstraint{
			{FieldName: "temp", Range: RangeConstraint{MinValue: 10, MaxValue: 50}},
			{FieldName: "pressure", Range: RangeConstraint{MinValue: 900, MaxValue: 1100}},
		},
	}
	if err := ValidateRequest(r); err != nil {
		t.Fatalf("valid multi_range rejected: %v", err)
	}
}

func TestValidateMultiRangeRejectsOneConstraint(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "multi_range",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		Constraints: []RangeFieldConstraint{
			{FieldName: "temp", Range: RangeConstraint{MinValue: 10, MaxValue: 50}},
		},
	}
	if err := ValidateRequest(r); err == nil {
		t.Fatal("one constraint should reject in multi_range")
	}
}

func TestValidateMultiRangeRejectsDuplicateFields(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "multi_range",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		Constraints: []RangeFieldConstraint{
			{FieldName: "temp", Range: RangeConstraint{MinValue: 10, MaxValue: 50}},
			{FieldName: "temp", Range: RangeConstraint{MinValue: 60, MaxValue: 80}},
		},
	}
	if err := ValidateRequest(r); err == nil {
		t.Fatal("duplicate fields should reject")
	}
}

func TestValidateMultiRangeRejectsMinGtMax(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "multi_range",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		Constraints: []RangeFieldConstraint{
			{FieldName: "a", Range: RangeConstraint{MinValue: 10, MaxValue: 50}},
			{FieldName: "b", Range: RangeConstraint{MinValue: 100, MaxValue: 10}},
		},
	}
	if err := ValidateRequest(r); err == nil {
		t.Fatal("min > max should reject in multi_range")
	}
}

func TestValidateRequestRejectsSubset(t *testing.T) {
	r := Request{
		Version:        1,
		ConstraintKind: "subset",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "test",
		RecordID:       "r1",
		FieldName:      "f",
		Range:          RangeConstraint{MinValue: 0, MaxValue: 10},
	}
	if err := ValidateRequest(r); err == nil {
		t.Fatal("subset should still reject")
	}
}

func formatUint(v uint64) string {
	var buf [20]byte
	i := len(buf)
	u := v
	for u >= 10 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	i--
	buf[i] = byte('0' + u)
	return string(buf[i:])
}
