package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"fishbone-data-trade-zk/internal/artifact"
	"fishbone-data-trade-zk/internal/gnarkadapter"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fishbone-zk <setup|prove|verify|fixture> [options]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "verify":
		verifyCmd(os.Args[2:])
	case "fixture":
		fixtureCmd(os.Args[2:])
	case "setup", "prove":
		fmt.Fprintf(os.Stderr, "%s: not yet implemented (use fixture for Stage 1)\n", os.Args[1])
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func verifyCmd(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	artifactPath := fs.String("artifact", "", "proof artifact JSON")
	_ = fs.Parse(args)
	if *artifactPath == "" {
		fmt.Fprintln(os.Stderr, "--artifact is required")
		os.Exit(2)
	}
	p, err := artifact.Read(*artifactPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read artifact: %v\n", err)
		os.Exit(1)
	}
	if err := p.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "rejected: %v\n", err)
		os.Exit(1)
	}
	// Run actual gnark proof verification.
	// Resolve file paths relative to the artifact's directory.
	baseDir := filepath.Dir(*artifactPath)
	if err := os.Chdir(baseDir); err != nil {
		fmt.Fprintf(os.Stderr, "chdir to artifact dir: %v\n", err)
		os.Exit(1)
	}
	if err := gnarkadapter.VerifyRangeRO(p); err != nil {
		fmt.Fprintf(os.Stderr, "rejected: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("accepted")
}

func fixtureCmd(args []string) {
	fs := flag.NewFlagSet("fixture", flag.ExitOnError)
	outDir := fs.String("out", "", "output directory")
	requestHash := fs.String("request-hash", "0x0000000000000000000000000000000000000000000000000000000000000000", "request hash")
	sessionID := fs.Uint("session-id", 0, "session ID")
	roundIndex := fs.Uint("round-index", 0, "round index")
	roDepth := fs.Uint("ro-depth", 10, "RO depth")
	_ = fs.Parse(args)
	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "--out is required")
		os.Exit(2)
	}
	out, err := gnarkadapter.GenerateRangeROFixture(gnarkadapter.GenerateInput{
		OutDir:      *outDir,
		RequestHash: *requestHash,
		SessionID:   uint32(*sessionID),
		RoundIndex:  uint32(*roundIndex),
		RODepth:     uint32(*roDepth),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("proof_digest=%s\n", out.Artifact.ProofDigest)
}
