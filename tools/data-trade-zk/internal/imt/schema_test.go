package imt

import "testing"

func TestDefaultFixtureValidates(t *testing.T) {
	f := DefaultFixture()
	if err := f.Validate(); err != nil {
		t.Fatalf("default fixture must validate: %v", err)
	}
}

func TestDepthMustBe10(t *testing.T) {
	f := DefaultFixture()
	f.Depth = 8
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for depth != 10")
	}
}

func TestLeafIndexMustBe0(t *testing.T) {
	f := DefaultFixture()
	f.LeafIndex = 1
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for leaf_index != 0")
	}
}

func TestEmptyDatasetIDRejects(t *testing.T) {
	f := DefaultFixture()
	f.DatasetID = ""
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for empty dataset_id")
	}
	f.DatasetID = "   "
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for whitespace-only dataset_id")
	}
}

func TestEmptyFieldNameRejects(t *testing.T) {
	f := DefaultFixture()
	f.FieldName = ""
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for empty field_name")
	}
	f.FieldName = "   "
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for whitespace-only field_name")
	}
}

func TestVersionMustBe1(t *testing.T) {
	f := DefaultFixture()
	f.Version = 2
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for version != 1")
	}
}

func TestNonASCIIDatasetIDRejects(t *testing.T) {
	f := DefaultFixture()
	f.DatasetID = "数据集"
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for non-ASCII dataset_id")
	}
}

func TestNonASCIIFieldNameRejects(t *testing.T) {
	f := DefaultFixture()
	f.FieldName = "字段名"
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for non-ASCII field_name")
	}
}

func TestRootListIndexMustBe0(t *testing.T) {
	f := DefaultFixture()
	f.RootListIndex = 1
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for root_list_index != 0")
	}
}
