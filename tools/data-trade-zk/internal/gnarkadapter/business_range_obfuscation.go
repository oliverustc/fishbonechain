package gnarkadapter

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fishbone-data-trade-zk/internal/artifact"
	"fishbone-data-trade-zk/internal/business"

	"gnarkabc/gnarkwrapper"
)

// GenerateBusinessRangeFixture generates a full proof artifact using the
// BusinessRangeProof circuit (Stage 2.2) instead of the old random-witness
// RangeHashProof. The RO proof is still generated with RandomWitness.
func GenerateBusinessRangeFixture(w business.RangeWitness, outDir string) (GenerateOutput, error) {
	if err := w.Validate(); err != nil {
		return GenerateOutput{}, err
	}

	artifactsDir := filepath.Join(outDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return GenerateOutput{}, fmt.Errorf("mkdir artifacts: %w", err)
	}

	curve := gnarkwrapper.CurveMap[curveName]

	// ── Business Range Proof ──────────────────────────────────────────
	var businessCircuit BusinessRangeProof
	businessCircuit.PreCompile()
	businessZk := gnarkwrapper.NewGnarkWrapper(scheme, &businessCircuit, curve)
	businessZk.Compile()
	businessZk.Setup()

	chCCS := filepath.Join(artifactsDir, "ch_range.css")
	chPK := filepath.Join(artifactsDir, "ch_range.pk")
	chVK := filepath.Join(artifactsDir, "ch_range.vk")
	businessZk.WriteCCS(chCCS)
	businessZk.WritePK(chPK)
	businessZk.WriteVK(chVK)

	businessCircuit.Assign(curveName, w)
	// Verify the fixture's masked_value_hash matches the circuit-computed MiMC hash
	if w.IsMaskedValueHashProvided() {
		mvhStr := fmt.Sprintf("0x%x", businessCircuit.MaskedValueHash)
		if mvhStr != w.MaskedValueHash {
			return GenerateOutput{}, fmt.Errorf("masked_value_hash mismatch: fixture=%s circuit=%s",
				w.MaskedValueHash, mvhStr)
		}
	} else {
		w.MaskedValueHash = fmt.Sprintf("0x%x", businessCircuit.MaskedValueHash)
	}

	businessZk.SetAssignment(&businessCircuit)
	businessZk.GenerateWitness(false)
	businessZk.Prove()

	chProof := filepath.Join(artifactsDir, "ch_range.proof")
	chPublic := filepath.Join(artifactsDir, "ch_range.public")
	businessZk.WriteProof(chProof)
	businessZk.GenerateWitness(true)
	businessZk.WriteWitness(chPublic, true)

	// ── Root Obfuscation Proof ───────────────────────────────────────
	var roCircuit RootObfuscationProof
	roCircuit.PreCompile(10)
	roZk := gnarkwrapper.NewGnarkWrapper(scheme, &roCircuit, curve)
	roZk.Compile()
	roZk.Setup()

	roCCS := filepath.Join(artifactsDir, "ro_depth10.css")
	roPK := filepath.Join(artifactsDir, "ro_depth10.pk")
	roVK := filepath.Join(artifactsDir, "ro_depth10.vk")
	roZk.WriteCCS(roCCS)
	roZk.WritePK(roPK)
	roZk.WriteVK(roVK)

	vkBundle := filepath.Join(artifactsDir, "vk_bundle.bin")
	chVkData, err := os.ReadFile(chVK)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("read ch vk: %w", err)
	}
	roVkData, err := os.ReadFile(roVK)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("read ro vk: %w", err)
	}
	vkData := append(chVkData, roVkData...)
	if err := os.WriteFile(vkBundle, vkData, 0o644); err != nil {
		return GenerateOutput{}, fmt.Errorf("write vk bundle: %w", err)
	}

	roCircuit.Assign(curveName, 10)
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

	// ── Business Input Hash ──────────────────────────────────────────
	u64le := func(v uint64) []byte {
		var out [8]byte
		binary.LittleEndian.PutUint64(out[:], v)
		return out[:]
	}
	saltBytes, err := hex.DecodeString(strings.TrimPrefix(w.SaltHex, "0x"))
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("decode salt: %w", err)
	}
	mvhBytes, err := hex.DecodeString(strings.TrimPrefix(w.MaskedValueHash, "0x"))
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("decode masked_value_hash: %w", err)
	}
	businessHash := artifact.Blake2Hex(
		u64le(w.RawValue),
		u64le(w.MinValue),
		u64le(w.MaxValue),
		u64le(w.MaskDelta),
		saltBytes,
		mvhBytes,
	)

	rel := func(p string) string {
		r, err := filepath.Rel(outDir, p)
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
		RODepth:            10,
		RequestHash:        w.RequestHash,
		SessionID:          w.SessionID,
		RoundIndex:         w.RoundIndex,
		VKHash:             vkHash,
		CHProofHash:        chProofHash,
		ROProofHash:        roProofHash,
		PublicInputHash:    publicInputHash,
		BusinessInputHash:  businessHash,
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

	if err := artifact.Write(filepath.Join(outDir, "artifact.json"), art); err != nil {
		return GenerateOutput{}, fmt.Errorf("write artifact: %w", err)
	}

	return GenerateOutput{Artifact: art}, nil
}
