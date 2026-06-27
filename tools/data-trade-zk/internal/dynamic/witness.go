package dynamic

import (
	"fmt"

	"fishbone-data-trade-zk/internal/business"
	"fishbone-data-trade-zk/internal/imt"
)

// BuildRangeWitnesses converts a validated dataset/request pair into a slice
// of business.RangeWitness. Single range returns one element; multi_range
// returns one per constraint.
func BuildRangeWitnesses(ds Dataset, req Request, sessionID, roundIndex uint32) ([]business.RangeWitness, error) {
	if ds.DatasetID != req.DatasetID {
		return nil, fmt.Errorf("dataset_id mismatch: dataset=%q request=%q", ds.DatasetID, req.DatasetID)
	}
	var foundRecord *Record
	for i := range ds.Records {
		if ds.Records[i].RecordID == req.RecordID {
			foundRecord = &ds.Records[i]
			break
		}
	}
	if foundRecord == nil {
		return nil, fmt.Errorf("record_id %q not found in dataset", req.RecordID)
	}

	switch req.ConstraintKind {
	case "range":
		w, err := buildOne(foundRecord, req.FieldName, req.Range, ds, req, sessionID, roundIndex)
		if err != nil {
			return nil, err
		}
		return []business.RangeWitness{w}, nil
	case "multi_range":
		var out []business.RangeWitness
		for _, c := range req.Constraints {
			w, err := buildOne(foundRecord, c.FieldName, c.Range, ds, req, sessionID, roundIndex)
			if err != nil {
				return nil, fmt.Errorf("constraint %q: %w", c.FieldName, err)
			}
			out = append(out, w)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported constraint_kind: %q", req.ConstraintKind)
	}
}

func buildOne(record *Record, fieldName string, rng RangeConstraint, ds Dataset, req Request, sessionID, roundIndex uint32) (business.RangeWitness, error) {
	field, ok := record.Fields[fieldName]
	if !ok {
		return business.RangeWitness{}, fmt.Errorf("field %q not found in record %q", fieldName, req.RecordID)
	}
	if field.Type != "uint64" {
		return business.RangeWitness{}, fmt.Errorf("field %q type is %q, expected uint64", fieldName, field.Type)
	}
	if field.Value < rng.MinValue || field.Value > rng.MaxValue {
		return business.RangeWitness{}, fmt.Errorf("field value %d outside request range [%d, %d]", field.Value, rng.MinValue, rng.MaxValue)
	}
	fixture := imt.DefaultFixture()
	fixture.DatasetID = ds.DatasetID
	fixture.FieldName = fieldName
	fixture.RecordID = req.RecordID
	fixture.SchemaVersion = ds.SchemaVersion
	return business.RangeWitness{
		RequestHash:     req.RequestHash,
		SessionID:       sessionID,
		RoundIndex:      roundIndex,
		RawValue:        field.Value,
		MinValue:        rng.MinValue,
		MaxValue:        rng.MaxValue,
		MaskDelta:       field.MaskDelta,
		SaltHex:         field.SaltHex,
		MaskedValueHash: "",
		IMT:             fixture,
	}, nil
}

// BuildRangeWitness converts a validated dataset/request pair into a
// business.RangeWitness for fishbone-zk business-fixture.
func BuildRangeWitness(ds Dataset, req Request, sessionID, roundIndex uint32) (business.RangeWitness, error) {
	witnesses, err := BuildRangeWitnesses(ds, req, sessionID, roundIndex)
	if err != nil {
		return business.RangeWitness{}, err
	}
	if len(witnesses) != 1 {
		return business.RangeWitness{}, fmt.Errorf("BuildRangeWitness expects exactly 1 witness, got %d", len(witnesses))
	}
	return witnesses[0], nil
}
