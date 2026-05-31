# 经验教训记录

记录实际开发过程中踩过的坑和有效做法，供后续阶段参考。

---

## Phase 0：多链开发环境搭建（2026-05-31）

### 教训 1：永远先找官方独立模板，不要自己写或从 mono repo 提取

**踩坑过程**：
- 第一次用了旧的 `substrate-node-template`（polkadot-v1.9.0）→ `#[no_mangle]` 在 Rust 1.82+ 报错
- 第二次尝试从 polkadot-sdk mono repo 提取 solochain template → 依赖是 workspace 内部路径，完全不能独立编译
- 第三次从 parachain template 手动删减 cumulus → 删不干净，40+ 编译错误
- **正确做法**：`git clone https://github.com/paritytech/polkadot-sdk-solochain-template.git`，7 分钟编译成功

**规则**：Polkadot 生态有三个官方独立模板仓库，遇到新需求先去这三个找：
- `polkadot-sdk-solochain-template` — AURA+GRANDPA solo chain
- `polkadot-sdk-parachain-template` — Cumulus parachain
- `polkadot-sdk-minimal-template` — 最小化运行时

---

### 教训 2：Rust 版本选择要靠数据，不要靠文档描述

**踩坑过程**：
- 文档说"用 stable"，文档又说"用 1.88"，两者矛盾，浪费了大量时间争论
- **实际情况**：polkadot-sdk-solochain-template 没有 `rust-toolchain.toml`，直接用系统 stable（1.96）正常编译

**规则**：直接看目标模板仓库里有没有 `rust-toolchain.toml`。有就照用，没有就用当前 stable。不要看官网文档的 Rust 版本要求，那往往跟不上实际。

---

### 教训 3：WASM 编译的 `--allow-undefined` 是 polkadot-sdk 2512 的已知问题，直接加不要纠结

**踩坑过程**：
- 遇到 `undefined symbol: ext_hashing_blake2_256_version_1` 花了大量时间排查
- 试过 `.cargo/config.toml`（无效）、切换 `wasm32v1-none` target（无效）
- **正确解法**：`WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"` 固化到 Makefile

**根因**：sp-io 28.x（SDK 2512 自带）的 WASM builder 对 Rust 1.82+ 的链接器行为不兼容，新版 SDK 已修复。这是已知 bug，不是环境问题。

**规则**：凡是用 polkadot-sdk 2512 系列 + Rust 1.82+，直接在 Makefile 里加 `WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"`，无需深究。

---

### 教训 4：Cargo.lock 从能编译的参考实现里复制，不要重新解析

**踩坑过程**：
- 新建项目没有 Cargo.lock，Cargo 自动解析时命中了 `core2 0.4.0`（已被 yanked）
- 报错：`version 0.4.0 is yanked`，完全无法编译

**正确做法**：直接复制 solochain template 的 Cargo.lock 到项目根目录，依赖解析问题立刻消失。

**规则**：新项目第一次编译前，先从编译成功的参考实现里复制 Cargo.lock。

---

### 教训 5：包名不能和 references/ 里的模板重名

**踩坑过程**：
- fishbonechain 包名是 `solochain-template-node`，references/ 里也有同名包
- `cargo build -p solochain-template-node` 选中了 references/ 里的包，修改的代码完全无效
- 花了很长时间才发现二进制里根本没有我们的 `main-dev`/`child1-local` 逻辑

**规则**：从模板复制代码后**立即重命名所有包**（`fishbone-node`、`fishbone-runtime` 等），更新 Cargo.toml、所有 `use` 语句、Cargo.lock。用脚本批量替换，不要手动一个个改。

---

### 教训 6：Substrate 的 RPC 端口在 TIME_WAIT 期间会自动换端口

**踩坑过程**：
- 重启节点后 child2 的 `--rpc-port 9946` 被忽略，RPC 绑到了随机端口 42247
- 原因：上一次运行的 TIME_WAIT socket 仍占用 9946，Substrate 静默换端口

**规则**：
1. 节点端口之间留出间隔（9944、9945、9947，跳过 9946）
2. 重启网络前等待足够时间让 TIME_WAIT 清空（约 60 秒）
3. 用 `ss -tlnp` 确认端口真正空闲后再启动

---

### 教训 7：验证思路——先编译参考实现，再改名适配

**有效工作流**：
```
1. 找到官方模板仓库 → clone
2. 直接编译，确认基线可用（不改任何代码）
3. 复制代码到项目目录
4. 立即重命名包名（防止 Cargo 混用）
5. 做最小改动（chain spec、包名引用）
6. 编译，预期只有少量错误
```

**反面教训**：不要边写边猜 API。polkadot-sdk 的 API 版本变化快，靠记忆写的代码必然有大量错误。

---

### 有效工具和命令备忘

```bash
# 查看节点实际监听的端口
ss -tlnp | grep fishbone

# 查看节点日志里实际绑定的 RPC 端口（Substrate 可能换端口）
grep "Running JSON-RPC" /tmp/fishbone-network.log

# 强制重编译（touch 更改的文件）
touch node/src/command.rs && cargo build --release

# WASM 构建标志（必须加）
WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined" cargo build --release

# 确认 Cargo 选中了正确的包
strings ./target/release/fishbone-node | grep "fishbone\|FishboneChain" | head -3
```
