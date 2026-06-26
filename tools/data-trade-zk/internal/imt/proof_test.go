package imt

import (
	"bytes"
	"testing"
)

const testCurve = "BN254"

func dummyHash() []byte {
	// 32-byte deterministic hash for testing.
	return bytes.Repeat([]byte{0xab}, 32)
}

func TestPrepareProofSameInputProducesSameOutput(t *testing.T) {
	f := DefaultFixture()
	mvh := dummyHash()
	pp1, err := PrepareProof(testCurve, mvh, f)
	if err != nil {
		t.Fatalf("PrepareProof: %v", err)
	}
	pp2, err := PrepareProof(testCurve, mvh, f)
	if err != nil {
		t.Fatalf("PrepareProof: %v", err)
	}
	if !bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("same input produced different Root0")
	}
	for i := range pp1.Path {
		if !bytes.Equal(pp1.Path[i], pp2.Path[i]) {
			t.Fatalf("same input produced different Path[%d]", i)
		}
	}
}

func TestPrepareProofChangingMaskedValueHashChangesRoot0(t *testing.T) {
	f := DefaultFixture()
	mvh1 := bytes.Repeat([]byte{0xab}, 32)
	mvh2 := bytes.Repeat([]byte{0xcd}, 32)
	pp1, _ := PrepareProof(testCurve, mvh1, f)
	pp2, _ := PrepareProof(testCurve, mvh2, f)
	if bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("changing masked_value_hash did not change Root0")
	}
}

func TestPrepareProofChangingDatasetIDChangesRoot(t *testing.T) {
	f1 := DefaultFixture()
	f2 := DefaultFixture()
	f2.DatasetID = "other-dataset"
	mvh := dummyHash()
	pp1, _ := PrepareProof(testCurve, mvh, f1)
	pp2, _ := PrepareProof(testCurve, mvh, f2)
	// Padding leaves include fixture metadata, so changing dataset_id
	// must change the selected Root0.
	if bytes.Equal(pp1.Root0, pp2.Root0) {
		t.Fatal("changing dataset_id did not change Root0")
	}
}

func TestPrepareProofPathLengthIs10(t *testing.T) {
	f := DefaultFixture()
	pp, err := PrepareProof(testCurve, dummyHash(), f)
	if err != nil {
		t.Fatalf("PrepareProof: %v", err)
	}
	if len(pp.Path) != 10 {
		t.Fatalf("expected path length 10, got %d", len(pp.Path))
	}
}

func TestPrepareProofRejectsInvalidFixture(t *testing.T) {
	f := DefaultFixture()
	f.Depth = 5
	_, err := PrepareProof(testCurve, dummyHash(), f)
	if err == nil {
		t.Fatal("expected error for invalid fixture")
	}
}
