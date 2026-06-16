# Stage 2 Paper Business Witness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 分两步推进论文业务 witness：Stage 2.1 先把业务输入 canonical hash 绑定进 artifact/digest/链上 attestation；Stage 2.2 再替换 gnark 电路 witness，使电路实际证明业务约束。

**Architecture:** 先定义最小业务 witness schema，再扩展 `fishbone-zk` CLI 支持 business metadata fixture，并让 `zk_real_data_trade_flow.js` 读取业务样例生成带 `business_input_hash` 的 proof artifact。Stage 2.1 仍复用现有 CH/RO gnark fixture 电路，业务字段只通过 digest 绑定到链上，不代表电路已证明业务逻辑；Stage 2.2 单独替换 gnark assignment/circuit，使 proof witness 真正来自业务数据。

**Tech Stack:** Go/gnark, Node.js artifact reader, Rust pallet digest schema, JSON fixtures, VM E2E regression from Stage 1.

---

## Files

- Create: `tools/data-trade-zk/internal/business/schema.go`
- Create: `tools/data-trade-zk/internal/business/schema_test.go`
- Create: `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go`
- Create: `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation_test.go`
- Modify: `tools/data-trade-zk/cmd/fishbone-zk/main.go`
- Modify: `tools/data-trade-zk/internal/artifact/schema.go`
- Modify: `scripts/lib/zk_artifact.js`
- Modify: `scripts/zk_real_data_trade_flow.js`
- Create: `scripts/fixtures/data_trade_business_sample.json`
- Modify: `docs/implementation/data-trade-implementation.md`
- Modify: `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md`

## Stage Split

- **Stage 2.1: Business Metadata Binding**：本计划主体。证明 artifact 额外携带 `business_input_hash`，该 hash 进入 `proof_digest`，再由 child6 `submitDataProof` 与 Charlie attestation 绑定到链上。此阶段不得在文档或代码日志中声称“gnark 电路证明了业务数据”。
- **Stage 2.2: Circuit-Level Business Witness**：后续必须新增或改造 gnark circuit/assignment，使 `raw_value/min/max/mask_delta/salt` 真正进入约束系统。只有 Stage 2.2 完成后，才能声称 proof 证明了业务约束。

## Business Witness v1 Metadata

最小业务场景只实现 `constraint_kind = "range"`：

- DR 请求一个数值型数据字段，例如 `age` 或 `sensor_value`。
- DO 持有原始值 `raw_value`。
- DO 公开脱敏值承诺 `masked_value_hash`。
- Stage 2.1 metadata 绑定：
  - `raw_value` 在 `[min_value, max_value]` 范围内。
  - `masked_value = raw_value + mask_delta`。
  - `masked_value_hash = SHA256(masked_value || salt)` 的字段表示与 artifact public input 绑定。
  - `request_hash/session_id/round_index` 进入 public input digest。

Stage 2.1 不证明完整 IMT membership，也不让上述业务字段进入 gnark 电路约束；IMT/root obfuscation 与业务电路 witness 替换属于 Stage 2.2。

## Task 1: Define Business Witness Schema

- [ ] Step 1: Write failing Go tests in `tools/data-trade-zk/internal/business/schema_test.go`.

Expected test file:

```go
package business

import "testing"

func TestRangeWitnessValidateAcceptsValidSample(t *testing.T) {
	w := RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:  7,
		RoundIndex: 2,
		RawValue:  42,
		MinValue:  18,
		MaxValue:  65,
		MaskDelta: 1000,
		SaltHex:   "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	if err := w.Validate(); err != nil {
		t.Fatalf("validate valid witness: %v", err)
	}
}

func TestRangeWitnessValidateRejectsOutOfRange(t *testing.T) {
	w := RangeWitness{
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:  7,
		RoundIndex: 2,
		RawValue:  99,
		MinValue:  18,
		MaxValue:  65,
		MaskDelta: 1000,
		SaltHex:   "0x2222222222222222222222222222222222222222222222222222222222222222",
	}
	if err := w.Validate(); err == nil {
		t.Fatalf("expected out-of-range witness to fail")
	}
}
```

- [ ] Step 2: Run failing test.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/business
```

Expected: fail because package/types do not exist.

- [ ] Step 3: Implement `tools/data-trade-zk/internal/business/schema.go`.

Expected file:

```go
package business

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type RangeWitness struct {
	RequestHash string `json:"request_hash"`
	SessionID   uint32 `json:"session_id"`
	RoundIndex  uint32 `json:"round_index"`
	RawValue    uint64 `json:"raw_value"`
	MinValue    uint64 `json:"min_value"`
	MaxValue    uint64 `json:"max_value"`
	MaskDelta   uint64 `json:"mask_delta"`
	SaltHex     string `json:"salt_hex"`
}

func validHex32(value string) bool {
	raw := strings.TrimPrefix(strings.ToLower(value), "0x")
	b, err := hex.DecodeString(raw)
	return err == nil && len(b) == 32
}

func (w RangeWitness) Validate() error {
	if !validHex32(w.RequestHash) {
		return fmt.Errorf("request_hash must be 32-byte hex")
	}
	if !validHex32(w.SaltHex) {
		return fmt.Errorf("salt_hex must be 32-byte hex")
	}
	if w.MinValue > w.MaxValue {
		return fmt.Errorf("min_value must be <= max_value")
	}
	if w.RawValue < w.MinValue || w.RawValue > w.MaxValue {
		return fmt.Errorf("raw_value outside requested range")
	}
	return nil
}

func ReadRangeWitness(path string) (RangeWitness, error) {
	var w RangeWitness
	b, err := os.ReadFile(path)
	if err != nil {
		return w, err
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return w, err
	}
	return w, w.Validate()
}
```

- [ ] Step 4: Run passing test.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/business
```

Expected: pass.

- [ ] Step 5: Commit schema.

Run:

```bash
git add tools/data-trade-zk/internal/business
git commit -m "feat: add data trade business witness schema"
```

## Task 2: Extend Artifact Schema for Business Public Inputs

- [ ] Step 1: Add tests in `tools/data-trade-zk/internal/artifact/schema_test.go`.

Append:

```go
func TestBusinessProofDigestIncludesBusinessInputHash(t *testing.T) {
	p := ProofArtifact{
		Version:            1,
		ProofSystem:        "gnark-groth16-bn254",
		ProofSystemCode:    1,
		ConstraintKind:     "range",
		ConstraintKindCode: 1,
		RODepth:            10,
		RequestHash:        "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:          1,
		RoundIndex:         0,
		VKHash:             "0x2222222222222222222222222222222222222222222222222222222222222222",
		CHProofHash:        "0x3333333333333333333333333333333333333333333333333333333333333333",
		ROProofHash:        "0x4444444444444444444444444444444444444444444444444444444444444444",
		PublicInputHash:    "0x5555555555555555555555555555555555555555555555555555555555555555",
		BusinessInputHash:  "0x6666666666666666666666666666666666666666666666666666666666666666",
	}
	a, err := p.ComputeProofDigest()
	if err != nil {
		t.Fatalf("digest a: %v", err)
	}
	p.BusinessInputHash = "0x7777777777777777777777777777777777777777777777777777777777777777"
	b, err := p.ComputeProofDigest()
	if err != nil {
		t.Fatalf("digest b: %v", err)
	}
	if a == b {
		t.Fatalf("business input hash must affect proof digest")
	}
}
```

- [ ] Step 2: Run failing test.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/artifact
```

Expected: fail because `BusinessInputHash` does not exist.

- [ ] Step 3: Modify `tools/data-trade-zk/internal/artifact/schema.go`.

Add field:

```go
BusinessInputHash string `json:"business_input_hash"`
```

In `ComputeProofDigest`, after `public_input_hash`, include:

```go
business, err := mustHex(p.BusinessInputHash)
if err != nil {
	return "", fmt.Errorf("business_input_hash: %w", err)
}
```

And append `business` to `Blake2Hex(...)`.

Also update existing `GenerateRangeROFixture` so non-business artifacts remain valid after `business_input_hash` becomes required:

```go
const ZeroBusinessInputHash = "0x0000000000000000000000000000000000000000000000000000000000000000"
```

Set:

```go
BusinessInputHash: artifact.ZeroBusinessInputHash,
```

- [ ] Step 4: Run tests.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/artifact
```

Expected: pass.

- [ ] Step 5: Mirror JS artifact validation in `scripts/lib/zk_artifact.js`.

Add required `business_input_hash` check:

```js
if (!HEX_32.test(a.business_input_hash)) {
  throw new Error(`invalid business_input_hash: ${a.business_input_hash}`);
}
```

- [ ] Step 6: Run JS syntax check.

Run:

```bash
node --check scripts/lib/zk_artifact.js
```

Expected: pass.

- [ ] Step 7: Commit artifact schema.

Run:

```bash
git add tools/data-trade-zk/internal/artifact/schema.go tools/data-trade-zk/internal/artifact/schema_test.go scripts/lib/zk_artifact.js
git commit -m "feat: bind business input hash in zk artifact"
```

## Task 3: Add Business Sample Fixture

- [ ] Step 1: Create `scripts/fixtures/data_trade_business_sample.json`.

Expected file:

```json
{
  "request_hash": "0x6b9f5a1765adf428a2b7220c2fa6e11ef4f3d8235dc145d7c42b3e26fbd13a01",
  "session_id": 0,
  "round_index": 0,
  "raw_value": 42,
  "min_value": 18,
  "max_value": 65,
  "mask_delta": 1000,
  "salt_hex": "0x2222222222222222222222222222222222222222222222222222222222222222"
}
```

- [ ] Step 2: Validate fixture through business reader.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/business
```

Expected: pass.

- [ ] Step 3: Commit fixture.

Run:

```bash
git add scripts/fixtures/data_trade_business_sample.json
git commit -m "test: add data trade business witness fixture"
```

## Task 4: Implement CLI `business-fixture`

- [ ] Step 1: Add CLI command test by running expected failing command.

Run:

```bash
cd tools/data-trade-zk && go run ./cmd/fishbone-zk business-fixture --witness ../../scripts/fixtures/data_trade_business_sample.json --out ../../target/business-zk-smoke
```

Expected: fail with `unknown command: business-fixture`.

- [ ] Step 2: Modify `tools/data-trade-zk/cmd/fishbone-zk/main.go`.

Add switch case:

```go
case "business-fixture":
	businessFixtureCmd(os.Args[2:])
```

Add function:

```go
func businessFixtureCmd(args []string) {
	fs := flag.NewFlagSet("business-fixture", flag.ExitOnError)
	witnessPath := fs.String("witness", "", "business range witness JSON")
	outDir := fs.String("out", "", "output directory")
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
	out, err := gnarkadapter.GenerateBusinessRangeFixture(w, *outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "business fixture failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("proof_digest=%s\n", out.Artifact.ProofDigest)
}
```

Also import:

```go
"fishbone-data-trade-zk/internal/business"
```

- [ ] Step 3: Implement `GenerateBusinessRangeFixture`.

Create `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go` with a wrapper that:

- validates witness;
- calls existing `GenerateRangeROFixture` with request/session/round/depth;
- computes deterministic `business_input_hash = artifact.Blake2Hex(...)` from fixed-width little-endian encodings of `raw_value/min/max/mask_delta` plus salt bytes;
- sets `out.Artifact.BusinessInputHash`;
- recomputes `ProofDigest`;
- rewrites `artifact.json`.

Expected core function:

```go
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
	salt, err := hex.DecodeString(strings.TrimPrefix(w.SaltHex, "0x"))
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("decode salt: %w", err)
	}
	businessHash := artifact.Blake2Hex(
		u64le(w.RawValue),
		u64le(w.MinValue),
		u64le(w.MaxValue),
		u64le(w.MaskDelta),
		salt,
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
```

- [ ] Step 4: Run CLI smoke.

Run:

```bash
cd tools/data-trade-zk && go run ./cmd/fishbone-zk business-fixture --witness ../../scripts/fixtures/data_trade_business_sample.json --out ../../target/business-zk-smoke
cd ../.. && target/tools/fishbone-zk verify --artifact target/business-zk-smoke/artifact.json
```

Expected: `verify` prints `accepted`.

- [ ] Step 5: Commit CLI feature.

Run:

```bash
git add tools/data-trade-zk/cmd/fishbone-zk/main.go tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go
git commit -m "feat: generate business witness zk artifacts"
```

## Task 5: Use Business Witness in Real E2E Script

- [ ] Step 1: Modify `scripts/zk_real_data_trade_flow.js`.

Add `--business-witness` arg:

```js
const BUSINESS_WITNESS = parseArg("--business-witness") || "scripts/fixtures/data_trade_business_sample.json";
```

Replace fixture command:

```js
const fixResult = spawnSync(ZK_CMD, [
  "business-fixture", "--witness", BUSINESS_WITNESS, "--out", outDir,
], { stdio: "inherit" });
```

When reading artifact, continue using `artifact.proof_digest`, `artifact.public_input_hash`, and `artifact.vk_hash`.

- [ ] Step 2: Run syntax check.

Run:

```bash
node --check scripts/zk_real_data_trade_flow.js
```

Expected: pass.

- [ ] Step 3: Run local business fixture smoke.

Run:

```bash
mkdir -p target/tools
cd tools/data-trade-zk && go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
cd ../..
target/tools/fishbone-zk business-fixture --witness scripts/fixtures/data_trade_business_sample.json --out target/business-zk-smoke
target/tools/fishbone-zk verify --artifact target/business-zk-smoke/artifact.json
```

Expected: `accepted`.

- [ ] Step 4: Commit E2E script change.

Run:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat: use business witness in real zk data trade flow"
```

## Task 6: VM E2E Regression

- [ ] Step 1: Run Stage 1 regression script.

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD_WS=ws://10.2.2.11:9950 scripts/run_data_trade_vm_regression.sh
```

Expected: pass.

- [ ] Step 2: Record result in this plan.

Required record:

```markdown
- VM E2E passed at <timestamp>.
- Real ZK path used `business-fixture`.
- Artifact includes `business_input_hash`.
```

- [ ] Step 3: Update docs.

Modify `docs/implementation/data-trade-implementation.md`:

- Change “Paper witness” limitation to say Stage 2 minimal range business witness is implemented.
- Keep a limitation that full IMT membership and all constraint kinds remain future work.

- [ ] Step 4: Commit docs and roadmap.

Run:

```bash
git add docs/implementation/data-trade-implementation.md docs/internal/agent-plans/2026-06-16-stage2-paper-business-witness.md docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md
git commit -m "docs: record business witness vm verification"
```

## Execution Record

- Not started.
