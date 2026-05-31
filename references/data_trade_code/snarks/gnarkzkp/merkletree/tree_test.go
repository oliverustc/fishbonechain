package merkletree

import (
	"gnarkabc/logger"
	"gnarkabc/utils"
	"math"
	"testing"
)

func TestRandTree(t *testing.T) {
	depth := 2
	tree := New("BN254", 2, depth)
	// var leavesBytes [][]byte
	leafNum := int(math.Pow(2, float64(depth)))
	leavesBytes := make([][]byte, leafNum)
	for i := 0; i < leafNum; i++ {
		leavesBytes[i] = []byte(utils.RandStr(2))
	}
	tree.SetLeaves(leavesBytes)
	tree.ParseLeaves()
	tree.ConstructTree()
	tree.PrintLeaves()

	proofSet := tree.GenerateMerkleProof(0)
	logger.Info("proofSet: %x", proofSet)
	logger.Info("proofSet length: %d", len(proofSet))
	logger.Info("root: %v", tree.Root)
	logger.Info("root child nodes %v", tree.Root.Children)

	proofSet2 := tree.GenerateMerkleProof2(0)
	logger.Info("proofSet2: %x", proofSet2)
	logger.Info("proofSet2 length: %d", len(proofSet2))
	tree.VerifyMerkleProof2(0, leavesBytes[0], proofSet2)
}
