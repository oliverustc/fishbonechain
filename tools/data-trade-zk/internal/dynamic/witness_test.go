package dynamic

import (
	"testing"
)

func validDataset() Dataset {
	return Dataset{
		Version:       1,
		DatasetID:     "factory-sensors",
		SchemaVersion: 1,
		Records: []Record{
			{
				RecordID: "sensor-a",
				Fields: map[string]Field{
					"temperature": {Type: "uint64", Value: 42, SaltHex: "0x2222222222222222222222222222222222222222222222222222222222222222", MaskDelta: 1000},
					"humidity":    {Type: "uint64", Value: 58, SaltHex: "0x3333333333333333333333333333333333333333333333333333333333333333", MaskDelta: 2000},
				},
			},
		},
	}
}

func validRequest() Request {
	return Request{
		Version:        1,
		ConstraintKind: "range",
		RequestHash:    "0x1111111111111111111111111111111111111111111111111111111111111111",
		DatasetID:      "factory-sensors",
		RecordID:       "sensor-a",
		FieldName:      "temperature",
		Range:          RangeConstraint{MinValue: 18, MaxValue: 65},
	}
}

func TestBuildRangeWitnessProducesExpectedFields(t *testing.T) {
	w, err := BuildRangeWitness(validDataset(), validRequest(), 7, 2)
	if err != nil {
		t.Fatalf("BuildRangeWitness: %v", err)
	}
	if w.RawValue != 42 {
		t.Fatalf("expected raw_value 42, got %d", w.RawValue)
	}
	if w.MaskDelta != 1000 {
		t.Fatalf("expected mask_delta 1000, got %d", w.MaskDelta)
	}
	if w.SessionID != 7 || w.RoundIndex != 2 {
		t.Fatalf("expected session=7 round=2, got %d/%d", w.SessionID, w.RoundIndex)
	}
	if w.IMT.DatasetID != "factory-sensors" {
		t.Fatalf("expected imt.dataset_id factory-sensors, got %q", w.IMT.DatasetID)
	}
	if w.IMT.FieldName != "temperature" {
		t.Fatalf("expected imt.field_name temperature, got %q", w.IMT.FieldName)
	}
	if w.IMT.RecordID != "sensor-a" {
		t.Fatalf("expected imt.record_id sensor-a, got %q", w.IMT.RecordID)
	}
}

func TestBuildRangeWitnessChangingFieldChangesWitness(t *testing.T) {
	ds := validDataset()
	req := validRequest()
	req.FieldName = "humidity"
	w, err := BuildRangeWitness(ds, req, 0, 0)
	if err != nil {
		t.Fatalf("BuildRangeWitness: %v", err)
	}
	if w.RawValue != 58 {
		t.Fatalf("expected raw_value 58 for humidity, got %d", w.RawValue)
	}
	if w.MaskDelta != 2000 {
		t.Fatalf("expected mask_delta 2000 for humidity, got %d", w.MaskDelta)
	}
	if w.IMT.FieldName != "humidity" {
		t.Fatalf("expected imt.field_name humidity, got %q", w.IMT.FieldName)
	}
}

func TestBuildRangeWitnessOutOfRangeRejects(t *testing.T) {
	req := validRequest()
	req.Range = RangeConstraint{MinValue: 100, MaxValue: 200}
	_, err := BuildRangeWitness(validDataset(), req, 0, 0)
	if err == nil {
		t.Fatal("out-of-range value should reject")
	}
}

func TestBuildRangeWitnessDatasetIDMismatchRejects(t *testing.T) {
	req := validRequest()
	req.DatasetID = "other-dataset"
	_, err := BuildRangeWitness(validDataset(), req, 0, 0)
	if err == nil {
		t.Fatal("dataset_id mismatch should reject")
	}
}

func TestBuildRangeWitnessMissingRecordRejects(t *testing.T) {
	req := validRequest()
	req.RecordID = "nonexistent"
	_, err := BuildRangeWitness(validDataset(), req, 0, 0)
	if err == nil {
		t.Fatal("missing record should reject")
	}
}

func TestBuildRangeWitnessMissingFieldRejects(t *testing.T) {
	req := validRequest()
	req.FieldName = "nonexistent"
	_, err := BuildRangeWitness(validDataset(), req, 0, 0)
	if err == nil {
		t.Fatal("missing field should reject")
	}
}
