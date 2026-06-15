# Data Trade Gnark ZK Attestation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将论文源码中的 gnark CH/RO proof 原型接入 FishboneChain 数据交易流程，形成“链下真实 proof 生成/验证 + child6 链上 verifier attestation 校验”的可运行闭环。

**Architecture:** 第一阶段不在 Substrate runtime/WASM 中直接验证 Groth16/Plonk proof。复用 `references/data_trade_code/snarks/gnarkzkp` 的 gnark 电路与 wrapper，在仓库内增加稳定 CLI，输出规范化 proof artifact JSON；E2E 脚本调用 CLI 生成/验证 proof，VerifierAuthority 对 proof digest 进行链上 attestation；`pallet-trade-session` 只验证 proof/public-input/vk/request/session/round 绑定与授权 verifier attestation。

**Tech Stack:** Go 1.23.x, gnark/gnark-crypto versions pinned to the paper prototype, Groth16 BN254, Node.js `@polkadot/api`, Substrate FRAME pallet, domain-separated Blake2-256 hashes with explicit byte encoding.

---

## 已处理的审阅意见

- 已把 `submit_data_proof` / `attest_data_proof` 的影响范围扩展到 `pallets/trade-session/src/mock.rs`、Rust tests、两个 dev E2E 脚本和新 real ZK E2E 脚本。
- 已避免依赖不存在的测试 helper：计划要求实现 `setup_accepted_session()` 与 `setup_session_with_submitted_data_proof()`。
- 已按 `references/data_trade_code/snarks/gnarkzkp/go.mod` 固定 Go/gnark 版本与 module name：`gnarkabc`、Go 1.23、gnark 0.11、gnark-crypto 0.14。
- 已明确 `RoundState` 必须存储 proof system、constraint kind、RO depth、CH/RO proof hash、public input hash、VK hash 和 verifier attestation hash。
- 已统一 attestation digest 的 verifier account 编码为 32-byte `addressRaw` / AccountId bytes，不使用 SS58 字符串。
- 已保留 `proof_hash` 作为 storage 字段名，但语义明确为 artifact 的 `proof_digest`。
- 已补充 `submit_proof_signature` 不变量：仍只能在 `DataProofVerified` 后调用，不能被 ZK 改造放宽。
- 已补充 `tools/data-trade-zk` 目录创建、fixture 依赖、`open_round` struct literal 初始化要求。

## Background And Boundaries

### 已阅读的源码与文档

- `references/data_trade_code/snarks/gnarkzkp`
  - `cmd/constraint-hash-proof`: range/subset/substr CH proof 原型。
  - `cmd/root-obfuscation-proof`: RO proof 原型。
  - `gnarkwrapper`: `Compile/Setup/Prove/Verify/WriteProof/ReadProof/WriteVK/ReadVK` 等通用封装。
- `references/data_trade_code/foundry`
  - Solidity verifier 和原 VC/Fund 原型，可作为 proof 语义参考，不直接迁入 Substrate。
- `docs/architecture/cdt.md`
  - 论文流程：CH Proof、RO Proof、公平多轮交付、争议。
- `docs/implementation/data-trade-zk-verifier-plan.md`
  - 已明确推荐路径 A：链下 verifier + 签名验证。
- `pallets/trade-session/src/proof.rs`
  - 当前 `ProofBundle` 只有 `constraint_kind/ch_proof_hash/ro_proof_hash/public_input_hash`。
- `scripts/lib/zk_verifier_client.js`
  - 当前仅支持外部命令返回 `accepted/rejected`，无 proof artifact 结构。

### 重要现实约束

- 论文源码当前偏实验：`Assign()` 多为随机生成 witness，不是业务数据驱动。
- 原型命令是批量 `gen/prove/verify/sol`，输出散落在 `output/`，不适合作为 E2E 的稳定接口。
- 第一阶段只要求证明“真实 gnark proof 可以生成并被链下 verifier 验证”，链上只验证 attestation 和哈希绑定。
- 不要在本计划中引入 Substrate host function、arkworks 重写或链上 Groth16 verifier；这些是后续阶段。

## File Map

### New Files

- `tools/data-trade-zk/README.md`
  - 说明 gnark CLI 使用方式、artifact schema、信任边界。
- `tools/data-trade-zk/go.mod`
  - 新 Go module，使用 replace 指向 `../../references/data_trade_code/snarks/gnarkzkp` 或直接引用其 module。
- `tools/data-trade-zk/cmd/fishbone-zk/main.go`
  - 稳定 CLI 入口：`setup`、`prove`、`verify`、`fixture`。
- `tools/data-trade-zk/internal/artifact/schema.go`
  - proof artifact JSON 结构、digest 计算、domain separator。
- `tools/data-trade-zk/internal/artifact/schema_test.go`
  - schema/digest 稳定性测试。
- `tools/data-trade-zk/internal/gnarkadapter/constraint_hash.go`
  - CH proof 适配层，先支持 `range`。
- `tools/data-trade-zk/internal/gnarkadapter/root_obfuscation.go`
  - RO proof 适配层，先支持固定 depth=10。
- `tools/data-trade-zk/internal/gnarkadapter/adapter_test.go`
  - 生成/验证正反例测试。
- `tools/data-trade-zk/testdata/range_ro_valid.json`
  - 稳定 valid proof fixture。
- `tools/data-trade-zk/testdata/range_ro_invalid_public_input.json`
  - 稳定 invalid fixture。
- `scripts/lib/zk_artifact.js`
  - Node 侧读取 artifact、计算/校验 digest、生成链上调用所需 hash。
- `scripts/lib/zk_attestation.js`
  - Node 侧 verifier attestation 消息构造与签名辅助。
- `scripts/zk_real_data_trade_flow.js`
  - 调用真实 gnark CLI 的数据交易 E2E。

### Modified Files

- `pallets/trade-session/src/proof.rs`
  - 增加 `ProofSystem`、proof digest / attestation digest helper。
- `pallets/trade-session/src/types.rs`
  - `RoundState` 增加 `proof_system`, `constraint_kind`, `ro_depth`, `ch_proof_hash`, `ro_proof_hash`, `public_input_hash`, `vk_hash`, `verifier_attestation_hash` 等字段。
- `pallets/trade-session/src/lib.rs`
  - 修改 `submit_data_proof` / `attest_data_proof` 绑定校验。
- `pallets/trade-session/src/tests.rs`
  - 增加 proof binding、错误 attestation、错误 vk/request/session/round 的单测。
- `pallets/trade-session/src/mock.rs`
  - 更新 `complete_round` helper 与新增 session setup helper，匹配 extrinsic 新签名。
- `runtime/src/configs/mod.rs`
  - 若需要，保持 `VerifierAuthority = Charlie`，确认 tests/runtime config 不退化。
- `scripts/lib/zk_verifier_client.js`
  - 从 `accepted/rejected` 简单协议升级为读取 artifact JSON + 调用 CLI verify。
- `scripts/zk_attested_data_trade_flow.js`
  - 保持 dev 模式，但标注为 `dev-attested`；真实 ZK 路径使用新脚本。
- `docs/implementation/data-trade-zk-verifier-plan.md`
  - 更新 Phase 2 说明和运行命令。

## Artifact Schema

第一阶段 artifact 必须是稳定 JSON，字段顺序不作为共识内容。digest 由 canonical byte payload 计算。**Go、Node、runtime 必须使用同一套 byte encoding；不能各自使用 JSON stringify、SCALE tuple、ABI encoding 或字符串拼接。**

```json
{
  "version": 1,
  "proof_system": "gnark-groth16-bn254",
  "proof_system_code": 1,
  "constraint_kind": "range",
  "constraint_kind_code": 1,
  "ro_depth": 10,
  "request_hash": "0x...",
  "session_id": 0,
  "round_index": 0,
  "vk_hash": "0x...",
  "ch_proof_hash": "0x...",
  "ro_proof_hash": "0x...",
  "public_input_hash": "0x...",
  "proof_digest": "0x...",
  "files": {
    "ch_proof": "artifacts/ch_range.proof",
    "ch_public_witness": "artifacts/ch_range.public",
    "ro_proof": "artifacts/ro_depth10.proof",
    "ro_public_witness": "artifacts/ro_depth10.public",
    "vk_bundle": "artifacts/vk_bundle.bin"
  }
}
```

`proof_digest` 计算规则：

```text
Blake2_256(
  "FISHBONE:DATA_TRADE:ZK_PROOF:v1" ||
  proof_system_code_u8 ||
  constraint_kind_code_u8 ||
  ro_depth_le_u32 ||
  request_hash ||
  session_id_le_u32 ||
  round_index_le_u32 ||
  vk_hash ||
  ch_proof_hash ||
  ro_proof_hash ||
  public_input_hash
)
```

链上 attestation payload digest 计算规则：

```text
Blake2_256(
  "FISHBONE:DATA_TRADE:ZK_ATTEST:v1" ||
  session_id_le_u32 ||
  round_index_le_u32 ||
  proof_digest ||
  accepted_bool_u8 ||
  verifier_account_scale_encode
)
```

编码约定：

- `proof_system_code`: `1 = GnarkGroth16Bn254`。
- `constraint_kind_code`: `1 = Range`, `2 = Subset`, `3 = Substr`。
- `*_hash`: 32-byte raw bytes from `0x...` hex。
- `u32`: little-endian 4 bytes。
- `accepted`: one byte, `1` or `0`。
- `verifier_account_scale_encode`: runtime `AccountId::encode()`；Node 侧使用 verifier keyring pair 的 `addressRaw`，当前 sr25519 AccountId 正好是 32 bytes。

## Tasks

### Task 1: 建立 gnark CLI 骨架与 schema 测试

**Files:**
- Create: `tools/data-trade-zk/go.mod`
- Create: `tools/data-trade-zk/cmd/fishbone-zk/main.go`
- Create: `tools/data-trade-zk/internal/artifact/schema.go`
- Create: `tools/data-trade-zk/internal/artifact/schema_test.go`
- Create: `tools/data-trade-zk/README.md`

- [ ] **Step 1: 创建 Go module**

先创建目录：

```bash
mkdir -p tools/data-trade-zk/cmd/fishbone-zk tools/data-trade-zk/internal/artifact tools/data-trade-zk/internal/gnarkadapter tools/data-trade-zk/testdata
```

在 `tools/data-trade-zk/go.mod` 中加入：

```go
module fishbone-data-trade-zk

go 1.23.0

require (
	gnarkabc v0.0.0
	github.com/consensys/gnark v0.11.0
	github.com/consensys/gnark-crypto v0.14.0
	golang.org/x/crypto v0.32.0
)

replace gnarkabc => ../../references/data_trade_code/snarks/gnarkzkp
```

这些版本必须与 `references/data_trade_code/snarks/gnarkzkp/go.mod` 保持一致。不要升级 gnark 大版本；gnark witness/proof serialization 跨版本不稳定。

- [ ] **Step 2: 写 artifact schema**

在 `tools/data-trade-zk/internal/artifact/schema.go` 中实现：

```go
package artifact

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	Version         uint32 `json:"version"`
	ProofSystem     string `json:"proof_system"`
	ProofSystemCode uint8  `json:"proof_system_code"`
	ConstraintKind  string `json:"constraint_kind"`
	ConstraintCode  uint8  `json:"constraint_kind_code"`
	RODepth         uint32 `json:"ro_depth"`
	RequestHash     string `json:"request_hash"`
	SessionID       uint32 `json:"session_id"`
	RoundIndex      uint32 `json:"round_index"`
	VKHash          string `json:"vk_hash"`
	CHProofHash     string `json:"ch_proof_hash"`
	ROProofHash     string `json:"ro_proof_hash"`
	PublicInputHash string `json:"public_input_hash"`
	ProofDigest     string `json:"proof_digest"`
	Files           Files  `json:"files"`
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

func mustHex(s string) []byte {
	b, err := hex.DecodeString(strip0x(s))
	if err != nil {
		panic(err)
	}
	return b
}

func le32(v uint32) []byte {
	var out [4]byte
	binary.LittleEndian.PutUint32(out[:], v)
	return out[:]
}

func (p ProofArtifact) ComputeProofDigest() string {
	return Blake2Hex(
		[]byte(ProofDigestDomain),
		[]byte{p.ProofSystemCode},
		[]byte{p.ConstraintCode},
		le32(p.RODepth),
		mustHex(p.RequestHash),
		le32(p.SessionID),
		le32(p.RoundIndex),
		mustHex(p.VKHash),
		mustHex(p.CHProofHash),
		mustHex(p.ROProofHash),
		mustHex(p.PublicInputHash),
	)
}

func (p ProofArtifact) Validate() error {
	if p.Version != 1 {
		return errors.New("version must be 1")
	}
	if p.ProofSystem != "gnark-groth16-bn254" {
		return errors.New("unsupported proof_system")
	}
	if p.ProofSystemCode != 1 {
		return errors.New("unsupported proof_system_code")
	}
	if p.ConstraintKind != "range" && p.ConstraintKind != "subset" && p.ConstraintKind != "substr" {
		return errors.New("unsupported constraint_kind")
	}
	if (p.ConstraintKind == "range" && p.ConstraintCode != 1) ||
		(p.ConstraintKind == "subset" && p.ConstraintCode != 2) ||
		(p.ConstraintKind == "substr" && p.ConstraintCode != 3) {
		return errors.New("constraint_kind_code mismatch")
	}
	if p.ProofDigest != p.ComputeProofDigest() {
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
```

- [ ] **Step 3: 写 digest 稳定性测试**

在 `tools/data-trade-zk/internal/artifact/schema_test.go` 中加入：

```go
package artifact

import "testing"

func TestComputeProofDigestIsStable(t *testing.T) {
	p := ProofArtifact{
		Version:         1,
		ProofSystem:     "gnark-groth16-bn254",
		ProofSystemCode: 1,
		ConstraintKind:  "range",
		ConstraintCode:  1,
		RODepth:         10,
		RequestHash:     "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:       7,
		RoundIndex:      2,
		VKHash:          "0x2222222222222222222222222222222222222222222222222222222222222222",
		CHProofHash:     "0x3333333333333333333333333333333333333333333333333333333333333333",
		ROProofHash:     "0x4444444444444444444444444444444444444444444444444444444444444444",
		PublicInputHash: "0x5555555555555555555555555555555555555555555555555555555555555555",
	}
	got := p.ComputeProofDigest()
	if got == "" || got[:2] != "0x" || len(got) != 66 {
		t.Fatalf("bad digest format: %q", got)
	}
	p.ProofDigest = got
	if err := p.Validate(); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}
}

func TestValidateRejectsDigestMismatch(t *testing.T) {
	p := ProofArtifact{
		Version:         1,
		ProofSystem:     "gnark-groth16-bn254",
		ProofSystemCode: 1,
		ConstraintKind:  "range",
		ConstraintCode:  1,
		RODepth:         10,
		RequestHash:     "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:       7,
		RoundIndex:      2,
		VKHash:          "0x2222222222222222222222222222222222222222222222222222222222222222",
		CHProofHash:     "0x3333333333333333333333333333333333333333333333333333333333333333",
		ROProofHash:     "0x4444444444444444444444444444444444444444444444444444444444444444",
		PublicInputHash: "0x5555555555555555555555555555555555555555555555555555555555555555",
		ProofDigest:     "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected digest mismatch")
	}
}
```

- [ ] **Step 4: 创建 CLI 空壳**

在 `tools/data-trade-zk/cmd/fishbone-zk/main.go` 中加入：

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"fishbone-data-trade-zk/internal/artifact"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fishbone-zk <setup|prove|verify|fixture> [options]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "verify":
		verifyCmd(os.Args[2:])
	case "setup", "prove", "fixture":
		fmt.Fprintf(os.Stderr, "%s is reserved for Task 2\n", os.Args[1])
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
	fmt.Println("accepted")
}
```

- [ ] **Step 5: 运行 Go 测试**

Run:

```bash
cd tools/data-trade-zk
go mod tidy
go test ./...
```

Expected: `ok fishbone-data-trade-zk/internal/artifact`.

- [ ] **Step 6: 写 README 初稿**

`tools/data-trade-zk/README.md` 必须说明：

```md
# Fishbone Data Trade ZK

This tool wraps the paper prototype in `references/data_trade_code/snarks/gnarkzkp`.

Stage 1 trust boundary:
- gnark proofs are generated and verified off-chain.
- child6 does not verify Groth16 in WASM.
- child6 verifies hashes, session binding, and VerifierAuthority attestation.

Commands:
- `fishbone-zk fixture`
- `fishbone-zk setup`
- `fishbone-zk prove`
- `fishbone-zk verify --artifact <path>`
```

### Task 2: 封装 gnark range CH proof 与 depth=10 RO proof

**Files:**
- Create: `tools/data-trade-zk/internal/gnarkadapter/constraint_hash.go`
- Create: `tools/data-trade-zk/internal/gnarkadapter/root_obfuscation.go`
- Create: `tools/data-trade-zk/internal/gnarkadapter/adapter_test.go`
- Modify: `tools/data-trade-zk/cmd/fishbone-zk/main.go`

- [ ] **Step 1: 先写 adapter 测试**

在 `tools/data-trade-zk/internal/gnarkadapter/adapter_test.go` 中加入：

```go
package gnarkadapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRangeAndROFixtureCanBeGeneratedAndVerified(t *testing.T) {
	dir := t.TempDir()
	out, err := GenerateRangeROFixture(GenerateInput{
		OutDir:      dir,
		RequestHash: "0x1111111111111111111111111111111111111111111111111111111111111111",
		SessionID:   1,
		RoundIndex:  0,
		RODepth:     10,
	})
	if err != nil {
		t.Fatalf("generate fixture: %v", err)
	}
	if out.Artifact.ProofDigest == "" {
		t.Fatal("missing proof digest")
	}
	if _, err := os.Stat(filepath.Join(dir, "artifact.json")); err != nil {
		t.Fatalf("artifact not written: %v", err)
	}
	if err := VerifyRangeRO(filepath.Join(dir, "artifact.json")); err != nil {
		t.Fatalf("verify fixture: %v", err)
	}
}
```

- [ ] **Step 2: 实现 adapter 类型**

在 `tools/data-trade-zk/internal/gnarkadapter/constraint_hash.go` 中加入公共类型：

```go
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
```

- [ ] **Step 3: 实现最小可用 fixture 生成**

同文件中实现 `GenerateRangeROFixture`。第一版允许调用现有 prototype 的 `go run` 或直接复用 wrapper；但输出必须固定到 `OutDir`，并生成 `artifact.json`。函数签名固定为：

```go
func GenerateRangeROFixture(in GenerateInput) (GenerateOutput, error) {
	// Return GenerateOutput{Artifact: artifact} and write the same artifact to
	// filepath.Join(in.OutDir, "artifact.json").
}
```

实现时可以先复制 `RangeHashProof` 和 `RootObfuscationProof` 电路结构到 `tools/data-trade-zk/internal/gnarkadapter`，避免从 `main` package 导入困难。复制时必须保留 MiMC/BN254 语义，不要改电路。

实现必须满足：

- `in.RODepth == 0` 时使用 `10`。
- `OutDir/artifacts` 下写入 CH proof、CH public witness、RO proof、RO public witness、VK bundle。
- `ch_proof_hash`、`ro_proof_hash`、`vk_hash` 使用 `SHA256FileHex` 计算。
- `public_input_hash` 使用 `Blake2_256(CH public witness bytes || RO public witness bytes)` 计算。
- `proof_digest` 使用 `artifact.ProofArtifact.ComputeProofDigest()` 计算。
- 不读写 `references/.../output`。

- [ ] **Step 4: 实现 verify**

在 `tools/data-trade-zk/internal/gnarkadapter/root_obfuscation.go` 中实现。该函数必须接收 artifact JSON 路径，并以 JSON 所在目录作为 `files.*` 相对路径的 base dir：

```go
package gnarkadapter

import (
	"path/filepath"

	"fishbone-data-trade-zk/internal/artifact"
)

func VerifyRangeRO(artifactPath string) error {
	p, err := artifact.Read(artifactPath)
	if err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	baseDir := filepath.Dir(artifactPath)
	chProofPath := filepath.Join(baseDir, p.Files.CHProof)
	chPublicWitnessPath := filepath.Join(baseDir, p.Files.CHPublicWitness)
	roProofPath := filepath.Join(baseDir, p.Files.ROProof)
	roPublicWitnessPath := filepath.Join(baseDir, p.Files.ROPublicWitness)
	vkBundlePath := filepath.Join(baseDir, p.Files.VKBundle)
	_, _, _, _, _ = chProofPath, chPublicWitnessPath, roProofPath, roPublicWitnessPath, vkBundlePath
	// Read CH proof, CH public witness, CH VK from these paths.
	// Read RO proof, RO public witness, RO VK from these paths.
	// Run gnark Groth16 Verify for both.
	// Return nil only if both proofs verify.
	return nil
}
```

实际实现必须真的调用 gnark verify，不允许只检查 hash。

- [ ] **Step 5: CLI 接入 fixture/prove/verify**

修改 `tools/data-trade-zk/cmd/fishbone-zk/main.go`：

```go
// fixture:
//   fishbone-zk fixture --out <dir> --request-hash 0x... --session-id 0 --round-index 0 --ro-depth 10
// prove:
//   alias to fixture for Stage 1
// verify:
//   fishbone-zk verify --artifact <path>
```

`verify` 必须调用 `gnarkadapter.VerifyRangeRO(artifactPath)`，成功打印 `accepted`，失败打印 `rejected: <reason>` 并 exit 1。

- [ ] **Step 6: 运行 adapter 测试**

Run:

```bash
cd tools/data-trade-zk
go test ./internal/gnarkadapter -run TestRangeAndROFixtureCanBeGeneratedAndVerified -v
```

Expected: PASS，且测试日志不能出现未实现函数或空 proof artifact。

### Task 3: Node 侧 artifact/verifier client 接入

**Files:**
- Create: `scripts/lib/zk_artifact.js`
- Create: `scripts/lib/zk_attestation.js`
- Modify: `scripts/lib/zk_verifier_client.js`
- Test: existing JS tests location if present; otherwise create `scripts/lib/zk_artifact.test.js`

- [ ] **Step 1: 写 Node artifact 工具**

创建 `scripts/lib/zk_artifact.js`：

```js
import fs from "node:fs";
import { blake2AsHex } from "@polkadot/util-crypto";
import { hexToU8a, u8aConcat, stringToU8a } from "@polkadot/util";

function le32(n) {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setUint32(0, n, true);
  return b;
}

export function readZkArtifact(path) {
  return JSON.parse(fs.readFileSync(path, "utf8"));
}

export function computeProofDigest(a) {
  return blake2AsHex(u8aConcat(
    stringToU8a("FISHBONE:DATA_TRADE:ZK_PROOF:v1"),
    new Uint8Array([a.proof_system_code]),
    new Uint8Array([a.constraint_kind_code]),
    le32(a.ro_depth),
    hexToU8a(a.request_hash),
    le32(a.session_id),
    le32(a.round_index),
    hexToU8a(a.vk_hash),
    hexToU8a(a.ch_proof_hash),
    hexToU8a(a.ro_proof_hash),
    hexToU8a(a.public_input_hash),
  ));
}

export function assertValidZkArtifact(a) {
  const digest = computeProofDigest(a);
  if (digest !== a.proof_digest) {
    throw new Error(`proof_digest mismatch: expected ${digest}, got ${a.proof_digest}`);
  }
  if (a.version !== 1) throw new Error("unsupported artifact version");
  if (a.proof_system !== "gnark-groth16-bn254") throw new Error("unsupported proof_system");
  if (a.proof_system_code !== 1) throw new Error("unsupported proof_system_code");
  if (a.constraint_kind === "range" && a.constraint_kind_code !== 1) throw new Error("constraint code mismatch");
  if (a.constraint_kind === "subset" && a.constraint_kind_code !== 2) throw new Error("constraint code mismatch");
  if (a.constraint_kind === "substr" && a.constraint_kind_code !== 3) throw new Error("constraint code mismatch");
  return a;
}
```

- [ ] **Step 2: 写 attestation digest 工具**

创建 `scripts/lib/zk_attestation.js`：

```js
import { blake2AsHex } from "@polkadot/util-crypto";
import { hexToU8a, stringToU8a, u8aConcat } from "@polkadot/util";

function le32(n) {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setUint32(0, n, true);
  return b;
}

export function computeZkAttestationDigest({ sessionId, roundIndex, proofDigest, accepted, verifierAccount }) {
  if (!(verifierAccount instanceof Uint8Array) || verifierAccount.length !== 32) {
    throw new Error("verifierAccount must be a 32-byte AccountId, e.g. charlie.addressRaw");
  }
  return blake2AsHex(u8aConcat(
    stringToU8a("FISHBONE:DATA_TRADE:ZK_ATTEST:v1"),
    le32(sessionId),
    le32(roundIndex),
    hexToU8a(proofDigest),
    new Uint8Array([accepted ? 1 : 0]),
    verifierAccount,
  ));
}
```

- [ ] **Step 3: 升级 verifier client**

修改 `scripts/lib/zk_verifier_client.js`，保留无命令时 dev 模式，但有命令时接受 artifact 路径：

```js
import { spawnSync } from "node:child_process";
import { assertValidZkArtifact, readZkArtifact } from "./zk_artifact.js";

export function verifyProofOffchain({ command, artifactPath }) {
  if (!command) {
    return { accepted: true, mode: "dev-always-accept" };
  }
  const artifact = assertValidZkArtifact(readZkArtifact(artifactPath));
  const result = spawnSync(command, ["verify", "--artifact", artifactPath], {
    encoding: "utf8",
    shell: false,
  });
  if (result.status === 0 && result.stdout.trim() === "accepted") {
    return { accepted: true, mode: "external", artifact };
  }
  return {
    accepted: false,
    mode: "external",
    artifact,
    error: result.stderr || result.stdout,
  };
}
```

- [ ] **Step 4: 增加 JS 单测**

如果项目已有 JS test runner，接入现有 runner；否则创建可直接 `node` 执行的小测试 `scripts/lib/zk_artifact.test.js`：

```js
import assert from "node:assert";
import { computeProofDigest } from "./zk_artifact.js";

const artifact = {
  version: 1,
  proof_system: "gnark-groth16-bn254",
  proof_system_code: 1,
  constraint_kind: "range",
  constraint_kind_code: 1,
  ro_depth: 10,
  request_hash: "0x1111111111111111111111111111111111111111111111111111111111111111",
  session_id: 7,
  round_index: 2,
  vk_hash: "0x2222222222222222222222222222222222222222222222222222222222222222",
  ch_proof_hash: "0x3333333333333333333333333333333333333333333333333333333333333333",
  ro_proof_hash: "0x4444444444444444444444444444444444444444444444444444444444444444",
  public_input_hash: "0x5555555555555555555555555555555555555555555555555555555555555555",
};
const digest = computeProofDigest(artifact);
assert.equal(typeof digest, "string");
assert.equal(digest.length, 66);
console.log("zk artifact tests passed");
```

Run:

```bash
node scripts/lib/zk_artifact.test.js
```

Expected: `zk artifact tests passed`.

### Task 4: 链上 proof binding 与 verifier attestation 校验

**Files:**
- Modify: `pallets/trade-session/src/proof.rs`
- Modify: `pallets/trade-session/src/types.rs`
- Modify: `pallets/trade-session/src/lib.rs`
- Modify: `pallets/trade-session/src/tests.rs`

- [ ] **Step 1: 写失败测试：runtime 必须复算 proof digest**

在 `pallets/trade-session/src/tests.rs` 中新增测试：

```rust
#[test]
fn submit_data_proof_rejects_digest_not_bound_to_session() {
	new_test_ext().execute_with(|| {
		let session_id = setup_accepted_session();
		let round = 0;
		let payment_hash = sp_core::H256::repeat_byte(10);
		let ch_proof_hash = sp_core::H256::repeat_byte(11);
		let ro_proof_hash = sp_core::H256::repeat_byte(12);
		let public_input_hash = sp_core::H256::repeat_byte(13);
		let vk_hash = sp_core::H256::repeat_byte(14);
		let wrong_digest = sp_core::H256::repeat_byte(99);

		assert_ok!(crate::Pallet::<Test>::open_round(RuntimeOrigin::signed(1), session_id, round, payment_hash));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(RuntimeOrigin::signed(1), session_id, round, payment_hash));
		assert_noop!(
			crate::Pallet::<Test>::submit_data_proof(
				RuntimeOrigin::signed(2),
				session_id,
				round,
				crate::proof::ProofSystem::GnarkGroth16Bn254,
				crate::proof::ConstraintKind::Range,
				10,
				ch_proof_hash,
				ro_proof_hash,
				public_input_hash,
				vk_hash,
				wrong_digest,
			),
			Error::<Test>::InvalidProofDigest,
		);
	});
}

#[test]
fn submit_data_proof_records_bound_metadata() {
	new_test_ext().execute_with(|| {
		let session_id = setup_accepted_session();
		let round = 0;
		let payment_hash = sp_core::H256::repeat_byte(10);
		let ch_proof_hash = sp_core::H256::repeat_byte(11);
		let ro_proof_hash = sp_core::H256::repeat_byte(12);
		let public_input_hash = sp_core::H256::repeat_byte(12);
		let vk_hash = sp_core::H256::repeat_byte(13);
		let session = crate::Sessions::<Test>::get(session_id).unwrap();
		let proof_digest = crate::Pallet::<Test>::compute_proof_digest(
			crate::proof::ProofSystem::GnarkGroth16Bn254,
			crate::proof::ConstraintKind::Range,
			10,
			session.request_hash,
			session_id,
			round,
			vk_hash,
			ch_proof_hash,
			ro_proof_hash,
			public_input_hash,
		);

		assert_ok!(crate::Pallet::<Test>::open_round(RuntimeOrigin::signed(1), session_id, round, payment_hash));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(RuntimeOrigin::signed(1), session_id, round, payment_hash));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			RuntimeOrigin::signed(2),
			session_id,
			round,
			crate::proof::ProofSystem::GnarkGroth16Bn254,
			crate::proof::ConstraintKind::Range,
			10,
			ch_proof_hash,
			ro_proof_hash,
			public_input_hash,
			vk_hash,
			proof_digest,
		));

		let state = crate::Rounds::<Test>::get(session_id, round).unwrap();
		assert_eq!(state.proof_hash, Some(proof_digest));
		assert_eq!(state.ch_proof_hash, Some(ch_proof_hash));
		assert_eq!(state.ro_proof_hash, Some(ro_proof_hash));
		assert_eq!(state.public_input_hash, Some(public_input_hash));
		assert_eq!(state.vk_hash, Some(vk_hash));
	});
}
```

这些测试应先失败，因为 `submit_data_proof` 当前只有 `proof_hash` 参数，且 runtime 还不能复算 digest。

- [ ] **Step 2: 增加 ProofSystem 与 digest helper**

在 `pallets/trade-session/src/proof.rs` 中增加：

```rust
#[derive(
	Encode,
	Decode,
	DecodeWithMemTracking,
	Clone,
	Copy,
	PartialEq,
	Eq,
	RuntimeDebug,
	TypeInfo,
	MaxEncodedLen,
)]
pub enum ProofSystem {
	GnarkGroth16Bn254,
}

impl ProofSystem {
	pub fn code(self) -> u8 {
		match self {
			ProofSystem::GnarkGroth16Bn254 => 1,
		}
	}
}

impl ConstraintKind {
	pub fn code(&self) -> u8 {
		match self {
			ConstraintKind::Range => 1,
			ConstraintKind::Subset => 2,
			ConstraintKind::Substr => 3,
		}
	}
}
```

在 `pallets/trade-session/src/lib.rs` 的 `impl<T: Config> Pallet<T>` 中增加 helper。注意这里使用 `Encode` 写入 hash/account bytes，`H256.encode()` 正好是 32-byte raw，Node/Go 侧必须按 Artifact Schema 的 byte 规则镜像：

```rust
pub fn compute_proof_digest(
	proof_system: ProofSystem,
	constraint_kind: ConstraintKind,
	ro_depth: u32,
	request_hash: T::Hash,
	session_id: SessionId,
	round_index: RoundIndex,
	vk_hash: T::Hash,
	ch_proof_hash: T::Hash,
	ro_proof_hash: T::Hash,
	public_input_hash: T::Hash,
) -> T::Hash {
	let mut bytes = b"FISHBONE:DATA_TRADE:ZK_PROOF:v1".to_vec();
	bytes.push(proof_system.code());
	bytes.push(constraint_kind.code());
	bytes.extend(ro_depth.to_le_bytes());
	bytes.extend(request_hash.encode());
	bytes.extend(session_id.to_le_bytes());
	bytes.extend(round_index.to_le_bytes());
	bytes.extend(vk_hash.encode());
	bytes.extend(ch_proof_hash.encode());
	bytes.extend(ro_proof_hash.encode());
	bytes.extend(public_input_hash.encode());
	T::Hashing::hash(&bytes)
}

pub fn compute_attestation_digest(
	session_id: SessionId,
	round_index: RoundIndex,
	proof_digest: T::Hash,
	accepted: bool,
	verifier: &T::AccountId,
) -> T::Hash {
	let mut bytes = b"FISHBONE:DATA_TRADE:ZK_ATTEST:v1".to_vec();
	bytes.extend(session_id.to_le_bytes());
	bytes.extend(round_index.to_le_bytes());
	bytes.extend(proof_digest.encode());
	bytes.push(if accepted { 1 } else { 0 });
	bytes.extend(verifier.encode());
	T::Hashing::hash(&bytes)
}
```

- [ ] **Step 3: 修改 RoundState**

在 `pallets/trade-session/src/types.rs` 的 `RoundState` 增加：

```rust
pub proof_system: Option<crate::proof::ProofSystem>,
pub constraint_kind: Option<crate::proof::ConstraintKind>,
pub ro_depth: Option<u32>,
pub ch_proof_hash: Option<Hash>,
pub ro_proof_hash: Option<Hash>,
pub public_input_hash: Option<Hash>,
pub vk_hash: Option<Hash>,
pub verifier_attestation_hash: Option<Hash>,
```

在 `open_round` 初始化这些字段为 `None`。

- [ ] **Step 4: 修改 submit_data_proof 签名并复算 digest**

将 `submit_data_proof` 从：

```rust
pub fn submit_data_proof(origin, session_id, round_index, proof_hash)
```

改为：

```rust
pub fn submit_data_proof(
	origin: OriginFor<T>,
	session_id: SessionId,
	round_index: RoundIndex,
	proof_system: ProofSystem,
	constraint_kind: ConstraintKind,
	ro_depth: u32,
	ch_proof_hash: T::Hash,
	ro_proof_hash: T::Hash,
	public_input_hash: T::Hash,
	vk_hash: T::Hash,
	proof_digest: T::Hash,
) -> DispatchResult
```

增加校验：

```rust
ensure!(ch_proof_hash != T::Hash::default(), Error::<T>::InvalidProof);
ensure!(ro_proof_hash != T::Hash::default(), Error::<T>::InvalidProof);
ensure!(public_input_hash != T::Hash::default(), Error::<T>::InvalidProof);
ensure!(vk_hash != T::Hash::default(), Error::<T>::InvalidProof);
ensure!(ro_depth > 0, Error::<T>::InvalidProof);

let expected_digest = Self::compute_proof_digest(
	proof_system,
	constraint_kind.clone(),
	ro_depth,
	session.request_hash,
	session_id,
	round_index,
	vk_hash,
	ch_proof_hash,
	ro_proof_hash,
	public_input_hash,
);
ensure!(proof_digest == expected_digest, Error::<T>::InvalidProofDigest);
```

如果当前 `Error` 没有 `InvalidProof`，新增：

```rust
InvalidProof,
InvalidProofDigest,
InvalidAttestation,
```

存储时：

```rust
round.proof_hash = Some(proof_digest);
round.proof_system = Some(proof_system);
round.constraint_kind = Some(constraint_kind);
round.ro_depth = Some(ro_depth);
round.ch_proof_hash = Some(ch_proof_hash);
round.ro_proof_hash = Some(ro_proof_hash);
round.public_input_hash = Some(public_input_hash);
round.vk_hash = Some(vk_hash);
round.status = RoundStatus::DataProofSubmitted;
```

- [ ] **Step 5: 修改 attest_data_proof 绑定并复算 attestation digest**

将 `attest_data_proof` 扩展为接受 `attestation_hash`：

```rust
pub fn attest_data_proof(
	origin: OriginFor<T>,
	session_id: SessionId,
	round_index: RoundIndex,
	proof_hash: T::Hash,
	accepted: bool,
	attestation_hash: T::Hash,
) -> DispatchResult
```

校验：

```rust
ensure!(who == T::VerifierAuthority::get(), Error::<T>::NotVerifier);
ensure!(attestation_hash != T::Hash::default(), Error::<T>::InvalidAttestation);
ensure!(round.proof_hash == Some(proof_hash), Error::<T>::InvalidProof);
let expected_attestation = Self::compute_attestation_digest(
	session_id,
	round_index,
	proof_hash,
	accepted,
	&who,
);
ensure!(attestation_hash == expected_attestation, Error::<T>::InvalidAttestation);
```

若 `accepted == true`，状态转 `DataProofVerified` 并保存 `verifier_attestation_hash`。若 `accepted == false`，状态转 `DataProofRejected` 并保存同一个 attestation hash，便于审计 verifier 的拒绝决定。

- [ ] **Step 6: 增加 attestation 反例测试**

在 `pallets/trade-session/src/tests.rs` 中新增：

```rust
#[test]
fn verifier_attestation_must_match_payload() {
	new_test_ext().execute_with(|| {
		let session_id = setup_session_with_submitted_data_proof();
		let round = 0;
		let proof_digest = crate::Rounds::<Test>::get(session_id, round).unwrap().proof_hash.unwrap();
		let wrong_attestation = sp_core::H256::repeat_byte(88);

		assert_noop!(
			crate::Pallet::<Test>::attest_data_proof(
				RuntimeOrigin::signed(3),
				session_id,
				round,
				proof_digest,
				true,
				wrong_attestation,
			),
			Error::<Test>::InvalidAttestation,
		);

		let expected = crate::Pallet::<Test>::compute_attestation_digest(
			session_id,
			round,
			proof_digest,
			true,
			&3,
		);
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			RuntimeOrigin::signed(3),
			session_id,
			round,
			proof_digest,
			true,
			expected,
		));
	});
}
```

如果当前 test helper 没有 `setup_session_with_submitted_data_proof()`，在 `tests.rs` 中新增一个小 helper，内部完成 `setup_accepted_session -> open_round -> submit_payment_proof -> submit_data_proof`。

- [ ] **Step 7: 更新 mock.rs helper**

在 `pallets/trade-session/src/mock.rs` 中：

- 更新现有 `complete_round` helper，使其调用新的 `submit_data_proof` 和 `attest_data_proof` 签名。
- 新增 `setup_accepted_session()`：

```rust
pub fn setup_accepted_session() -> u32 {
	create_session_helper();
	accept_session_helper(0);
	0
}
```

- 新增 `setup_session_with_submitted_data_proof()`，内部完成：

```rust
let session_id = setup_accepted_session();
let round = 0;
let payment_hash = sp_core::H256::repeat_byte(10);
assert_ok!(crate::Pallet::<Test>::open_round(RuntimeOrigin::signed(1), session_id, round, payment_hash));
assert_ok!(crate::Pallet::<Test>::submit_payment_proof(RuntimeOrigin::signed(1), session_id, round, payment_hash));
// Compute proof_digest with crate::Pallet::<Test>::compute_proof_digest(...)
// Then call submit_data_proof with matching metadata.
session_id
```

helper 内的成功路径必须使用 `compute_proof_digest` 计算出的 digest，不允许用固定 repeat byte 作为成功 digest。

- [ ] **Step 8: 更新所有 Rust 测试调用**

所有原调用：

```rust
submit_data_proof(origin, session_id, round, proof_hash)
attest_data_proof(origin, session_id, round, proof_hash, true)
```

改为：

```rust
submit_data_proof(
	origin,
	session_id,
	round,
	ProofSystem::GnarkGroth16Bn254,
	ConstraintKind::Range,
	10,
	ch_proof_hash,
	ro_proof_hash,
	public_input_hash,
	vk_hash,
	proof_digest,
)
attest_data_proof(origin, session_id, round, proof_digest, true, expected_attestation_hash)
```

测试中不要手写 fake `proof_digest` 或 fake `attestation_hash` 作为成功路径；成功路径必须调用 runtime helper 计算 expected hash。

- [ ] **Step 9: 确认 submit_proof_signature 不变量**

确认 `submit_proof_signature` 仍然要求：

```rust
round.status == RoundStatus::DataProofVerified
```

不要因为引入 ZK attestation 而允许 requester 在 `DataProofSubmitted` 或 `DataProofRejected` 状态下提交签名。

- [ ] **Step 10: 跑 pallet 测试**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
```

Expected: all tests pass。

### Task 5: 真实 gnark E2E 脚本

**Files:**
- Create: `scripts/zk_real_data_trade_flow.js`
- Modify: `scripts/zk_attested_data_trade_flow.js`
- Modify: `scripts/data_trade_flow.js`

- [ ] **Step 1: 标记 dev 脚本**

将 `scripts/zk_attested_data_trade_flow.js` 输出从：

```js
log("verifier=zk-attested");
```

改为：

```js
log("verifier=dev-zk-attested");
```

真实 ZK 使用新脚本，避免误导论文记录。

- [ ] **Step 2: 新建真实 ZK E2E 脚本**

`scripts/zk_real_data_trade_flow.js` 从 `scripts/zk_attested_data_trade_flow.js` 复制起步，但每轮 delivery 做以下流程：

```js
const zkCmd = process.env.ZK_VERIFIER_CMD || "tools/data-trade-zk/fishbone-zk";
const outDir = `target/data-trade-zk/session-${sessionId}-round-${round}`;

// 1. 生成 artifact
spawnSync(zkCmd, [
  "fixture",
  "--out", outDir,
  "--request-hash", sample.requestHash,
  "--session-id", String(sessionId),
  "--round-index", String(round),
  "--ro-depth", "10",
], { stdio: "inherit" });

// 2. 验证 artifact
const artifactPath = `${outDir}/artifact.json`;
const verify = verifyProofOffchain({ command: zkCmd, artifactPath });
if (!verify.accepted) throw new Error(`zk proof rejected: ${verify.error}`);

// 3. 提交完整 proof metadata。runtime 会复算 proof_digest。
const artifact = assertValidZkArtifact(readZkArtifact(artifactPath));
await submitTx(bob, childApi.tx.tradeSession.submitDataProof(
  sessionId,
  round,
  "GnarkGroth16Bn254",
  "Range",
  artifact.ro_depth,
  artifact.ch_proof_hash,
  artifact.ro_proof_hash,
  artifact.public_input_hash,
  artifact.vk_hash,
  artifact.proof_digest,
), `submitDataProof(${round})`);

// 4. 计算 attestation_hash 并提交
const attestationHash = computeZkAttestationDigest({
  sessionId,
  roundIndex: round,
  proofDigest: artifact.proof_digest,
  accepted: true,
  verifierAccount: charlie.addressRaw,
});
await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(
  sessionId,
  round,
  artifact.proof_digest,
  true,
  attestationHash,
), `attestDataProof(${round})`);
```

注意：Polkadot.js keyring pair 使用 `charlie.addressRaw` 作为 AccountId bytes；不要使用 SS58 地址字符串参与 digest。

- [ ] **Step 3: 更新旧脚本适配新 extrinsic**

`scripts/data_trade_flow.js` 和 `scripts/zk_attested_data_trade_flow.js` 的 dev path 需要构造一组可被 runtime 复算通过的 deterministic proof metadata：

```js
const proofSystem = "GnarkGroth16Bn254";
const constraintKind = "Range";
const roDepth = 10;
const chProofHash = hashNTimes(`ch-proof-${sessionId}-${roundIndex}`, 1);
const roProofHash = hashNTimes(`ro-proof-${sessionId}-${roundIndex}`, 1);
const publicInputHash = hashNTimes(`public-input-${sessionId}-${roundIndex}`, 1);
const vkHash = hashNTimes(`vk-${roundIndex}`, 1);
const proofDigest = computeProofDigest({
  version: 1,
  proof_system: "gnark-groth16-bn254",
  proof_system_code: 1,
  constraint_kind: "range",
  constraint_kind_code: 1,
  ro_depth: roDepth,
  request_hash: sample.requestHash,
  session_id: sessionId,
  round_index: roundIndex,
  vk_hash: vkHash,
  ch_proof_hash: chProofHash,
  ro_proof_hash: roProofHash,
  public_input_hash: publicInputHash,
});
const attestationHash = computeZkAttestationDigest({
  sessionId,
  roundIndex,
  proofDigest,
  accepted: true,
  verifierAccount: charlie.addressRaw,
});
```

dev path 的 `submitDataProof` 也必须提交 `proofDigest`，不能继续提交 `ch` 或任意 dummy hash，否则 runtime digest guard 会拒绝。

- [ ] **Step 4: 本地脚本语法检查**

Run:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/data_trade_flow.js
```

Expected: no syntax errors。

### Task 6: 端到端验证与 VM 路径

**Files:**
- Modify: `Makefile`
- Modify: `docs/implementation/data-trade-zk-verifier-plan.md`
- Modify: `docs/implementation/data-trade-flow.md`

- [ ] **Step 1: Makefile 增加 zk tool build**

在 `Makefile` 增加：

```make
.PHONY: build-data-trade-zk

build-data-trade-zk:
	cd tools/data-trade-zk && go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

- [ ] **Step 2: 构建 zk CLI**

Run:

```bash
make build-data-trade-zk
target/tools/fishbone-zk verify --artifact tools/data-trade-zk/testdata/range_ro_valid.json
```

Expected:

```text
accepted
```

如果 fixture 尚未提交，先运行：

```bash
target/tools/fishbone-zk fixture --out tools/data-trade-zk/testdata/range_ro_valid --request-hash 0x1111111111111111111111111111111111111111111111111111111111111111 --session-id 0 --round-index 0 --ro-depth 10
```

- [ ] **Step 3: 跑 Rust/JS 单测**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
node scripts/lib/zk_artifact.test.js
```

Expected: both pass。

- [ ] **Step 4: 本地或 VM 跑真实 ZK E2E**

本地节点或 VM 已部署 child6 时运行：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk \
node scripts/zk_real_data_trade_flow.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950
```

Expected:

```text
verifier=gnark-groth16-bn254
✅ Real ZK-attested path 完成！
```

- [ ] **Step 5: 文档更新**

更新 `docs/implementation/data-trade-zk-verifier-plan.md`，必须明确：

```md
Phase 2 implemented:
- gnark proof generation: off-chain
- gnark proof verification: off-chain CLI
- child6 validation: proof digest + public input hash + vk hash + VerifierAuthority attestation

Not implemented:
- on-chain Groth16 verification
- verifier committee / threshold attestation
- production trusted setup ceremony
```

更新 `docs/implementation/data-trade-flow.md`，增加真实 ZK E2E 命令。

### Task 7: Review Checklist For Implementing Agent

实现 agent 完成后必须逐项确认：

- [ ] `tools/data-trade-zk` 不读写 `references/.../output`，所有输出在调用方指定目录。
- [ ] Go CLI `verify` 对篡改后的 proof/public witness/artifact 会返回非 0。
- [ ] `proof_digest` 同时绑定 `request_hash/session_id/round_index/vk_hash/ch_proof_hash/ro_proof_hash/public_input_hash`。
- [ ] `submit_data_proof` 不能接受 zero `public_input_hash` 或 zero `vk_hash`。
- [ ] `attest_data_proof` 只能由 `VerifierAuthority` 调用。
- [ ] `attest_data_proof` 不能为另一个 proof_hash 提交 attestation。
- [ ] 旧 dev E2E 仍能通过。
- [ ] 新 real ZK E2E 能在 VM child6 上跑通。
- [ ] 文档明确说明这是链下 ZK 验证 + 链上 attestation，不是链上 Groth16 verifier。

## Suggested Commit Sequence

1. `feat(zk): add data trade zk artifact schema`
2. `feat(zk): wrap gnark range and root-obfuscation proofs`
3. `feat(scripts): add zk artifact verifier client`
4. `feat(trade-session): bind zk proof metadata to rounds`
5. `feat(e2e): add real gnark data trade flow`
6. `docs: document data trade zk attestation phase`

## Out Of Scope For This Plan

- Substrate runtime 内直接验证 Groth16/Plonk。
- arkworks 重写 gnark 电路。
- 多 verifier 委员会与阈值签名。
- 生产级 trusted setup ceremony。
- 完整数据集查询 DSL、真实脱敏规则引擎、批量 proof 聚合。

## Self Review

- 覆盖论文核心 ZK 路径：CH proof、RO proof、public input、proof verification、VerifierAuthority attestation。
- 覆盖当前平台边界：child6 只做数据交易 session 与 attestation 校验，主链 escrow 不直接接触 ZK proof。
- 明确了论文源码缺口：随机 witness、批量 demo 命令、`output/` 目录耦合。
- 明确了非目标：链上 Groth16 verifier、Rust/arkworks 重写、多 verifier。
- 没有要求实现 agent 修改无关 crowdsource/FMC/TMC 平台逻辑。
