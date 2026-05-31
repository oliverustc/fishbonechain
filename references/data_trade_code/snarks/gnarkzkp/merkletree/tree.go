package merkletree

import (
	"bytes"
	"fmt"
	"gnarkabc/hash/mimchash"
	"gnarkabc/logger"
	"gnarkabc/utils"
	"hash"
	"math"
	"math/big"
)

/*
多叉Merkle树数据结构
hash.Hash: 哈希函数
Depth: 树的深度
Root: 根节点
MaxChildren: 最大子节点数
*/
type Tree struct {
	hash.Hash
	Field       *big.Int
	Depth       int
	Root        *Node
	MaxChildren int
	LeavesNum   int
	LeavesData  [][]byte
	LeavesList  []*Node
}

func (t *Tree) String() string {
	treeStr := ""
	nodeQueue := []*Node{t.Root}
	for len(nodeQueue) != 0 {
		node := nodeQueue[0]
		nodeQueue = nodeQueue[1:]
		treeStr += fmt.Sprintf("hash: %x\n", node.Hash)
		if len(node.Children) != 0 {
			nodeQueue = append(nodeQueue, node.Children...)
		}

	}
	return treeStr
}

/*
Merkle树节点
Hash: 节点哈希
Children: 子节点
Parent: 父节点
Next: 下一个兄弟节点
*/
type Node struct {
	Hash     []byte
	Children []*Node
	Parent   *Node
	Next     *Node
	Depth    int
}

func (n *Node) String() string {
	childrenPointStr := ""
	for _, child := range n.Children {
		childrenPointStr += fmt.Sprintf("%p ", child)
	}
	return fmt.Sprintf("Node Info:\nself: %p,\tdepth: %d,\thash: %x,\nchildren: %s, \nParent: %p, Next: %p", n, n.Depth, n.Hash[:4], childrenPointStr, n.Parent, n.Next)
}

func New(curveName string, MaxChildren, depth int) *Tree {
	return &Tree{
		Hash:        mimchash.MiMCCaseMap[curveName].Hash,
		Field:       mimchash.MiMCCaseMap[curveName].Curve.ScalarField(),
		MaxChildren: MaxChildren,
		Depth:       depth,
		Root:        nil,
		LeavesNum:   int(math.Pow(float64(MaxChildren), float64(depth))),
	}
}

func (t *Tree) sum(data ...[]byte) []byte {
	t.Hash.Reset()

	for _, d := range data {
		_, err := t.Hash.Write(d)
		if err != nil {
			logger.Fatal("hash.Write failed, %v", err)
		}
	}
	result := t.Hash.Sum(nil)
	logger.Debug("result: %x", result)
	return result
}

func (t *Tree) RandGenerateLeaves() {
	logger.Debug("Random Generate Leaves")
	for range t.LeavesNum {
		leafByteSlice := utils.RandBigInt(t.Field)
		logger.Debug("LeafData: %x", leafByteSlice)
		t.LeavesData = append(t.LeavesData, leafByteSlice)
	}
}

func (t *Tree) ParseLeaves() {
	logger.Debug("Parse LeavesData to LeavesNode")
	t.LeavesList = make([]*Node, t.LeavesNum)
	for i := range t.LeavesNum {
		t.LeavesList[i] = &Node{
			Hash:     t.sum(t.LeavesData[i]),
			Depth:    0,
			Children: nil,
			Parent:   nil,
			Next:     nil,
		}
		logger.Debug("leaf[%d]: %x", i, t.LeavesList[i].Hash)
	}
}

func (t *Tree) CalcParentHash(children []*Node) []byte {
	if len(children) != t.MaxChildren {
		logger.Fatal("Tree is NOT Full !")
	}
	logger.Debug("Calculate Parent Hash")
	var childrenHash [][]byte
	for _, child := range children {
		childrenHash = append(childrenHash, child.Hash)
	}
	return t.sum(childrenHash...)
}

func (t *Tree) ContractMiddleNode(depth int, children []*Node) []*Node {
	nodeNum := int(math.Pow(float64(t.MaxChildren), float64(depth)))
	if len(children)%t.MaxChildren != 0 {
		logger.Fatal("Tree is Not Full !")
	}
	logger.Debug("Contract %d Nodes into upper %d Middle Node", len(children), nodeNum)
	middleNodes := make([]*Node, nodeNum)
	for i := 0; i < nodeNum; i++ {
		logger.Debug("%d-th middle node", i+1)
		childrenNodes := make([]*Node, t.MaxChildren)
		for j := 0; j < t.MaxChildren; j++ {
			childrenNodes[j] = children[j*t.MaxChildren+i]
		}
		middleNodes[i] = &Node{
			Hash:     t.CalcParentHash(childrenNodes),
			Children: childrenNodes,
			Depth:    depth,
			Parent:   nil,
			Next:     nil,
		}
		for j := 0; j < t.MaxChildren; j++ {
			childrenNodes[j].Parent = middleNodes[i]
			if j < t.MaxChildren-1 {
				childrenNodes[j].Next = childrenNodes[j+1]
			}
		}
	}
	return middleNodes
}

func (t *Tree) ConstructRoot(children []*Node) {
	if t.Root != nil {
		logger.Fatal("Root Node is NOT Nil !")
	}
	t.Root = &Node{
		Hash:     t.CalcParentHash(children),
		Children: children,
		Parent:   nil,
		Next:     nil,
	}
	for i := 0; i < len(children); i++ {
		children[i].Parent = t.Root
		if i < len(children)-1 {
			children[i].Next = children[i+1]
		}
	}
}

func (t *Tree) RandConstruct() {
	t.RandGenerateLeaves()
	t.ParseLeaves()
	lowerlayerNodes := t.LeavesList
	for i := t.Depth - 1; i > 0; i-- {
		logger.Debug("build the %d layer nodes", i)
		lowerlayerNodes = t.ContractMiddleNode(i, lowerlayerNodes)
	}
	t.ConstructRoot(lowerlayerNodes)
	t.BFS()
}

// 广度优先遍历树
func (t *Tree) BFS() {
	nodeQueue := []*Node{t.Root}
	for len(nodeQueue) != 0 {
		node := nodeQueue[0]
		nodeQueue = nodeQueue[1:]
		logger.Debug(node.String())
		if len(node.Children) != 0 {
			nodeQueue = append(nodeQueue, node.Children...)
		}
	}
}

func (t *Tree) GenerateMerkleProof(index int) (proofSet [][]byte) {
	if index >= t.LeavesNum {
		logger.Fatal("Index Out of Range")
	}
	leaf := t.LeavesList[index]
	for leaf != t.Root {
		nodes := leaf.Parent.Children
		for _, node := range nodes {
			proofSet = append(proofSet, node.Hash)
		}
		leaf = leaf.Parent
	}
	return proofSet
}

// 二叉merkle tree proof 生成
func (t *Tree) GenerateMerkleProof2(index int) (proofSet [][]byte) {
	if index >= t.LeavesNum {
		logger.Fatal("Index Out of Range")
	}
	leaf := t.LeavesList[index]
	for leaf != t.Root {
		nodes := leaf.Parent.Children
		for _, node := range nodes {
			if node != leaf {
				proofSet = append(proofSet, node.Hash)
			}
		}
		leaf = leaf.Parent
	}
	return proofSet
}

func (t *Tree) VerifyMerkleProof(index int, leafhash []byte, proofSet [][]byte) bool {
	if index >= t.LeavesNum {
		logger.Fatal("Index Out of Range")
		return false
	}
	if len(proofSet) != (t.MaxChildren * t.Depth) {
		logger.Fatal("ProofSet Length %d is not enough to %d", len(proofSet), (t.MaxChildren * t.Depth))
		return false
	}
	leaf := t.LeavesList[index]
	if !bytes.Equal(leaf.Hash, leafhash) {
		logger.Info("leafhash:%x is not equal to LeavesList[%d].Hash:%x", leafhash, index, leaf.Hash)
	}
	var parentHash []byte
	for i := 0; i < t.Depth; i++ {
		children := proofSet[:t.MaxChildren]
		proofSet = proofSet[t.MaxChildren:]
		parentHash = t.sum(children...)
		for _, parent := range proofSet {
			if bytes.Equal(parent, parentHash) {
				continue
			}
		}
	}
	if bytes.Equal(parentHash, t.Root.Hash) {
		return true
	} else {
		logger.Error("Final parentHash: %x is not equal root hash: %x", parentHash, t.Root.Hash)
		return false
	}
}

// 二叉merkle tree 的merkle proof 验证
func (t *Tree) VerifyMerkleProof2(index int, leafhash []byte, proofSet [][]byte) bool {
	if index >= t.LeavesNum {
		logger.Fatal("Index Out of Range")
		return false
	}
	if len(proofSet) != (t.Depth) {
		logger.Fatal("ProofSet Length %d is not enough to %d", len(proofSet), (t.Depth))
		return false
	}
	var parentHash []byte
	node := t.LeavesList[index]
	for range t.Depth {
		children := proofSet[0]
		proofSet = proofSet[1:]
		parentHash = t.sum(node.Hash, children)
		node = node.Parent
	}
	if bytes.Equal(parentHash, t.Root.Hash) {
		logger.Info("Proof is valid")
		return true
	} else {
		logger.Error("Final parentHash: %x is not equal root hash: %x", parentHash, t.Root.Hash)
		return false
	}
}

func (t *Tree) SetLeaves(leaves [][]byte) {
	t.LeavesData = leaves
}

func (t *Tree) ConstructTree() {
	t.ParseLeaves()
	lowerlayerNodes := t.LeavesList
	for i := t.Depth - 1; i > 0; i-- {
		lowerlayerNodes = t.ContractMiddleNode(i, lowerlayerNodes)
	}
	t.ConstructRoot(lowerlayerNodes)
}

func (t *Tree) PrintLeaves() {
	for _, node := range t.LeavesList {
		logger.Info(node.String())
	}
}
