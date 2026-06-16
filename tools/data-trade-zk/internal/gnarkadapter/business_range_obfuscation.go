package gnarkadapter

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"fishbone-data-trade-zk/internal/artifact"
	"fishbone-data-trade-zk/internal/business"
)

// GenerateBusinessRangeFixture wraps GenerateRangeROFixture with business
// witness metadata. Stage 2.1: business_input_hash is bound to the artifact
// digest but the gnark circuit still uses random witness — this stage does
// NOT claim the circuit proves business constraints.
func GenerateBusinessRangeFixture(w business.RangeWitness, outDir string) (GenerateOutput, error) {
	if err := w.Validate(); err != nil {
		return GenerateOutput{}, err
	}
	out, err := GenerateRangeROFixture(GenerateInput{
		OutDir:      outDir,
		RequestHash: w.RequestHash,
		SessionID:   w.SessionID,
		RoundIndex:  w.RoundIndex,
		RODepth:     10,
	})
	if err != nil {
		return GenerateOutput{}, err
	}
	u64le := func(v uint64) []byte {
		var out [8]byte
		binary.LittleEndian.PutUint64(out[:], v)
		return out[:]
	}
	saltBytes, err := hex.DecodeString(strings.TrimPrefix(w.SaltHex, "0x"))
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("decode salt: %w", err)
	}
	maskedValueHash := w.ComputeMaskedValueHash()
	mvhBytes, err := hex.DecodeString(strings.TrimPrefix(maskedValueHash, "0x"))
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
	out.Artifact.BusinessInputHash = businessHash
	digest, err := out.Artifact.ComputeProofDigest()
	if err != nil {
		return GenerateOutput{}, err
	}
	out.Artifact.ProofDigest = digest
	if err := artifact.Write(filepath.Join(outDir, "artifact.json"), out.Artifact); err != nil {
		return GenerateOutput{}, err
	}
	return out, nil
}
