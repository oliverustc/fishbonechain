package gnarkadapter

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"

	"fishbone-data-trade-zk/internal/artifact"

	"gnarkabc/gnarkwrapper"
	"gnarkabc/hash/mimchash"
	"gnarkabc/merkletree"
	"gnarkabc/utils"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// ── Range Hash Proof Circuit ────────────────────────────────────────────

type RangeHashProof struct {
	PreImage frontend.Variable
	Hash     frontend.Variable `gnark:",public"`
	Min      frontend.Variable `gnark:",public"`
	Max      frontend.Variable `gnark:",public"`
}

func (c *RangeHashProof) Define(api frontend.API) error {
	m, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	m.Reset()
	m.Write(c.PreImage)
	hash := m.Sum()
	api.AssertIsEqual(hash, c.Hash)
	api.AssertIsLessOrEqual(c.Min, c.PreImage)
	api.AssertIsLessOrEqual(c.PreImage, c.Max)
	return nil
}

func (c *RangeHashProof) PreCompile(params ...interface{}) {}

func (c *RangeHashProof) Assign(curveName string, params ...interface{}) {
	mod := gnarkwrapper.CurveMap[curveName].ScalarField()
	preImageStr := utils.RandStr(3)
	preImage := mimchash.Convert2Byte(preImageStr, mod)
	c.PreImage = preImage
	c.Min = 0
	c.Max = new(big.Int).Sub(mod, new(big.Int).SetInt64(1))
	hashFunc := mimchash.MiMCCaseMap[curveName].Hash
	hash := mimchash.MiMCHash(hashFunc, [][]byte{preImage})
	c.Hash = hash
}

// ── Root Obfuscation Proof Circuit ──────────────────────────────────────

type RootObfuscationProof struct {
	Leaf  frontend.Variable `gnark:",public"`
	Path  []frontend.Variable
	Root0 frontend.Variable `gnark:",public"`
	Root1 frontend.Variable `gnark:",public"`
	Root2 frontend.Variable `gnark:",public"`
	Root3 frontend.Variable `gnark:",public"`

	Index0 frontend.Variable
	Index1 frontend.Variable
}

func (rop *RootObfuscationProof) Define(api frontend.API) error {
	h, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	var sum frontend.Variable
	node := rop.Leaf
	for len(rop.Path) != 0 {
		h.Reset()
		brotherHash := rop.Path[0]
		rop.Path = rop.Path[1:]
		h.Write(node)
		h.Write(brotherHash)
		sum = h.Sum()
		node = sum
	}
	root := api.Lookup2(rop.Index0, rop.Index1, rop.Root0, rop.Root1, rop.Root2, rop.Root3)
	api.AssertIsEqual(sum, root)
	return nil
}

func (rop *RootObfuscationProof) PreCompile(args ...interface{}) {
	depth := args[0].(int)
	rop.Path = make([]frontend.Variable, depth)
}

func (rop *RootObfuscationProof) Assign(curveName string, args ...interface{}) {
	depth := args[0].(int)
	mod := gnarkwrapper.CurveMap[curveName].ScalarField()
	hashFunc := mimchash.MiMCCaseMap[curveName].Hash
	rop.Path = make([]frontend.Variable, depth)
	tree := merkletree.New(curveName, 2, depth)
	tree.RandConstruct()
	proofSet := tree.GenerateMerkleProof2(0)
	for i := range proofSet {
		rop.Path[i] = proofSet[i]
	}
	rop.Leaf = tree.LeavesList[0].Hash
	rop.Root0 = tree.Root.Hash
	rop.Root1 = mimchash.MiMCHash(hashFunc, [][]byte{mimchash.Convert2Byte(utils.RandStr(10), mod)})
	rop.Root2 = mimchash.MiMCHash(hashFunc, [][]byte{mimchash.Convert2Byte(utils.RandStr(10), mod)})
	rop.Root3 = mimchash.MiMCHash(hashFunc, [][]byte{mimchash.Convert2Byte(utils.RandStr(10), mod)})
	rop.Index0 = 0
	rop.Index1 = 0
}

// ── Fixture Generation ──────────────────────────────────────────────────

const curveName = "BN254"
const scheme = "groth16"

func GenerateRangeROFixture(in GenerateInput) (GenerateOutput, error) {
	if in.RODepth != 10 {
		return GenerateOutput{}, fmt.Errorf("only ro_depth=10 is supported in Stage 1, got %d", in.RODepth)
	}

	artifactsDir := filepath.Join(in.OutDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return GenerateOutput{}, fmt.Errorf("mkdir artifacts: %w", err)
	}

	curve := gnarkwrapper.CurveMap[curveName]

	// ── Range Hash Proof ─────────────────────────────────────────────
	var rangeCircuit RangeHashProof
	rangeCircuit.PreCompile()
	rangeZk := gnarkwrapper.NewGnarkWrapper(scheme, &rangeCircuit, curve)
	rangeZk.Compile()
	rangeZk.Setup()

	chCCS := filepath.Join(artifactsDir, "ch_range.css")
	chPK := filepath.Join(artifactsDir, "ch_range.pk")
	chVK := filepath.Join(artifactsDir, "ch_range.vk")
	rangeZk.WriteCCS(chCCS)
	rangeZk.WritePK(chPK)
	rangeZk.WriteVK(chVK)

	rangeCircuit.Assign(curveName)
	rangeZk.SetAssignment(&rangeCircuit)
	rangeZk.GenerateWitness(false)
	rangeZk.Prove()

	chProof := filepath.Join(artifactsDir, "ch_range.proof")
	chPublic := filepath.Join(artifactsDir, "ch_range.public")
	rangeZk.WriteProof(chProof)
	rangeZk.GenerateWitness(true)
	rangeZk.WriteWitness(chPublic, true)

	// ── Root Obfuscation Proof ───────────────────────────────────────
	var roCircuit RootObfuscationProof
	roCircuit.PreCompile(int(in.RODepth))
	roZk := gnarkwrapper.NewGnarkWrapper(scheme, &roCircuit, curve)
	roZk.Compile()
	roZk.Setup()

	roCCS := filepath.Join(artifactsDir, "ro_depth10.css")
	roPK := filepath.Join(artifactsDir, "ro_depth10.pk")
	roVK := filepath.Join(artifactsDir, "ro_depth10.vk")
	roZk.WriteCCS(roCCS)
	roZk.WritePK(roPK)
	roZk.WriteVK(roVK)

	// Write VK bundle (concatenation of both VK files)
	vkBundle := filepath.Join(artifactsDir, "vk_bundle.bin")
	vkData, err := os.ReadFile(chVK)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("read ch vk: %w", err)
	}
	roVkData, err := os.ReadFile(roVK)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("read ro vk: %w", err)
	}
	vkData = append(vkData, roVkData...)
	if err := os.WriteFile(vkBundle, vkData, 0o644); err != nil {
		return GenerateOutput{}, fmt.Errorf("write vk bundle: %w", err)
	}

	roCircuit.Assign(curveName, int(in.RODepth))
	roZk.SetAssignment(&roCircuit)
	roZk.GenerateWitness(false)
	roZk.Prove()

	roProof := filepath.Join(artifactsDir, "ro_depth10.proof")
	roPublic := filepath.Join(artifactsDir, "ro_depth10.public")
	roZk.WriteProof(roProof)
	roZk.GenerateWitness(true)
	roZk.WriteWitness(roPublic, true)

	// ── Compute Hashes ───────────────────────────────────────────────
	chProofHash, err := artifact.SHA256FileHex(chProof)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("hash CH proof: %w", err)
	}
	roProofHash, err := artifact.SHA256FileHex(roProof)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("hash RO proof: %w", err)
	}
	vkHash, err := artifact.SHA256FileHex(vkBundle)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("hash VK bundle: %w", err)
	}

	chPublicBytes, err := os.ReadFile(chPublic)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("read CH public witness: %w", err)
	}
	roPublicBytes, err := os.ReadFile(roPublic)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("read RO public witness: %w", err)
	}
	publicInputHash := artifact.Blake2Hex(chPublicBytes, roPublicBytes)

	// Write artifact-relative paths (strip outDir prefix for portability)
	rel := func(p string) string {
		r, err := filepath.Rel(in.OutDir, p)
		if err != nil {
			return p
		}
		return r
	}

	art := artifact.ProofArtifact{
		Version:            1,
		ProofSystem:        "gnark-groth16-bn254",
		ProofSystemCode:    1,
		ConstraintKind:     "range",
		ConstraintKindCode: 1,
		RODepth:            in.RODepth,
		RequestHash:        in.RequestHash,
		SessionID:          in.SessionID,
		RoundIndex:         in.RoundIndex,
		VKHash:             vkHash,
		CHProofHash:        chProofHash,
		ROProofHash:        roProofHash,
		PublicInputHash:    publicInputHash,
		Files: artifact.Files{
			CHProof:         rel(chProof),
			CHPublicWitness: rel(chPublic),
			ROProof:         rel(roProof),
			ROPublicWitness: rel(roPublic),
			VKBundle:        rel(vkBundle),
		},
	}
	proofDigest, err := art.ComputeProofDigest()
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("compute proof digest: %w", err)
	}
	art.ProofDigest = proofDigest

	artifactPath := filepath.Join(in.OutDir, "artifact.json")
	if err := artifact.Write(artifactPath, art); err != nil {
		return GenerateOutput{}, fmt.Errorf("write artifact: %w", err)
	}

	return GenerateOutput{Artifact: art}, nil
}

// ── Verification ───────────────────────────────────────────────────────

// VerifyRangeRO verifies both proofs in the artifact.
// Caller must ensure CWD is the artifact's directory so relative paths in
// p.Files resolve correctly.
// Also re-computes file hashes and compares with artifact fields to ensure
// the verified files match the digest stored on-chain.
func VerifyRangeRO(p artifact.ProofArtifact) error {
	if err := p.Validate(); err != nil {
		return err
	}

	// Re-hash proof files and compare with artifact fields
	chProofHash, err := artifact.SHA256FileHex(p.Files.CHProof)
	if err != nil {
		return fmt.Errorf("hash CH proof: %w", err)
	}
	if chProofHash != p.CHProofHash {
		return fmt.Errorf("CH proof hash mismatch: artifact=%s file=%s", p.CHProofHash, chProofHash)
	}

	roProofHash, err := artifact.SHA256FileHex(p.Files.ROProof)
	if err != nil {
		return fmt.Errorf("hash RO proof: %w", err)
	}
	if roProofHash != p.ROProofHash {
		return fmt.Errorf("RO proof hash mismatch: artifact=%s file=%s", p.ROProofHash, roProofHash)
	}

	vkHash, err := artifact.SHA256FileHex(p.Files.VKBundle)
	if err != nil {
		return fmt.Errorf("hash VK bundle: %w", err)
	}
	if vkHash != p.VKHash {
		return fmt.Errorf("VK hash mismatch: artifact=%s file=%s", p.VKHash, vkHash)
	}

	chPub, err := os.ReadFile(p.Files.CHPublicWitness)
	if err != nil {
		return fmt.Errorf("read CH public witness: %w", err)
	}
	roPub, err := os.ReadFile(p.Files.ROPublicWitness)
	if err != nil {
		return fmt.Errorf("read RO public witness: %w", err)
	}
	publicInputHash := artifact.Blake2Hex(chPub, roPub)
	if publicInputHash != p.PublicInputHash {
		return fmt.Errorf("public input hash mismatch: artifact=%s computed=%s", p.PublicInputHash, publicInputHash)
	}

	// Bind VK files to vk_bundle: concatenate the actual .vk files and
	// compare against vk_bundle.bin to ensure the verifier uses the same
	// keys that the on-chain digest committed to.
	chVKPath := filepath.Join(filepath.Dir(p.Files.CHProof), "ch_range.vk")
	roVKPath := filepath.Join(filepath.Dir(p.Files.ROProof), "ro_depth10.vk")

	chVKBytes, err := os.ReadFile(chVKPath)
	if err != nil {
		return fmt.Errorf("read CH vk: %w", err)
	}
	roVKBytes, err := os.ReadFile(roVKPath)
	if err != nil {
		return fmt.Errorf("read RO vk: %w", err)
	}

	expectedBundle, err := os.ReadFile(p.Files.VKBundle)
	if err != nil {
		return fmt.Errorf("read vk bundle: %w", err)
	}
	actualBundle := append(chVKBytes, roVKBytes...)
	if !bytes.Equal(actualBundle, expectedBundle) {
		return fmt.Errorf("VK bundle mismatch: actual .vk files do not match vk_bundle.bin")
	}

	curve := gnarkwrapper.CurveMap[curveName]

	// Verify Range Proof
	var rangeCircuit RangeHashProof
	rangeCircuit.PreCompile()
	rangeZk := gnarkwrapper.NewGnarkWrapper(scheme, &rangeCircuit, curve)
	rangeZk.ReadVK(chVKPath)
	rangeZk.ReadProof(p.Files.CHProof)
	rangeZk.ReadWitness(p.Files.CHPublicWitness, true)
	rangeZk.Verify()

	// Verify RO Proof
	var roCircuit RootObfuscationProof
	roCircuit.PreCompile(int(p.RODepth))
	roZk := gnarkwrapper.NewGnarkWrapper(scheme, &roCircuit, curve)
	roZk.ReadVK(roVKPath)
	roZk.ReadProof(p.Files.ROProof)
	roZk.ReadWitness(p.Files.ROPublicWitness, true)
	roZk.Verify()

	return nil
}

func init() {
	// suppress unused import warning
	_ = strconv.Itoa
}
