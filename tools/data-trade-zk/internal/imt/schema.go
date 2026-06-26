package imt

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Fixture defines a canonical IMT fixture. Stage 6 fields (Depth, LeafIndex)
// remain as deprecated aliases for PublishedDepth and PublishedLeafIndex
// respectively; all new Stage 7 structured fields are added.
type Fixture struct {
	Version            int    `json:"version"`
	Depth              int    `json:"depth,omitempty"`
	LeafIndex          int    `json:"leaf_index,omitempty"`
	RootListIndex      int    `json:"root_list_index"`
	DatasetID          string `json:"dataset_id"`
	FieldName          string `json:"field_name"`
	RecordID           string `json:"record_id,omitempty"`
	SchemaVersion      int    `json:"schema_version,omitempty"`
	EntryDepth         int    `json:"entry_depth,omitempty"`
	DatasetDepth       int    `json:"dataset_depth,omitempty"`
	AggregateDepth     int    `json:"aggregate_depth,omitempty"`
	PublishedDepth     int    `json:"published_depth,omitempty"`
	EntryIndex         int    `json:"entry_index,omitempty"`
	DatasetIndex       int    `json:"dataset_index,omitempty"`
	AggregateIndex     int    `json:"aggregate_index,omitempty"`
	PublishedLeafIndex int    `json:"published_leaf_index,omitempty"`
}

// rawFixture is a mirror struct used for detecting omitted keys via
// json.RawMessage before defaulting and validation.
type rawFixture struct {
	Version            *int    `json:"version"`
	Depth              *int    `json:"depth"`
	LeafIndex          *int    `json:"leaf_index"`
	RootListIndex      *int    `json:"root_list_index"`
	DatasetID          *string `json:"dataset_id"`
	FieldName          *string `json:"field_name"`
	RecordID           *string `json:"record_id"`
	SchemaVersion      *int    `json:"schema_version"`
	EntryDepth         *int    `json:"entry_depth"`
	DatasetDepth       *int    `json:"dataset_depth"`
	AggregateDepth     *int    `json:"aggregate_depth"`
	PublishedDepth     *int    `json:"published_depth"`
	EntryIndex         *int    `json:"entry_index"`
	DatasetIndex       *int    `json:"dataset_index"`
	AggregateIndex     *int    `json:"aggregate_index"`
	PublishedLeafIndex *int    `json:"published_leaf_index"`
}

// DefaultFixture returns the default IMT fixture used when the witness
// JSON omits the "imt" field.
func DefaultFixture() Fixture {
	return Fixture{
		Version:            1,
		Depth:              10,
		LeafIndex:          0,
		RootListIndex:      0,
		DatasetID:          "demo-range-dataset",
		FieldName:          "sensor_value",
		RecordID:           "demo-record-0",
		SchemaVersion:      1,
		EntryDepth:         4,
		DatasetDepth:       4,
		AggregateDepth:     2,
		PublishedDepth:     10,
		EntryIndex:         0,
		DatasetIndex:       0,
		AggregateIndex:     0,
		PublishedLeafIndex: 0,
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// Validate returns an error if the fixture fails validation rules.
// It expects that defaulting has already been applied (via DefaultFixture
// or defaultFromRaw).
func (f Fixture) Validate() error {
	if f.Version != 1 {
		return fmt.Errorf("imt version must be 1, got %d", f.Version)
	}
	// Depth and LeafIndex are deprecated aliases; actual validation is on
	// PublishedDepth and PublishedLeafIndex.
	if f.PublishedDepth != 10 {
		return fmt.Errorf("published_depth must be 10, got %d", f.PublishedDepth)
	}
	if f.EntryDepth != 4 {
		return fmt.Errorf("entry_depth must be 4, got %d", f.EntryDepth)
	}
	if f.DatasetDepth != 4 {
		return fmt.Errorf("dataset_depth must be 4, got %d", f.DatasetDepth)
	}
	if f.AggregateDepth != 2 {
		return fmt.Errorf("aggregate_depth must be 2, got %d", f.AggregateDepth)
	}
	if f.PublishedLeafIndex != 0 {
		return fmt.Errorf("published_leaf_index must be 0, got %d", f.PublishedLeafIndex)
	}
	if f.RootListIndex != 0 {
		return fmt.Errorf("root_list_index must be 0, got %d", f.RootListIndex)
	}
	if f.EntryIndex != 0 {
		return fmt.Errorf("entry_index must be 0, got %d", f.EntryIndex)
	}
	if f.DatasetIndex != 0 {
		return fmt.Errorf("dataset_index must be 0, got %d", f.DatasetIndex)
	}
	if f.AggregateIndex != 0 {
		return fmt.Errorf("aggregate_index must be 0, got %d", f.AggregateIndex)
	}
	if f.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1, got %d", f.SchemaVersion)
	}
	if strings.TrimSpace(f.DatasetID) == "" {
		return fmt.Errorf("dataset_id must not be empty")
	}
	if !isASCII(f.DatasetID) {
		return fmt.Errorf("dataset_id must be ASCII")
	}
	if strings.TrimSpace(f.FieldName) == "" {
		return fmt.Errorf("field_name must not be empty")
	}
	if !isASCII(f.FieldName) {
		return fmt.Errorf("field_name must be ASCII")
	}
	if strings.TrimSpace(f.RecordID) == "" {
		return fmt.Errorf("record_id must not be empty")
	}
	if !isASCII(f.RecordID) {
		return fmt.Errorf("record_id must be ASCII")
	}
	return nil
}

// defaultFromRaw applies default values for omitted JSON keys.
// Keys present in raw are kept as-is; omitted keys are filled from the
// default fixture. If deprecated alias keys conflict with new keys, it
// returns an error.
func defaultFromRaw(raw map[string]json.RawMessage, f *Fixture, def Fixture) error {
	// Detect which keys are present.
	has := func(key string) bool {
		_, ok := raw[key]
		return ok
	}

	// Handle deprecated aliases: depth -> published_depth, leaf_index -> published_leaf_index.
	if has("depth") && has("published_depth") {
		if f.Depth != f.PublishedDepth {
			return fmt.Errorf("depth (%d) and published_depth (%d) conflict", f.Depth, f.PublishedDepth)
		}
	}
	if has("depth") && !has("published_depth") {
		f.PublishedDepth = f.Depth
	}

	if has("leaf_index") && has("published_leaf_index") {
		if f.LeafIndex != f.PublishedLeafIndex {
			return fmt.Errorf("leaf_index (%d) and published_leaf_index (%d) conflict", f.LeafIndex, f.PublishedLeafIndex)
		}
	}
	if has("leaf_index") && !has("published_leaf_index") {
		f.PublishedLeafIndex = f.LeafIndex
	}

	// Default omitted fields. Fields already set via deprecated aliases
	// must not be overwritten by defaults.
	publishedDepthFromAlias := has("depth") && !has("published_depth")
	publishedLeafFromAlias := has("leaf_index") && !has("published_leaf_index")

	if !has("record_id") {
		f.RecordID = def.RecordID
	}
	if !has("schema_version") {
		f.SchemaVersion = def.SchemaVersion
	}
	if !has("entry_depth") {
		f.EntryDepth = def.EntryDepth
	}
	if !has("dataset_depth") {
		f.DatasetDepth = def.DatasetDepth
	}
	if !has("aggregate_depth") {
		f.AggregateDepth = def.AggregateDepth
	}
	if !has("published_depth") && !publishedDepthFromAlias {
		f.PublishedDepth = def.PublishedDepth
	}
	if !has("entry_index") {
		f.EntryIndex = def.EntryIndex
	}
	if !has("dataset_index") {
		f.DatasetIndex = def.DatasetIndex
	}
	if !has("aggregate_index") {
		f.AggregateIndex = def.AggregateIndex
	}
	if !has("published_leaf_index") && !publishedLeafFromAlias {
		f.PublishedLeafIndex = def.PublishedLeafIndex
	}
	// Always default deprecated aliases to match their canonical counterparts.
	f.Depth = f.PublishedDepth
	f.LeafIndex = f.PublishedLeafIndex
	return nil
}

// UnmarshalFixtureJSON parses a JSON byte slice into a Fixture, applying
// default values for omitted fields and rejecting invalid explicit values.
func UnmarshalFixtureJSON(b []byte, def Fixture) (Fixture, error) {
	var f Fixture
	// First pass: detect which keys exist.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return f, err
	}
	// Second pass: unmarshal to struct.
	if err := json.Unmarshal(b, &f); err != nil {
		return f, err
	}
	// Apply defaults for omitted keys.
	if err := defaultFromRaw(raw, &f, def); err != nil {
		return f, err
	}
	// Validate.
	if err := f.Validate(); err != nil {
		return f, err
	}
	return f, nil
}

// ReadFixture reads and validates an IMT fixture from a JSON file.
func ReadFixture(path string) (Fixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, err
	}
	return UnmarshalFixtureJSON(b, DefaultFixture())
}
