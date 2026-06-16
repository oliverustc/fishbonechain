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

## Stage 2.2 待明确事项（Open Questions）

以下问题在 Stage 2.2 执行前必须回答，否则无法确定实现路径和验收标准。

### 2.2.1 电路改造范围

当前 gnark 电路使用两个独立电路：`RangeHashProof`（prove `preimage` 在 `[min, max]` 范围内且 hash 匹配）和 `RootObfuscationProof`（Merkle proof + 4-root obfuscation）。Stage 2.2 需要决定：

- **方案 A**：只修改 `RangeHashProof.Assign()`，将 `utils.RandStr(3)` 替换为 `witness.RawValue`。`MaskDelta` 和 `Salt` 不进电路，仅通过 `business_input_hash` 在 digest 层面绑定。优点：改动最小。缺点：`masked_value` 的正确性依赖链下信任。
- **方案 B**：新增一个组合电路，同时证明 `raw_value ∈ [min, max]` 和 `masked_value_hash = H(raw_value + delta || salt)`。优点：电路完整证明了业务逻辑。缺点：需要新电路定义、新 trusted setup、新 VK。
- **方案 C**：修改现有 `RangeHashProof` 增加 public input 字段（`MaskedValueHash`），在 `Define()` 中增加 `masked = raw + delta; api.AssertIsEqual(H(masked || salt), MaskedValueHash)`。优点：复用已有电路结构。缺点：MiMC hash 的 `masked_value_hash` 与 SHA256 版本不一致——当前 `business_input_hash` 用的是 SHA256，但 gnark 电路使用 MiMC（BN254 原生）。

待决定：选 A / B / C？若选 B 或 C，SHA256 ↔ MiMC 的 hash 函数不一致如何解决？

### 2.2.2 Witness 数据来源

Stage 2.1 的 witness 来自静态 JSON fixture。Stage 2.2 需要回答：

- Witness 数据是静态 fixture 还是由 E2E 脚本运行时动态生成？
- 如果是动态生成，每轮 round 的 `raw_value` 是否需要变化（如不同 sensor reading）？
- `MaskDelta` 是 DO 随机选择还是固定值？
- `Salt` 是否需要每轮重新生成以防止 rainbow table 攻击？

### 2.2.3 IMT / Root Obfuscation 与业务电路的耦合

Stage 2.1 中 RO proof 完全独立于业务 witness。Stage 2.2 是否需要：

- 将 `masked_value_hash` 作为 RO proof 的叶子节点之一？
- 还是在现有 RO proof 基础上叠加业务证明（两个无关 proof 的组合）？

### 2.2.4 多约束种类 (subset / substr) 的优先级

当前 `constraint_kind` 固定为 `range`。`subset` 和 `substr` 的 gnark 电路原型存在于 `references/data_trade_code/snarks/gnarkzkp/cmd/constraint-hash-proof/circuit_subset_hash.go` 和 `circuit_substr_hash.go`，但尚未接入。Stage 2.2 是否应包含 subset/substr？还是先完成 range，subset/substr 留在 Stage 3？

### 2.2.5 Trusted Setup 管理

当前 Groth16 setup 在 `fixture` 命令中每次重新生成（`Compile` + `Setup`），proof 和 VK 只存在于临时目录。生产级使用需要：

- 预编译电路和 VK，作为构建产物管理（类似 Solidity verifier 的预编译合约）
- 确定性 setup（publicly verifiable）
- VK 版本化：链上 `vk_hash` 绑定到特定版本的 VK，VK 升级时需同步更新链上配置

### 2.2.6 验收标准差异

Stage 2.1 完成标志是 "VM E2E 通过 + business_input_hash 进入 digest"。Stage 2.2 的完成标志需要回答：

- 是否需要新的 VM E2E scenario（如 "wrong business value → gnark verify rejects"）？
- 是否需要链上能区分 "gnark proof rejected by verifier" 和 "proof digest 不匹配"？
- Go 测试是否需要覆盖 "修改 witness 后 proof 验证失败"（目前 `VerifyRangeRO` 可以做到，但 `business_range_obfuscation_test.go` 未覆盖此路径）？

### 2.2.7 与 `DataTradeProofVerifier` trait 的关系

当前 `AlwaysPassVerifier` 意味着链上不调用任何真实 proof 验证逻辑。Stage 2.2 是否需要：

- 将 `DataTradeProofVerifier` 替换为能调用 `fishbone-zk verify` 的链下服务？
- 还是保持 `AlwaysPassVerifier`，完全依赖 VerifierAuthority attestation？
- 若替换，新的 verifier 签名机制如何防止 replay（同一 proof 被多次 attest 到不同 session）？

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

- [x] Step 1: Write failing Go tests in `tools/data-trade-zk/internal/business/schema_test.go`.

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

- [x] Step 2: Run failing test.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/business
```

Expected: fail because package/types do not exist.

- [x] Step 3: Implement `tools/data-trade-zk/internal/business/schema.go`.

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

- [x] Step 4: Run passing test.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/business
```

Expected: pass.

- [x] Step 5: Commit schema.

Run:

```bash
git add tools/data-trade-zk/internal/business
git commit -m "feat: add data trade business witness schema"
```

## Task 2: Extend Artifact Schema for Business Public Inputs

- [x] Step 1: Add tests in `tools/data-trade-zk/internal/artifact/schema_test.go`.

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

- [x] Step 2: Run failing test.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/artifact
```

Expected: fail because `BusinessInputHash` does not exist.

- [x] Step 3: Modify `tools/data-trade-zk/internal/artifact/schema.go`.

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

- [x] Step 4: Run tests.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/artifact
```

Expected: pass.

- [x] Step 5: Mirror JS artifact validation in `scripts/lib/zk_artifact.js`.

Add required `business_input_hash` check:

```js
if (!HEX_32.test(a.business_input_hash)) {
  throw new Error(`invalid business_input_hash: ${a.business_input_hash}`);
}
```

- [x] Step 6: Run JS syntax check.

Run:

```bash
node --check scripts/lib/zk_artifact.js
```

Expected: pass.

- [x] Step 7: Commit artifact schema.

Run:

```bash
git add tools/data-trade-zk/internal/artifact/schema.go tools/data-trade-zk/internal/artifact/schema_test.go scripts/lib/zk_artifact.js
git commit -m "feat: bind business input hash in zk artifact"
```

## Task 3: Add Business Sample Fixture

- [x] Step 1: Create `scripts/fixtures/data_trade_business_sample.json`.

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

- [x] Step 2: Validate fixture through business reader.

Run:

```bash
cd tools/data-trade-zk && go test ./internal/business
```

Expected: pass.

- [x] Step 3: Commit fixture.

Run:

```bash
git add scripts/fixtures/data_trade_business_sample.json
git commit -m "test: add data trade business witness fixture"
```

## Task 4: Implement CLI `business-fixture`

- [x] Step 1: Add CLI command test by running expected failing command.

Run:

```bash
cd tools/data-trade-zk && go run ./cmd/fishbone-zk business-fixture --witness ../../scripts/fixtures/data_trade_business_sample.json --out ../../target/business-zk-smoke
```

Expected: fail with `unknown command: business-fixture`.

- [x] Step 2: Modify `tools/data-trade-zk/cmd/fishbone-zk/main.go`.

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

- [x] Step 3: Implement `GenerateBusinessRangeFixture`.

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

- [x] Step 4: Run CLI smoke.

Run:

```bash
cd tools/data-trade-zk && go run ./cmd/fishbone-zk business-fixture --witness ../../scripts/fixtures/data_trade_business_sample.json --out ../../target/business-zk-smoke
cd ../.. && target/tools/fishbone-zk verify --artifact target/business-zk-smoke/artifact.json
```

Expected: `verify` prints `accepted`.

- [x] Step 5: Commit CLI feature.

Run:

```bash
git add tools/data-trade-zk/cmd/fishbone-zk/main.go tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go
git commit -m "feat: generate business witness zk artifacts"
```

## Task 5: Use Business Witness in Real E2E Script

- [x] Step 1: Modify `scripts/zk_real_data_trade_flow.js`.

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

- [x] Step 2: Run syntax check.

Run:

```bash
node --check scripts/zk_real_data_trade_flow.js
```

Expected: pass.

- [x] Step 3: Run local business fixture smoke.

Run:

```bash
mkdir -p target/tools
cd tools/data-trade-zk && go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
cd ../..
target/tools/fishbone-zk business-fixture --witness scripts/fixtures/data_trade_business_sample.json --out target/business-zk-smoke
target/tools/fishbone-zk verify --artifact target/business-zk-smoke/artifact.json
```

Expected: `accepted`.

- [x] Step 4: Commit E2E script change.

Run:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat: use business witness in real zk data trade flow"
```

## Task 6: VM E2E Regression

- [x] Step 1: Run Stage 1 regression script.

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD_WS=ws://10.2.2.11:9950 scripts/run_data_trade_vm_regression.sh
```

Expected: pass.

- [x] Step 2: Record result in this plan.

Required record:

```markdown
- VM E2E passed at <timestamp>.
- Real ZK path used `business-fixture`.
- Artifact includes `business_input_hash`.
```

- [x] Step 3: Update docs.

Modify `docs/implementation/data-trade-implementation.md`:

- Change “Paper witness” limitation to say Stage 2 minimal range business witness is implemented.
- Keep a limitation that full IMT membership and all constraint kinds remain future work.

- [x] Step 4: Commit docs and roadmap.

Run:

```bash
git add docs/implementation/data-trade-implementation.md docs/internal/agent-plans/2026-06-16-stage2-paper-business-witness.md docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md
git commit -m "docs: record business witness vm verification"
```

## Execution Record

### 2026-06-16 Stage 2.1 Complete

- Tasks 1-6 全部执行完成。
- `business/schema.go` + `business_range_obfuscation.go` + CLI `business-fixture` 实现，含 `u64le` canonical encoding 和 `masked_value_hash = SHA256(masked_value || salt)` 语义。
- `pallet-trade-session` 新增 `business_input_hash` 参数进入 `submit_data_proof` 和 `compute_zk_proof_digest`，链上 digest 包含业务 hash。
- Go 测试：`internal/business`（5 tests）、`internal/artifact`（6 tests）、`internal/gnarkadapter`（5 tests，含新 `business_range_obfuscation_test.go`）。
- Rust 测试：`pallet-trade-session` 19/19 pass。
- JS 语法检查：`data_trade_flow.js`、`zk_attested_data_trade_flow.js`、`zk_real_data_trade_flow.js` 全部 pass。
- VM E2E：`data_trade_flow.js --scenario happy` ✅、`zk_real_data_trade_flow.js` (business witness) ✅。
- 限制：gnark 电路 witness 仍使用随机 `utils.RandStr`，电路未真正证明业务约束（Stage 2.2 pending）。
