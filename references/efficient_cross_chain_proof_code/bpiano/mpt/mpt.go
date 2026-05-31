// Package mpt provides a simple binary Merkle Patricia Trie for demonstration.
package mpt

import (
	"github.com/oliverustc/bpiano/keccak"
)

// PathLevel represents one level in an MPT path.
type PathLevel struct {
	Left   [32]byte // left child hash
	Right  [32]byte // right child hash
	Parent [32]byte // hash of left||right
	IsLeft bool     // are we on the left branch?
}

// Proof is an MPT membership proof.
type Proof struct {
	LeafData []byte
	Levels   []PathLevel // from leaf to root
	RootHash [32]byte
}

// nodeInput builds the 64-byte input for one MPT node hash: left_hash || right_hash.
func nodeInput(left, right [32]byte) [64]byte {
	var buf [64]byte
	copy(buf[:32], left[:])
	copy(buf[32:], right[:])
	return buf
}

// Build creates an MPT proof for a leaf.
// siblings: the sibling hash at each level (from leaf-level to root-level).
// directions: true = we are on the left side at this level.
func Build(leafData []byte, siblings [][32]byte, directions []bool) Proof {
	if len(siblings) != len(directions) {
		panic("mpt: siblings and directions must have same length")
	}

	// Hash the leaf data to get the leaf hash.
	leafHash := keccak.Hash256(leafData)

	levels := make([]PathLevel, len(siblings))
	current := leafHash

	for i, sibling := range siblings {
		var level PathLevel
		level.IsLeft = directions[i]
		if directions[i] {
			// We are on the left: our hash is left, sibling is right
			level.Left = current
			level.Right = sibling
		} else {
			// We are on the right: sibling is left, our hash is right
			level.Left = sibling
			level.Right = current
		}
		input := nodeInput(level.Left, level.Right)
		level.Parent = keccak.Hash256(input[:])
		levels[i] = level
		current = level.Parent
	}

	return Proof{
		LeafData: leafData,
		Levels:   levels,
		RootHash: current,
	}
}
