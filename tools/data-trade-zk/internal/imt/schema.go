package imt

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Fixture defines a minimal canonical IMT fixture for the range business
// witness path. It describes how a committed leaf is positioned in a
// deterministic Merkle tree of fixed depth.
type Fixture struct {
	Version       int    `json:"version"`
	Depth         int    `json:"depth"`
	LeafIndex     int    `json:"leaf_index"`
	RootListIndex int    `json:"root_list_index"`
	DatasetID     string `json:"dataset_id"`
	FieldName     string `json:"field_name"`
}

// DefaultFixture returns the default IMT fixture used when the witness
// JSON omits the "imt" field.
func DefaultFixture() Fixture {
	return Fixture{
		Version:       1,
		Depth:         10,
		LeafIndex:     0,
		RootListIndex: 0,
		DatasetID:     "demo-range-dataset",
		FieldName:     "sensor_value",
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
func (f Fixture) Validate() error {
	if f.Version != 1 {
		return fmt.Errorf("imt version must be 1, got %d", f.Version)
	}
	if f.Depth != 10 {
		return fmt.Errorf("imt depth must be 10 in Stage 6, got %d", f.Depth)
	}
	if f.LeafIndex != 0 {
		return fmt.Errorf("imt leaf_index must be 0 in Stage 6, got %d", f.LeafIndex)
	}
	if f.RootListIndex != 0 {
		return fmt.Errorf("imt root_list_index must be 0 in Stage 6, got %d", f.RootListIndex)
	}
	if strings.TrimSpace(f.DatasetID) == "" {
		return fmt.Errorf("imt dataset_id must not be empty")
	}
	if !isASCII(f.DatasetID) {
		return fmt.Errorf("imt dataset_id must be ASCII")
	}
	if strings.TrimSpace(f.FieldName) == "" {
		return fmt.Errorf("imt field_name must not be empty")
	}
	if !isASCII(f.FieldName) {
		return fmt.Errorf("imt field_name must be ASCII")
	}
	return nil
}

// ReadFixture reads and validates an IMT fixture from a JSON file.
func ReadFixture(path string) (Fixture, error) {
	var f Fixture
	b, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, err
	}
	if err := f.Validate(); err != nil {
		return f, err
	}
	return f, nil
}
