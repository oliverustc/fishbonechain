package imt

import (
	"encoding/json"
	"testing"
)

func TestDefaultFixtureValidates(t *testing.T) {
	f := DefaultFixture()
	if err := f.Validate(); err != nil {
		t.Fatalf("default fixture must validate: %v", err)
	}
}

func TestPublishedDepthMustBe10(t *testing.T) {
	f := DefaultFixture()
	f.PublishedDepth = 8
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for published_depth != 10")
	}
}

func TestEntryDepthMustBe4(t *testing.T) {
	f := DefaultFixture()
	f.EntryDepth = 3
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for entry_depth != 4")
	}
}

func TestDatasetDepthMustBe4(t *testing.T) {
	f := DefaultFixture()
	f.DatasetDepth = 5
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for dataset_depth != 4")
	}
}

func TestAggregateDepthMustBe2(t *testing.T) {
	f := DefaultFixture()
	f.AggregateDepth = 3
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for aggregate_depth != 2")
	}
}

func TestPublishedLeafIndexMustBe0(t *testing.T) {
	f := DefaultFixture()
	f.PublishedLeafIndex = 1
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for published_leaf_index != 0")
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

func TestEmptyRecordIDRejects(t *testing.T) {
	f := DefaultFixture()
	f.RecordID = ""
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for empty record_id")
	}
}

func TestVersionMustBe1(t *testing.T) {
	f := DefaultFixture()
	f.Version = 2
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for version != 1")
	}
}

func TestSchemaVersionMustBe1(t *testing.T) {
	f := DefaultFixture()
	f.SchemaVersion = 2
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for schema_version != 1")
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

func TestNonASCIIRecordIDRejects(t *testing.T) {
	f := DefaultFixture()
	f.RecordID = "记录"
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for non-ASCII record_id")
	}
}

func TestRootListIndexMustBe0(t *testing.T) {
	f := DefaultFixture()
	f.RootListIndex = 1
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for root_list_index != 0")
	}
}

func TestEntryIndexMustBe0(t *testing.T) {
	f := DefaultFixture()
	f.EntryIndex = 1
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for entry_index != 0")
	}
}

func TestStage6FixtureDefaultsStage7Fields(t *testing.T) {
	// Old Stage 6 JSON with only the old fields should default new fields.
	b := []byte(`{"version":1,"depth":10,"leaf_index":0,"root_list_index":0,"dataset_id":"test","field_name":"f"}`)
	f, err := UnmarshalFixtureJSON(b, DefaultFixture())
	if err != nil {
		t.Fatalf("Stage 6 fixture should default: %v", err)
	}
	if f.PublishedDepth != 10 {
		t.Fatalf("expected published_depth 10, got %d", f.PublishedDepth)
	}
	if f.RecordID != "demo-record-0" {
		t.Fatalf("expected default record_id, got %q", f.RecordID)
	}
}

func TestDepthAliasCopiesToPublishedDepth(t *testing.T) {
	b := []byte(`{"version":1,"depth":10,"root_list_index":0,"dataset_id":"test","field_name":"f"}`)
	f, err := UnmarshalFixtureJSON(b, DefaultFixture())
	if err != nil {
		t.Fatalf("depth alias should copy: %v", err)
	}
	if f.PublishedDepth != 10 {
		t.Fatalf("expected published_depth=10, got %d", f.PublishedDepth)
	}
}

func TestDepthAndPublishedDepthConflictRejects(t *testing.T) {
	b := []byte(`{"version":1,"depth":10,"published_depth":8,"root_list_index":0,"dataset_id":"test","field_name":"f"}`)
	_, err := UnmarshalFixtureJSON(b, DefaultFixture())
	if err == nil {
		t.Fatal("expected conflict between depth and published_depth to reject")
	}
}

func TestOmittedIMTFieldsReject(t *testing.T) {
	// Empty JSON must reject because version is missing (not defaulted).
	b := []byte(`{}`)
	_, err := UnmarshalFixtureJSON(b, DefaultFixture())
	if err == nil {
		t.Fatal("expected missing version to reject")
	}
}

func TestExplicitInvalidZeroRejects(t *testing.T) {
	b := []byte(`{"version":1,"published_depth":0,"root_list_index":0,"dataset_id":"test","field_name":"f"}`)
	_, err := UnmarshalFixtureJSON(b, DefaultFixture())
	if err == nil {
		t.Fatal("expected explicit published_depth=0 to reject")
	}
}

func TestUnmarshalFixtureJSONRoundTripsDefault(t *testing.T) {
	def := DefaultFixture()
	b, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	f, err := UnmarshalFixtureJSON(b, DefaultFixture())
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f.PublishedDepth != def.PublishedDepth || f.RecordID != def.RecordID {
		t.Fatal("round-trip mismatch")
	}
}
