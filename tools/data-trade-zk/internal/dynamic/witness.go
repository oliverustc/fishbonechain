package dynamic

import (
	"fmt"

	"fishbone-data-trade-zk/internal/business"
	"fishbone-data-trade-zk/internal/imt"
)

// BuildRangeWitness converts a validated dataset/request pair into a
// business.RangeWitness for fishbone-zk business-fixture.
func BuildRangeWitness(ds Dataset, req Request, sessionID, roundIndex uint32) (business.RangeWitness, error) {
	// Cross-document consistency checks.
	if ds.DatasetID != req.DatasetID {
		return business.RangeWitness{}, fmt.Errorf("dataset_id mismatch: dataset=%q request=%q", ds.DatasetID, req.DatasetID)
	}

	var foundRecord *Record
	for i := range ds.Records {
		if ds.Records[i].RecordID == req.RecordID {
			foundRecord = &ds.Records[i]
			break
		}
	}
	if foundRecord == nil {
		return business.RangeWitness{}, fmt.Errorf("record_id %q not found in dataset", req.RecordID)
	}

	field, ok := foundRecord.Fields[req.FieldName]
	if !ok {
		return business.RangeWitness{}, fmt.Errorf("field %q not found in record %q", req.FieldName, req.RecordID)
	}
	if field.Type != "uint64" {
		return business.RangeWitness{}, fmt.Errorf("field %q type is %q, expected uint64", req.FieldName, field.Type)
	}
	if field.Value < req.Range.MinValue || field.Value > req.Range.MaxValue {
		return business.RangeWitness{}, fmt.Errorf("field value %d outside request range [%d, %d]", field.Value, req.Range.MinValue, req.Range.MaxValue)
	}

	// Build Stage 7 IMT fixture metadata from dataset/request.
	fixture := imt.DefaultFixture()
	fixture.DatasetID = ds.DatasetID
	fixture.FieldName = req.FieldName
	fixture.RecordID = req.RecordID
	fixture.SchemaVersion = ds.SchemaVersion

	return business.RangeWitness{
		RequestHash:     req.RequestHash,
		SessionID:       sessionID,
		RoundIndex:      roundIndex,
		RawValue:        field.Value,
		MinValue:        req.Range.MinValue,
		MaxValue:        req.Range.MaxValue,
		MaskDelta:       field.MaskDelta,
		SaltHex:         field.SaltHex,
		MaskedValueHash: "", // business-fixture computes this
		IMT:             fixture,
	}, nil
}
