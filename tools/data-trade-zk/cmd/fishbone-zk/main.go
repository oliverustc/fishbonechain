package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"fishbone-data-trade-zk/internal/artifact"
	"fishbone-data-trade-zk/internal/business"
	"fishbone-data-trade-zk/internal/dynamic"
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
	case "business-fixture":
		businessFixtureCmd(os.Args[2:])
	case "make-witness":
		makeWitnessCmd(os.Args[2:])
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

func businessFixtureCmd(args []string) {
	fs := flag.NewFlagSet("business-fixture", flag.ExitOnError)
	witnessPath := fs.String("witness", "", "business range witness JSON")
	outDir := fs.String("out", "", "output directory")
	requestHash := fs.String("request-hash", "", "override request hash (default from witness)")
	sessionID := fs.Uint("session-id", 0, "override session ID")
	roundIndex := fs.Uint("round-index", 0, "override round index")
	_ = fs.Parse(args)
	if *witnessPath == "" || *outDir == "" {
		fmt.Fprintln(os.Stderr, "--witness and --out are required")
		os.Exit(2)
	}
	w, err := business.ReadRangeWitness(*witnessPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read witness: %v\n", err)
		os.Exit(1)
	}
	// CLI overrides for session-specific fields (only when explicitly provided)
	setFlags := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })
	if setFlags["request-hash"] {
		w.RequestHash = *requestHash
	}
	if setFlags["session-id"] {
		w.SessionID = uint32(*sessionID)
	}
	if setFlags["round-index"] {
		w.RoundIndex = uint32(*roundIndex)
	}
	out, err := gnarkadapter.GenerateBusinessRangeFixture(w, *outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "business fixture failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("proof_digest=%s\n", out.Artifact.ProofDigest)
	fmt.Printf("business_input_hash=%s\n", out.Artifact.BusinessInputHash)
}

func makeWitnessCmd(args []string) {
	fs := flag.NewFlagSet("make-witness", flag.ExitOnError)
	datasetPath := fs.String("dataset", "", "dataset JSON")
	requestPath := fs.String("request", "", "request JSON")
	outPath := fs.String("out", "", "output witness JSON (single range)")
	outDir := fs.String("out-dir", "", "output directory (multi range)")
	sessionID := fs.Uint("session-id", 0, "session ID")
	roundIndex := fs.Uint("round-index", 0, "round index")
	_ = fs.Parse(args)

	hasOut := *outPath != ""
	hasOutDir := *outDir != ""

	if *datasetPath == "" || *requestPath == "" {
		fmt.Fprintln(os.Stderr, "--dataset and --request are required")
		os.Exit(2)
	}
	if hasOut && hasOutDir {
		fmt.Fprintln(os.Stderr, "--out and --out-dir cannot be used together")
		os.Exit(2)
	}

	ds, err := dynamic.ReadDataset(*datasetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dataset: %v\n", err)
		os.Exit(1)
	}
	req, err := dynamic.ReadRequest(*requestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read request: %v\n", err)
		os.Exit(1)
	}

	if req.ConstraintKind == "range" {
		if hasOutDir {
			fmt.Fprintln(os.Stderr, "--out-dir is not allowed for single range requests (use --out)")
			os.Exit(2)
		}
		if !hasOut {
			fmt.Fprintln(os.Stderr, "--out is required for single range requests")
			os.Exit(2)
		}
		w, err := dynamic.BuildRangeWitness(ds, req, uint32(*sessionID), uint32(*roundIndex))
		if err != nil {
			fmt.Fprintf(os.Stderr, "build witness: %v\n", err)
			os.Exit(1)
		}
		b, err := json.MarshalIndent(w, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal witness: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*outPath, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write witness: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("witness=%s\n", *outPath)
		fmt.Printf("dataset_id=%s\n", ds.DatasetID)
		fmt.Printf("record_id=%s\n", req.RecordID)
		fmt.Printf("field_name=%s\n", req.FieldName)
		return
	}

	// multi_range
	if hasOut {
		fmt.Fprintln(os.Stderr, "--out is not allowed for multi_range requests (use --out-dir)")
		os.Exit(2)
	}
	if !hasOutDir {
		fmt.Fprintln(os.Stderr, "--out-dir is required for multi_range requests")
		os.Exit(2)
	}
	witnesses, err := dynamic.BuildRangeWitnesses(ds, req, uint32(*sessionID), uint32(*roundIndex))
	if err != nil {
		fmt.Fprintf(os.Stderr, "build witnesses: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create out dir: %v\n", err)
		os.Exit(1)
	}
	type witnessEntry struct {
		Index       int    `json:"index"`
		FieldName   string `json:"field_name"`
		WitnessPath string `json:"witness_path"`
	}
	manifest := struct {
		Version        int             `json:"version"`
		ConstraintKind string          `json:"constraint_kind"`
		RequestHash    string          `json:"request_hash"`
		DatasetID      string          `json:"dataset_id"`
		RecordID       string          `json:"record_id"`
		Witnesses      []witnessEntry  `json:"witnesses"`
	}{1, req.ConstraintKind, req.RequestHash, ds.DatasetID, req.RecordID, nil}

	for i, w := range witnesses {
		wPath := filepath.Join(*outDir, fmt.Sprintf("witness-%d.json", i))
		b, err := json.MarshalIndent(w, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal witness %d: %v\n", i, err)
			os.Exit(1)
		}
		if err := os.WriteFile(wPath, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write witness %d: %v\n", i, err)
			os.Exit(1)
		}
		fn := "?"
		for _, c := range req.Constraints {
			if c.FieldName == w.IMT.FieldName {
				fn = c.FieldName
				break
			}
		}
		fmt.Printf("witness[%d]=%s field=%s\n", i, wPath, fn)
		manifest.Witnesses = append(manifest.Witnesses, witnessEntry{i, fn, fmt.Sprintf("witness-%d.json", i)})
	}

	manifestPath := filepath.Join(*outDir, "manifest.json")
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal manifest: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(manifestPath, append(mb, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write manifest: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("witness_manifest=%s\n", manifestPath)
}
