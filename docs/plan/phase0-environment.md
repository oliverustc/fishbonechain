# Phase 0：本地多链开发环境搭建

**状态**：✅ 完成  
**方案**：Route B — Rust stable (1.96) + polkadot-sdk-solochain-template  
**目标**：在本地运行一个主链节点 + 两个子链节点，三者独立出块，为后续业务逻辑开发提供基础。

---

## 一、版本基线（所有组件必须内部一致）

| 组件 | 版本 | 说明 |
|------|------|------|
| Rust | **stable 1.96**（已就绪）| 满足 alloy 要求的 ≥1.91，当前 stable |
| Polkadot SDK | **polkadot-stable2512-3** | cookbook `versions.yml` 中验证过的最新稳定版 |
| Parachain Template | **v0.0.5** | 与上述 SDK 配套的官方模板 |
| polkadot-omni-node | **0.14.0** | 需重新安装（当前 0.5.0 不匹配）|
| chain-spec-builder | **16.0.0** | 需重新安装（当前 10.0.0 不匹配）|
| zombienet | **v1.3.138** | 需安装 |

---

## 二、架构策略（分两步走）

**Step A（本 Phase）：多 Solo Chain 模式**

用同一份节点代码跑三个实例（主链 + 子链1 + 子链2），通过不同的 chain spec 区分。三链各自独立出块。应用层通过 RPC 调用实现跨链交互，无需引入 XCM 复杂性。

这样做的原因：
- `polkadot-sdk-parachain-template` 本身是 parachain 模式（依赖 relay chain），需要额外的 relay chain 才能出块
- Solo chain 模式使用 AURA+GRANDPA，无需 relay chain，开发迭代最快
- 先把 pallet 业务逻辑跑通，再迁移到 parachain 架构

**Step B（Phase 1 结束后）：迁移到 Relay+Parachain**

待核心 pallet（CCMC、FMC、task）稳定后，引入本地 relay chain，将主链和子链升级为 parachain，启用 XCM 跨链通信。

---

## 三、实现步骤

### Step 1：清理之前的错误起点

之前基于 `polkadot-v1.9.0` 的 substrate-node-template 复制的代码结构已过时，删除重建。

```bash
cd /home/swt/fishbonechain

# 删除错误复制来的文件
rm -rf node runtime pallets Cargo.lock
# 保留 docs/ 和 references/

# 确认保留的内容
ls
# 应只剩：Cargo.toml docs/ references/ rust-toolchain.toml rustfmt.toml scripts/ zombienet/
```

同时恢复 `Cargo.toml` 为 workspace 占位文件，后续按模板结构重建。

### Step 2：安装/更新匹配的工具链

```bash
# 更新 chain-spec-builder 到 16.0.0（会覆盖现有 10.0.0）
cargo install --locked staging-chain-spec-builder@16.0.0

# 更新 polkadot-omni-node 到 0.14.0（会覆盖现有 0.5.0）
cargo install --locked polkadot-omni-node@0.14.0

# 安装 zombienet
ZOMBIENET_VERSION=v1.3.138
wget -q https://github.com/paritytech/zombienet/releases/download/${ZOMBIENET_VERSION}/zombienet-linux-x64 \
  -O /home/swt/.cargo/bin/zombienet
chmod +x /home/swt/.cargo/bin/zombienet
zombienet --version

# 验证全部版本
rustc --version           # 1.96.x
chain-spec-builder --version   # 16.0.0
polkadot-omni-node --version   # 0.14.x
zombienet --version       # 1.3.138
```

### Step 3：克隆 polkadot-sdk-parachain-template

```bash
cd /home/swt/fishbonechain

# 克隆官方 parachain template，切换到 v0.0.5
git clone https://github.com/paritytech/polkadot-sdk-parachain-template.git \
  --branch v0.0.5 --depth 1 \
  parachain-template-ref

# 查看其结构和 rust-toolchain.toml
cat parachain-template-ref/rust-toolchain.toml
cat parachain-template-ref/Cargo.toml | head -30
ls parachain-template-ref/
```

> **注意**：这个 clone 只是参考和验证用。实际项目代码在 `/home/swt/fishbonechain/` 下自建。

### Step 4：验证模板可以编译（确认基线可用）

```bash
cd /home/swt/fishbonechain/parachain-template-ref

# 用 stable 1.96 编译（不用 --locked，让 cargo 解析兼容的依赖版本）
cargo build --release 2>&1 | tail -20
```

如果编译成功，说明 Rust 1.96 + 这个模板完全兼容，基线确立。

### Step 5：建立 fishbonechain 项目结构

参考 parachain template，但使用 **substrate-node-template 风格的 AURA+GRANDPA** 模式（solo chain，无需 relay chain 出块）。

```bash
mkdir -p /home/swt/fishbonechain/{node/src,runtime/src,pallets/template/src,scripts,zombienet}
```

**关键：使用 polkadot-stable2512-3 的依赖版本**，而不是 v1.9.0。

`Cargo.toml`（workspace 根）：
```toml
[workspace.package]
authors   = ["FishboneChain"]
edition   = "2021"
license   = "MIT-0"
repository = "https://github.com/oliverustc/fishbonechain"

[workspace]
members = [
    "node",
    "runtime",
    "pallets/template",
]
resolver = "2"
```

`rust-toolchain.toml`（更新）：
```toml
[toolchain]
channel  = "stable"          # 1.96
targets  = ["wasm32-unknown-unknown"]
components = ["rustfmt", "clippy", "rust-src"]
profile  = "minimal"
```

### Step 6：从 stable2512-3 引入依赖构建节点

将 parachain-template-ref 中对 polkadot-sdk 的依赖引用（git tag = `polkadot-stable2512-3`）移植到 fishbonechain 的各 Cargo.toml 中。

核心依赖示例（node/Cargo.toml）：
```toml
[dependencies]
# 以 polkadot-stable2512-3 为 tag
sc-cli        = { git = "https://github.com/paritytech/polkadot-sdk", tag = "polkadot-stable2512-3" }
sc-service    = { git = "https://github.com/paritytech/polkadot-sdk", tag = "polkadot-stable2512-3" }
sc-consensus-aura    = { git = "...", tag = "polkadot-stable2512-3" }
sc-consensus-grandpa = { git = "...", tag = "polkadot-stable2512-3" }
# ...其余 substrate 客户端依赖
fishbone-runtime = { path = "../runtime" }
```

### Step 7：定义三套 chain spec

在 `node/src/chain_spec.rs` 中定义：
- `main_chain_local_config()` — 主链，chain_id = `fishbone_main`，Alice+Bob 为验证者，6 秒出块
- `child_chain1_local_config()` — 子链1（众包），chain_id = `fishbone_child_1`，Bob+Charlie 为验证者，3 秒出块
- `child_chain2_local_config()` — 子链2（数据交易），chain_id = `fishbone_child_2`，Dave+Eve 为验证者，3 秒出块

`command.rs` 中 `load_spec` 匹配：
```rust
"main-local"   => main_chain_local_config(),
"child1-local" => child_chain1_local_config(),
"child2-local" => child_chain2_local_config(),
```

### Step 8：编译

```bash
cd /home/swt/fishbonechain
cargo build --release -p fishbone-node 2>&1 | tail -5
```

产物：`target/release/fishbone-node`

### Step 9：生成 chain spec 并编写启动脚本

```bash
NODE=./target/release/fishbone-node

# 生成三套 raw chain spec
$NODE build-spec --chain main-local   --disable-default-bootnode --raw > zombienet/main-raw.json
$NODE build-spec --chain child1-local --disable-default-bootnode --raw > zombienet/child1-raw.json
$NODE build-spec --chain child2-local --disable-default-bootnode --raw > zombienet/child2-raw.json
```

`scripts/start-network.sh`：
```bash
#!/bin/bash
set -e
NODE=./target/release/fishbone-node
BASE=/tmp/fishbone

# 清理旧数据
rm -rf $BASE && mkdir -p $BASE/{main,child1,child2}

echo "启动主链（Alice, 9944）..."
$NODE --chain zombienet/main-raw.json \
  --alice --validator \
  --base-path $BASE/main --port 30333 --rpc-port 9944 \
  --node-key 0000000000000000000000000000000000000000000000000000000000000001 \
  --log info &
MAIN_PID=$!

sleep 3
BOOT="/ip4/127.0.0.1/tcp/30333/p2p/12D3KooWEyoppNCUx8Yx66oV9fJnriXwCZXwDkoIoQUZspnNaqHX"

echo "启动子链1（Bob, 9945）..."
$NODE --chain zombienet/child1-raw.json \
  --bob --validator \
  --base-path $BASE/child1 --port 30334 --rpc-port 9945 \
  --log info &
CHILD1_PID=$!

echo "启动子链2（Dave, 9946）..."
$NODE --chain zombienet/child2-raw.json \
  --dave --validator \
  --base-path $BASE/child2 --port 30335 --rpc-port 9946 \
  --log info &
CHILD2_PID=$!

echo ""
echo "三节点已启动（Ctrl+C 停止）："
echo "  主链  RPC: ws://127.0.0.1:9944"
echo "  子链1 RPC: ws://127.0.0.1:9945"
echo "  子链2 RPC: ws://127.0.0.1:9946"
echo "  Polkadot.js: https://polkadot.js.org/apps"

# 等待任意节点退出
wait $MAIN_PID $CHILD1_PID $CHILD2_PID
```

---

## 四、验证方式

### 验证 1：编译成功

```bash
ls -lh target/release/fishbone-node
# 预期：存在，大小约 200~400 MB
```

### 验证 2：三节点独立出块

```bash
# 等待 30 秒后检查块高
for port in 9944 9945 9946; do
  echo -n "port $port 块高: "
  curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":1,"jsonrpc":"2.0","method":"chain_getHeader","params":[]}' \
    http://127.0.0.1:$port | python3 -c "import sys,json; h=json.load(sys.stdin); print(int(h['result']['number'],16))"
done
# 预期：三个节点各自有递增的块高（数值不同没关系，各自独立）
```

### 验证 3：通过 Polkadot.js Apps 查看

访问 https://polkadot.js.org/apps，分别连接三个 RPC，确认：
- 链名称正确（Fishbone Main / Child-1 / Child-2）
- 块高在增长
- 创世账户有余额

### 验证 4：主链转账测试

在 Polkadot.js Apps 上，从 Alice 向 Bob 发一笔转账，确认交易被打包、余额变化正确。

---

## 五、完成标准

- [ ] `parachain-template-ref` 编译成功（基线验证）
- [ ] `fishbone-node` 编译成功（Rust 1.96 + stable2512-3 依赖）
- [ ] `chain-spec-builder@16.0.0` 安装就绪
- [ ] `polkadot-omni-node@0.14.0` 安装就绪
- [ ] `zombienet@v1.3.138` 安装就绪
- [ ] 主链节点启动，持续出块（6 秒/块）
- [ ] 子链1节点启动，持续出块（3 秒/块）
- [ ] 子链2节点启动，持续出块（3 秒/块）
- [ ] Polkadot.js Apps 可连接三条链
- [ ] 主链转账测试通过

---

## 六、执行记录与实际偏差（2026-05-31 完成）

**实际方案偏差**：plan 中预设用 `polkadot-stable2512-3` git 依赖，实际改用官方独立仓库 `polkadot-sdk-solochain-template`（直接 clone，依赖 crates.io 版本），避免了 git 大量下载和版本冲突。

**关键问题与解决方案**：

1. **WASM 链接器**：`WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"` → 固化到 Makefile
2. **yanked crate**：core2 0.4.0 被撤销 → 复制 solochain template 的 Cargo.lock
3. **包名冲突**：fishbonechain 和 references 里的模板同名 → Python 批量重命名
4. **RPC 端口**：9946 被 TIME_WAIT 占用 → 改用 9947

**完成标准核验**：

- [x] `cargo build --release` 成功（8分14秒），产出 `fishbone-node` 二进制
- [x] 四套 chain spec 正确（fishbone_main_dev / fishbone_main / fishbone_child_1 / fishbone_child_2）
- [x] 主链节点启动，正常出块（RPC 9944）
- [x] 子链1节点启动，正常出块（RPC 9945）
- [x] 子链2节点启动，正常出块（RPC 9947）
- [x] 三链各自独立，genesis hash 不同
- [x] `scripts/start-network.sh` 和 `scripts/check-blocks.sh` 可用

**验证命令**：
```bash
bash scripts/start-network.sh &
sleep 20
bash scripts/check-blocks.sh
# 输出：主链/子链1/子链2 块高均递增
```
