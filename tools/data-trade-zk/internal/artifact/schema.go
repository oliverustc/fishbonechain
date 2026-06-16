package artifact

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/blake2b"
)

const (
	ProofDigestDomain = "FISHBONE:DATA_TRADE:ZK_PROOF:v1"
	AttestDomain      = "FISHBONE:DATA_TRADE:ZK_ATTEST:v1"
)

type Files struct {
	CHProof         string `json:"ch_proof"`
	CHPublicWitness string `json:"ch_public_witness"`
	ROProof         string `json:"ro_proof"`
	ROPublicWitness string `json:"ro_public_witness"`
	VKBundle        string `json:"vk_bundle"`
}

type ProofArtifact struct {
	Version            uint32 `json:"version"`
	ProofSystem        string `json:"proof_system"`
	ProofSystemCode    uint8  `json:"proof_system_code"`
	ConstraintKind     string `json:"constraint_kind"`
	ConstraintKindCode uint8  `json:"constraint_kind_code"`
	RODepth            uint32 `json:"ro_depth"`
	RequestHash        string `json:"request_hash"`
	SessionID          uint32 `json:"session_id"`
	RoundIndex         uint32 `json:"round_index"`
	VKHash             string `json:"vk_hash"`
	CHProofHash        string `json:"ch_proof_hash"`
	ROProofHash        string `json:"ro_proof_hash"`
	PublicInputHash    string `json:"public_input_hash"`
	ProofDigest        string `json:"proof_digest"`
	Files              Files  `json:"files"`
}

func Blake2Hex(parts ...[]byte) string {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	for _, p := range parts {
		_, _ = h.Write(p)
	}
	return "0x" + hex.EncodeToString(h.Sum(nil))
}

func SHA256FileHex(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

func strip0x(s string) string {
	return strings.TrimPrefix(strings.ToLower(s), "0x")
}

func mustHex(s string) ([]byte, error) {
	b, err := hex.DecodeString(strip0x(s))
	if err != nil {
		return nil, fmt.Errorf("invalid hex %q: %w", s, err)
	}
	return b, nil
}

func le32(v uint32) []byte {
	var out [4]byte
	binary.LittleEndian.PutUint32(out[:], v)
	return out[:]
}

func (p ProofArtifact) ComputeProofDigest() (string, error) {
	req, err := mustHex(p.RequestHash)
	if err != nil {
		return "", fmt.Errorf("request_hash: %w", err)
	}
	vk, err := mustHex(p.VKHash)
	if err != nil {
		return "", fmt.Errorf("vk_hash: %w", err)
	}
	ch, err := mustHex(p.CHProofHash)
	if err != nil {
		return "", fmt.Errorf("ch_proof_hash: %w", err)
	}
	ro, err := mustHex(p.ROProofHash)
	if err != nil {
		return "", fmt.Errorf("ro_proof_hash: %w", err)
	}
	pi, err := mustHex(p.PublicInputHash)
	if err != nil {
		return "", fmt.Errorf("public_input_hash: %w", err)
	}
	return Blake2Hex(
		[]byte(ProofDigestDomain),
		[]byte{p.ProofSystemCode},
		[]byte{p.ConstraintKindCode},
		le32(p.RODepth),
		req,
		le32(p.SessionID),
		le32(p.RoundIndex),
		vk,
		ch,
		ro,
		pi,
	), nil
}

func (p ProofArtifact) Validate() error {
	if p.Version != 1 {
		return errors.New("version must be 1")
	}
	if p.ProofSystem != "gnark-groth16-bn254" {
		return errors.New("unsupported proof_system")
	}
	if p.ProofSystemCode != 1 {
		return errors.New("proof_system_code must be 1")
	}
	if p.ConstraintKind != "range" && p.ConstraintKind != "subset" && p.ConstraintKind != "substr" {
		return errors.New("unsupported constraint_kind")
	}
	if p.ConstraintKindCode < 1 || p.ConstraintKindCode > 3 {
		return errors.New("constraint_kind_code out of range")
	}
	// Enforce kind/code consistency
	kindCodeMap := map[string]uint8{"range": 1, "subset": 2, "substr": 3}
	if kindCodeMap[p.ConstraintKind] != p.ConstraintKindCode {
		return errors.New("constraint_kind does not match constraint_kind_code")
	}
	digest, err := p.ComputeProofDigest()
	if err != nil {
		return err
	}
	if p.ProofDigest != digest {
		return errors.New("proof_digest mismatch")
	}
	return nil
}

func Read(path string) (ProofArtifact, error) {
	var p ProofArtifact
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(b, &p)
	return p, err
}

func Write(path string, p ProofArtifact) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
