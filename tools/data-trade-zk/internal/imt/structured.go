package imt

import (
	"fmt"
)

const (
	domainEntry        = "FISHBONE:DATA_TRADE:IMT:ENTRY:v1"
	domainEntryPad     = "FISHBONE:DATA_TRADE:IMT:ENTRY_PAD:v1"
	domainDataset      = "FISHBONE:DATA_TRADE:IMT:DATASET:v1"
	domainDatasetPad   = "FISHBONE:DATA_TRADE:IMT:DATASET_PAD:v1"
	domainAggregate    = "FISHBONE:DATA_TRADE:IMT:AGGREGATE:v1"
	domainAggregatePad = "FISHBONE:DATA_TRADE:IMT:AGGREGATE_PAD:v1"
	domainPublishedPad = "FISHBONE:DATA_TRADE:IMT:PUBLISHED_PAD:v1"
)

func labelString(domain, datasetID, fieldName, recordID string, idx int) string {
	return fmt.Sprintf("%s%c%s%c%s%c%s%c%d",
		domain, separator, datasetID, separator, fieldName, separator, recordID, separator, idx)
}

// buildDeterministicTree builds a balanced binary Merkle tree of the given
// depth from the given leaves. The curve+hash pair functions must be provided
// (they are defined in proof.go).
func buildDeterministicTree(curveName string, leaves [][]byte, depth int) ([][]byte, error) {
	if len(leaves) != 1<<depth {
		return nil, fmt.Errorf("expected %d leaves for depth %d, got %d", 1<<depth, depth, len(leaves))
	}
	nodes := make([][]byte, len(leaves))
	copy(nodes, leaves)
	for level := 0; level < depth; level++ {
		step := 1 << level
		for i := 0; i < len(nodes); i += 2 * step {
			left := nodes[i]
			right := nodes[i+step]
			parent, err := deterministicMiMCPair(curveName, left, right)
			if err != nil {
				return nil, fmt.Errorf("tree level %d idx %d: %w", level, i, err)
			}
			nodes[i] = parent
		}
	}
	return nodes, nil
}

// buildEntryLeaf builds the entry leaf: MiMC(domain || dataset_id || field_name || record_id || schema_version || masked_value_hash).
func buildEntryLeaf(curveName string, f Fixture, maskedValueHash []byte) ([]byte, error) {
	label := []byte(fmt.Sprintf("%s%c%s%c%s%c%s%c%d",
		domainEntry, separator, f.DatasetID, separator, f.FieldName, separator, f.RecordID, separator, f.SchemaVersion))
	leaf, err := deterministicMiMC(curveName, label)
	if err != nil {
		return nil, fmt.Errorf("entry leaf: %w", err)
	}
	// Second hash: H(leaf || masked_value_hash)
	return deterministicMiMCPair(curveName, leaf, maskedValueHash)
}

// buildEntryRoot builds the entry Merkle tree and returns the root and proof
// path for entry_index. Padding leaves are derived from fixture metadata.
func buildEntryRoot(curveName string, f Fixture, entryLeaf []byte) (root []byte, path [][]byte, err error) {
	leafCount := 1 << f.EntryDepth
	leaves := make([][]byte, leafCount)
	leaves[f.EntryIndex] = entryLeaf
	for i := 0; i < leafCount; i++ {
		if i == f.EntryIndex {
			continue
		}
		label := []byte(labelString(domainEntryPad, f.DatasetID, f.FieldName, f.RecordID, i))
		pad, e := deterministicMiMC(curveName, label)
		if e != nil {
			return nil, nil, fmt.Errorf("entry pad %d: %w", i, e)
		}
		leaves[i] = pad
	}

	nodes, err := buildDeterministicTree(curveName, leaves, f.EntryDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("entry tree: %w", err)
	}

	// Extract proof path for entry_index.
	path = make([][]byte, f.EntryDepth)
	idx := f.EntryIndex
	for level := 0; level < f.EntryDepth; level++ {
		step := 1 << level
		siblingIdx := idx ^ step
		sibling := nodes[siblingIdx]
		path[level] = make([]byte, len(sibling))
		copy(path[level], sibling)
		idx &= ^step // move to parent position in flattened array
	}

	return nodes[0], path, nil
}

// buildDatasetRoot builds the dataset Merkle tree from entry roots and returns
// the dataset root with proof path for dataset_index.
func buildDatasetRoot(curveName string, f Fixture, entryRoot []byte) (root []byte, path [][]byte, err error) {
	// Dataset node = MiMC(domain || dataset_id || schema_version || entry_root)
	label := []byte(fmt.Sprintf("%s%c%s%c%d",
		domainDataset, separator, f.DatasetID, separator, f.SchemaVersion))
	nodeSeed, err := deterministicMiMC(curveName, label)
	if err != nil {
		return nil, nil, fmt.Errorf("dataset seed: %w", err)
	}
	datasetNode, err := deterministicMiMCPair(curveName, nodeSeed, entryRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("dataset node: %w", err)
	}

	leafCount := 1 << f.DatasetDepth
	leaves := make([][]byte, leafCount)
	leaves[f.DatasetIndex] = datasetNode
	for i := 0; i < leafCount; i++ {
		if i == f.DatasetIndex {
			continue
		}
		label := []byte(labelString(domainDatasetPad, f.DatasetID, f.FieldName, f.RecordID, i))
		pad, e := deterministicMiMC(curveName, label)
		if e != nil {
			return nil, nil, fmt.Errorf("dataset pad %d: %w", i, e)
		}
		leaves[i] = pad
	}

	nodes, err := buildDeterministicTree(curveName, leaves, f.DatasetDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("dataset tree: %w", err)
	}

	path = make([][]byte, f.DatasetDepth)
	idx := f.DatasetIndex
	for level := 0; level < f.DatasetDepth; level++ {
		step := 1 << level
		siblingIdx := idx ^ step
		sibling := nodes[siblingIdx]
		path[level] = make([]byte, len(sibling))
		copy(path[level], sibling)
		idx &= ^step
	}

	return nodes[0], path, nil
}

// buildAggregateRoot builds the aggregate Merkle tree from dataset roots.
func buildAggregateRoot(curveName string, f Fixture, datasetRoot []byte) (root []byte, path [][]byte, err error) {
	// Aggregate node = MiMC(domain || dataset_id || dataset_root)
	label := []byte(fmt.Sprintf("%s%c%s",
		domainAggregate, separator, f.DatasetID))
	seed, err := deterministicMiMC(curveName, label)
	if err != nil {
		return nil, nil, fmt.Errorf("aggregate seed: %w", err)
	}
	aggNode, err := deterministicMiMCPair(curveName, seed, datasetRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("aggregate node: %w", err)
	}

	leafCount := 1 << f.AggregateDepth
	leaves := make([][]byte, leafCount)
	leaves[f.AggregateIndex] = aggNode
	for i := 0; i < leafCount; i++ {
		if i == f.AggregateIndex {
			continue
		}
		label := []byte(labelString(domainAggregatePad, f.DatasetID, f.FieldName, f.RecordID, i))
		pad, e := deterministicMiMC(curveName, label)
		if e != nil {
			return nil, nil, fmt.Errorf("aggregate pad %d: %w", i, e)
		}
		leaves[i] = pad
	}

	nodes, err := buildDeterministicTree(curveName, leaves, f.AggregateDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("aggregate tree: %w", err)
	}

	path = make([][]byte, f.AggregateDepth)
	idx := f.AggregateIndex
	for level := 0; level < f.AggregateDepth; level++ {
		step := 1 << level
		siblingIdx := idx ^ step
		sibling := nodes[siblingIdx]
		path[level] = make([]byte, len(sibling))
		copy(path[level], sibling)
		idx &= ^step
	}

	return nodes[0], path, nil
}

// buildPublishedRoot builds the published Merkle tree from the aggregate root
// and returns the final PreparedProof for RO proof assignment. It extracts
// the Merkle proof path during tree construction because the tree nodes are
// overwritten in-place by the compression pass.
func buildPublishedRoot(curveName string, f Fixture, aggregateRoot []byte) (root []byte, path [][]byte, err error) {
	leafCount := 1 << f.PublishedDepth
	leaves := make([][]byte, leafCount)
	leaves[f.PublishedLeafIndex] = aggregateRoot
	for i := 0; i < leafCount; i++ {
		if i == f.PublishedLeafIndex {
			continue
		}
		label := []byte(labelString(domainPublishedPad, f.DatasetID, f.FieldName, f.RecordID, i))
		pad, e := deterministicMiMC(curveName, label)
		if e != nil {
			return nil, nil, fmt.Errorf("published pad %d: %w", i, e)
		}
		leaves[i] = pad
	}

	nodes := make([][]byte, leafCount)
	copy(nodes, leaves)
	path = make([][]byte, f.PublishedDepth)
	for level := 0; level < f.PublishedDepth; level++ {
		step := 1 << level
		for i := 0; i < leafCount; i += 2 * step {
			left := nodes[i]
			right := nodes[i+step]
			parent, e := deterministicMiMCPair(curveName, left, right)
			if e != nil {
				return nil, nil, fmt.Errorf("published tree level %d: %w", level, e)
			}
			nodes[i] = parent
		}
		// Sibling for leaf at published_leaf_index.
		sibling := nodes[f.PublishedLeafIndex^step]
		path[level] = make([]byte, len(sibling))
		copy(path[level], sibling)
	}

	return nodes[0], path, nil
}

// PrepareStructuredProof builds the full four-layer structured IMT proof.
func PrepareStructuredProof(curveName string, maskedValueHash []byte, f Fixture) (PreparedProof, error) {
	if err := f.Validate(); err != nil {
		return PreparedProof{}, err
	}

	// Entry layer
	entryLeaf, err := buildEntryLeaf(curveName, f, maskedValueHash)
	if err != nil {
		return PreparedProof{}, err
	}
	entryRoot, _, err := buildEntryRoot(curveName, f, entryLeaf)
	if err != nil {
		return PreparedProof{}, err
	}

	// Dataset layer
	datasetRoot, _, err := buildDatasetRoot(curveName, f, entryRoot)
	if err != nil {
		return PreparedProof{}, err
	}

	// Aggregate layer
	aggregateRoot, _, err := buildAggregateRoot(curveName, f, datasetRoot)
	if err != nil {
		return PreparedProof{}, err
	}

	// Published layer → RO proof inputs
	publishedRoot, path, err := buildPublishedRoot(curveName, f, aggregateRoot)
	if err != nil {
		return PreparedProof{}, err
	}

	// Decoy roots: still derived from fixture metadata.
	decoyRoots := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		label := []byte(decoyString(domainDecoy, f.DatasetID, f.FieldName, i))
		dr, err := deterministicMiMC(curveName, label)
		if err != nil {
			return PreparedProof{}, fmt.Errorf("decoy root %d: %w", i, err)
		}
		decoyRoots[i] = dr
	}

	return PreparedProof{
		Leaf:          aggregateRoot,
		Path:          path,
		Root0:         publishedRoot,
		Root1:         decoyRoots[0],
		Root2:         decoyRoots[1],
		Root3:         decoyRoots[2],
		Index0:        0,
		Index1:        0,
		EntryRoot:     entryRoot,
		DatasetRoot:   datasetRoot,
		AggregateRoot: aggregateRoot,
		PublishedRoot: publishedRoot,
	}, nil
}
