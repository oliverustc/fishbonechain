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

## Stage 2.2 已定案执行口径（Agent Read This First）

Stage 2.2 不再作为开放设计题交给执行 agent 自由选择。执行范围必须保持窄而可验收：**只实现 range 业务 witness 真正进入 gnark 电路**，不实现 subset/substr，不把 IMT/root obfuscation 与业务电路耦合，不做 VK registry，不做链上 ZK verifier，不改变当前 VerifierAuthority attestation 安全模型。

Stage 2.2 的完成含义是：

- `raw_value/min_value/max_value/mask_delta/salt/masked_value_hash` 不再只是链下 metadata，而是进入 gnark circuit 的 witness/public inputs。
- gnark proof 能证明：
  - `raw_value in [min_value, max_value]`
  - `masked_value = raw_value + mask_delta`
  - `masked_value_hash = MiMC(masked_value, salt)`
- `business_input_hash` 继续进入 artifact `proof_digest` 和 child6 `submitDataProof`，链上仍只校验 digest + verifier attestation。
- 文档必须清楚说明：Stage 2.2 完成后，**range 业务约束由 gnark proof 证明**；但 RO/IMT、subset/substr、链上 verifier、trustless bridge 仍是后续工作。

非目标（不要在 Stage 2.2 做）：

- 不实现 subset/substr 电路接入。
- 不把 `masked_value_hash` 放进 RO proof leaf。
- 不实现 runtime 里直接验证 gnark proof。
- 不实现 production trusted setup / VK registry。
- 不实现动态数据源、真实随机 salt 管理或多轮不同业务数据策略。

### 2.2.1 电路改造范围

当前 gnark 电路使用两个独立电路：`RangeHashProof`（prove `preimage` 在 `[min, max]` 范围内且 hash 匹配）和 `RootObfuscationProof`（Merkle proof + 4-root obfuscation）。此前有三个候选方向，现记录为背景，执行时以下方“决策”为准：

- **方案 A**：只修改 `RangeHashProof.Assign()`，将 `utils.RandStr(3)` 替换为 `witness.RawValue`。`MaskDelta` 和 `Salt` 不进电路，仅通过 `business_input_hash` 在 digest 层面绑定。优点：改动最小。缺点：`masked_value` 的正确性依赖链下信任。
- **方案 B**：新增一个组合电路，同时证明 `raw_value ∈ [min, max]` 和 `masked_value_hash = H(raw_value + delta || salt)`。优点：电路完整证明了业务逻辑。缺点：需要新电路定义、新 trusted setup、新 VK。
- **方案 C**：修改现有 `RangeHashProof` 增加 public input 字段（`MaskedValueHash`），在 `Define()` 中增加 `masked = raw + delta; api.AssertIsEqual(H(masked || salt), MaskedValueHash)`。优点：复用已有电路结构。缺点：MiMC hash 的 `masked_value_hash` 与 SHA256 版本不一致——当前 `business_input_hash` 用的是 SHA256，但 gnark 电路使用 MiMC（BN254 原生）。

**决策：选方案 C 的简化版。**

执行方式：

- 新增或扩展一个 `BusinessRangeProof` 电路，优先新建，避免破坏 Stage 1/2.1 的 `RangeHashProof` smoke。
- 电路内使用 MiMC，不使用 SHA256。原因：MiMC 是当前 gnark/BN254 代码已采用的 ZK-friendly hash；SHA256 在电路内成本高，也会引入新的复杂依赖。
- Stage 2.1 的 `masked_value_hash = SHA256(masked_value || salt)` 语义在 Stage 2.2 迁移为 `masked_value_hash = MiMC(masked_value, salt)`。文档必须明确这是 Stage 2.2 为了电路友好性做出的 hash 语义升级。
- `business_input_hash` 仍可使用 Blake2 作为 artifact/digest 层面的 canonical metadata hash，但它绑定的 `masked_value_hash` 字段值应来自 MiMC commitment。

验收要求：

- 不能只把 `utils.RandStr(3)` 替换成 `raw_value` 就结束；`mask_delta/salt/masked_value_hash` 必须参与电路约束。
- 修改 `masked_value_hash` 后，gnark verify 必须失败。
- 修改 `raw_value/min/max/mask_delta/salt` 中任一会影响业务 public input/hash，并且对应测试必须覆盖至少一个篡改路径。

### 2.2.2 Witness 数据来源

Stage 2.1 的 witness 来自静态 JSON fixture。此前关于动态数据源、每轮数据变化和 salt 随机化有若干讨论点，现统一收敛如下：

- 动态生成 witness、多轮不同 `raw_value`、DO 随机 `MaskDelta`、每轮随机 `Salt` 都是后续增强，不属于 Stage 2.2。

**决策：继续使用 JSON fixture，E2E 脚本只覆盖 session-specific 字段。**

执行方式：

- `scripts/fixtures/data_trade_business_sample.json` 继续作为默认 witness。
- `zk_real_data_trade_flow.js` 继续在运行时覆盖 `request_hash/session_id/round_index`。
- `raw_value/min/max/mask_delta/salt` 先保持 fixture 固定值。
- 每轮 salt 随机化、动态 sensor reading、多数据源接入均不属于 Stage 2.2。

验收要求：

- CLI 必须允许 `--session-id 0 --round-index 0` 显式覆盖。
- fixture 中若提供 `masked_value_hash`，必须校验它等于由当前 witness 计算出的 MiMC commitment；若不提供，可以由 CLI/业务 reader 计算并填充到 artifact 语义中。

### 2.2.3 IMT / Root Obfuscation 与业务电路的耦合

Stage 2.1 中 RO proof 完全独立于业务 witness。此前讨论过将业务承诺耦合进 RO proof，但 Stage 2.2 不采用该方向：

- 不将 `masked_value_hash` 作为 RO proof 的叶子节点。
- 不把 RO proof 与业务 proof 合并成一个组合 proof。

**决策：暂不耦合。**

执行方式：

- Stage 2.2 只让 business range proof 真实证明业务约束。
- `RootObfuscationProof` 继续保持当前独立 proof。
- `ProofArtifact` 仍可以携带 CH/RO 两类 proof hash；业务 range proof 可替换现有 CH range proof，RO 不绑定业务 leaf。

验收要求：

- 不修改 RO proof 的 Merkle leaf 语义。
- 文档必须说明：Stage 2.2 后，range 业务约束已由 gnark proof 证明；完整 IMT membership 与业务承诺的耦合仍未完成。

### 2.2.4 多约束种类 (subset / substr) 的优先级

当前 `constraint_kind` 固定为 `range`。`subset` 和 `substr` 的 gnark 电路原型存在于 `references/data_trade_code/snarks/gnarkzkp/cmd/constraint-hash-proof/circuit_subset_hash.go` 和 `circuit_substr_hash.go`，但尚未接入。Stage 2.2 先完成 range，subset/substr 留给后续阶段。

**决策：Stage 2.2 只做 range。**

执行方式：

- 不接入 `subset` / `substr`。
- `constraint_kind` 仍为 `range`。
- subset/substr 后续单独规划，可作为 Stage 2.3 或 Stage 3 的一部分。

验收要求：

- 不新增 subset/substr runtime 或 JS profile 复杂度。
- 不修改当前 artifact kind code map，除非只是保持兼容已有 enum。

### 2.2.5 Trusted Setup 管理

当前 Groth16 setup 在 `fixture` 命令中每次重新生成（`Compile` + `Setup`），proof 和 VK 只存在于临时目录。生产级使用需要：

- 预编译电路和 VK，作为构建产物管理（类似 Solidity verifier 的预编译合约）
- 确定性 setup（publicly verifiable）
- VK 版本化：链上 `vk_hash` 绑定到特定版本的 VK，VK 升级时需同步更新链上配置

**决策：Stage 2.2 仍使用 dev per-run setup。**

执行方式：

- 保持 `business-fixture` / fixture 生成时临时 `Compile + Setup + Prove`。
- 继续在 artifact 中绑定 `vk_hash`，并由链上 digest 间接绑定 VK。
- 不实现 VK registry、版本升级治理、预编译 setup artifact。

验收要求：

- 文档必须把当前 setup 标注为 dev fixture 模式。
- 不得声称生产级 trusted setup 已完成。

### 2.2.6 验收标准差异

Stage 2.1 完成标志是 "VM E2E 通过 + business_input_hash 进入 digest"。Stage 2.2 的验收标准如下，不再扩展 VM negative matrix：

- 不新增 VM negative scenario。
- 不要求链上区分 "gnark proof rejected by verifier" 和 "proof digest mismatch"。
- 必须在 Go 层覆盖 witness/proof 篡改失败路径。

**决策：Go negative tests 必须做；VM negative path 暂缓。**

Stage 2.2 必须新增/更新 Go 测试：

- valid business range witness -> proof generation succeeds and `verify` accepts.
- `raw_value` outside `[min_value, max_value]` -> witness validation or generation rejects.
- wrong `masked_value_hash` -> rejects before proving, or proof verification fails.
- tampered public witness / artifact commitment -> `verify` rejects.

VM E2E 验收：

- `scripts/run_data_trade_vm_regression.sh` happy regression 必须通过。
- Real ZK path 必须使用 Stage 2.2 business circuit artifact。
- 可以不新增 VM negative scenario，避免慢速 VM 回归膨胀。

链上错误类型：

- Stage 2.2 不要求 runtime 区分 "gnark verify rejected" 和 "proof digest mismatch"。链上仍接收 verifier attestation 后的 digest。

### 2.2.7 与 `DataTradeProofVerifier` trait 的关系

当前 `AlwaysPassVerifier` 意味着链上不调用任何真实 proof 验证逻辑。Stage 2.2 不改变该安全模型：

- 不替换 `DataTradeProofVerifier`。
- 继续依赖 VerifierAuthority attestation。
- 不新增签名/replay 机制。

**决策：保持 `AlwaysPassVerifier` + VerifierAuthority attestation。**

执行方式：

- 不在 Substrate runtime 中调用 `fishbone-zk verify`。
- 链下 CLI 负责 proof verification；Charlie/VerifierAuthority 负责 attest。
- 链上继续校验 `proof_digest`、`business_input_hash`、`attestation_digest` 和 verifier 权限。

安全边界说明：

- Stage 2.2 提升的是链下 proof 的真实性：proof 现在确实证明 range 业务约束。
- Stage 2.2 不提升为 trustless on-chain ZK verification；链上仍信任 VerifierAuthority 的 attestation。
- Replay 防护继续依赖 digest 中绑定的 `request_hash/session_id/round_index/vk_hash/proof hashes/business_input_hash`，不新增签名机制。

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

### 2026-06-16 Stage 2.2 Complete

- 新增 `BusinessRangeProof` gnark 电路，证明：`raw_value ∈ [min, max]` + `masked_value = raw_value + delta` + `masked_value_hash = MiMC(masked_value, salt)`。
- `masked_value_hash` 从 SHA256 迁移至 MiMC（电路兼容性）。
- `GenerateBusinessRangeFixture` 改用 `BusinessRangeProof` 替代原随机 witness 的 `RangeHashProof`。
- Go 测试新增：wrong masked_value_hash 在电路层被拒绝 (`TestBusinessFixtureRejectsWrongMaskedValueHash`)、固定宽度 LE 编码 (`TestBusinessHashUsesFixedWidthLittleEndian`)、artifact 可验证 (`TestBusinessArtifactIsValidAndVerifiable`)。
- VM E2E：`zk_real_data_trade_flow.js` ✅。
