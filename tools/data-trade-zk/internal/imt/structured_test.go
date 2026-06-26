package imt

import (
	"bytes"
	"testing"
)

func TestStructuredProofSameInputProducesSameOutput(t *testing.T) {
	f := DefaultFixture()
	mvh := dummyHash()
	pp1, err := PrepareStructuredProof(testCurve, mvh, f)
	if err != nil {
		t.Fatalf("PrepareStructuredProof: %v", err)
	}
	pp2, err := PrepareStructuredProof(testCurve, mvh, f)
	if err != nil {
		t.Fatalf("PrepareStructuredProof: %v", err)
	}
	if !bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("same input produced different Root0")
	}
	if !bytes.Equal(pp1.EntryRoot, pp2.EntryRoot) {
		t.Fatal("same input produced different EntryRoot")
	}
}

func TestStructuredProofChangingMaskedValueHashChangesDownstreamRoots(t *testing.T) {
	f := DefaultFixture()
	mvh1 := bytes.Repeat([]byte{0xab}, 32)
	mvh2 := bytes.Repeat([]byte{0xcd}, 32)
	pp1, _ := PrepareStructuredProof(testCurve, mvh1, f)
	pp2, _ := PrepareStructuredProof(testCurve, mvh2, f)
	if bytes.Equal(pp1.EntryRoot, pp2.EntryRoot) {
		t.Fatal("changing masked_value_hash did not change EntryRoot")
	}
	if bytes.Equal(pp1.DatasetRoot, pp2.DatasetRoot) {
		t.Fatal("changing masked_value_hash did not change DatasetRoot")
	}
	if bytes.Equal(pp1.AggregateRoot, pp2.AggregateRoot) {
		t.Fatal("changing masked_value_hash did not change AggregateRoot")
	}
	if bytes.Equal(pp1.PublishedRoot, pp2.PublishedRoot) {
		t.Fatal("changing masked_value_hash did not change PublishedRoot")
	}
	if bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("changing masked_value_hash did not change Root0")
	}
}

func TestStructuredProofChangingRecordIDChangesDownstreamRoots(t *testing.T) {
	f1 := DefaultFixture()
	f2 := DefaultFixture()
	f2.RecordID = "other-record"
	mvh := dummyHash()
	pp1, _ := PrepareStructuredProof(testCurve, mvh, f1)
	pp2, _ := PrepareStructuredProof(testCurve, mvh, f2)
	if bytes.Equal(pp1.EntryRoot, pp2.EntryRoot) {
		t.Fatal("changing record_id did not change EntryRoot")
	}
	if bytes.Equal(pp1.DatasetRoot, pp2.DatasetRoot) {
		t.Fatal("changing record_id did not change DatasetRoot")
	}
	if bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("changing record_id did not change Root0")
	}
}

func TestStructuredProofChangingDatasetIDChangesRoots(t *testing.T) {
	f1 := DefaultFixture()
	f2 := DefaultFixture()
	f2.DatasetID = "other-dataset"
	mvh := dummyHash()
	pp1, _ := PrepareStructuredProof(testCurve, mvh, f1)
	pp2, _ := PrepareStructuredProof(testCurve, mvh, f2)
	if bytes.Equal(pp1.DatasetRoot, pp2.DatasetRoot) {
		t.Fatal("changing dataset_id did not change DatasetRoot")
	}
	if bytes.Equal(pp1.AggregateRoot, pp2.AggregateRoot) {
		t.Fatal("changing dataset_id did not change AggregateRoot")
	}
	if bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("changing dataset_id did not change Root0")
	}
}

func TestStructuredProofChangingFieldNameChangesRoots(t *testing.T) {
	f1 := DefaultFixture()
	f2 := DefaultFixture()
	f2.FieldName = "other_field"
	mvh := dummyHash()
	pp1, _ := PrepareStructuredProof(testCurve, mvh, f1)
	pp2, _ := PrepareStructuredProof(testCurve, mvh, f2)
	if bytes.Equal(pp1.EntryRoot, pp2.EntryRoot) {
		t.Fatal("changing field_name did not change EntryRoot")
	}
	if bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("changing field_name did not change Root0")
	}
}

func TestStructuredProofPathLengthIs10(t *testing.T) {
	f := DefaultFixture()
	pp, err := PrepareStructuredProof(testCurve, dummyHash(), f)
	if err != nil {
		t.Fatalf("PrepareStructuredProof: %v", err)
	}
	if len(pp.Path) != 10 {
		t.Fatalf("expected published path length 10, got %d", len(pp.Path))
	}
}
