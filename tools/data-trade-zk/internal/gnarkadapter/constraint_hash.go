package gnarkadapter

import "fishbone-data-trade-zk/internal/artifact"

type GenerateInput struct {
	OutDir      string
	RequestHash string
	SessionID   uint32
	RoundIndex  uint32
	RODepth     uint32
}

type GenerateOutput struct {
	Artifact artifact.ProofArtifact
}
