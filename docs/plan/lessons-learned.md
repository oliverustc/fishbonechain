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

---

## Phase 1：核心 Pallet 开发（2026-05-31）

### 教训 8：Substrate pallet mock 里 Balance 类型必须用 u64，不能用 u128

**踩坑过程**：
- 在 `mock.rs` 里写 `type AccountData = pallet_balances::AccountData<u128>`
- 报错：`type mismatch resolving AccountData<u128> vs AccountData<u64>`
- `#[derive_impl(pallet_balances::config_preludes::TestDefaultConfig)]` 已经把 `Balance` 固定为 `u64`

**根因**：`TestDefaultConfig` 是 Substrate 提供的测试用 Config 实现，专门用 `u64` 做 Balance 类型（省空间、运算简单）。`AccountData<T>` 的泛型参数是 Balance 类型，必须跟 Currency 的 Balance 保持一致。

**规则**：写 pallet mock 时，`frame_system::Config` 里的 `AccountData` 写 `pallet_balances::AccountData<u64>`，测试用的 Balance 字面量也不要标注 `u128`，让编译器自动推导。

---

### 教训 9：新版 pallet_balances 的 GenesisConfig 增加了 `dev_accounts` 字段

**踩坑过程**：
- 直接复制旧写法 `pallet_balances::GenesisConfig::<Test> { balances: vec![...] }` 报错：
  `missing field dev_accounts in initializer of pallet_balances::GenesisConfig`
- 不是 API 破坏性变更，只是增加了一个有默认值的字段

**规则**：pallet_balances GenesisConfig 现在需要 `dev_accounts: None`：
```rust
pallet_balances::GenesisConfig::<Test> {
    balances: vec![(1, 100_000), ...],
    dev_accounts: None,
}
```

---

### 教训 10：Substrate 在 block 0 不注册事件，测试必须先 set_block_number(1)

**踩坑过程**：
- `System::assert_has_event(Event::ChainRegistered {...}.into())` 测试失败
- 报错：`WARNING: block number is zero, and events are not registered at block number zero`
- 12 个测试通过，2 个测试失败，只有涉及 event 断言的测试出错

**根因**：Substrate 的 `frame_system` 在 block 0（genesis）不初始化 event 存储，这是设计行为。

**规则**：所有测试的 `new_test_ext()` 函数结尾加一行：
```rust
let mut ext = sp_io::TestExternalities::new(storage);
ext.execute_with(|| frame_system::Pallet::<Test>::set_block_number(1));
ext
```
只要测试里有任何 event 断言，不加这行就会失败。

---

### 教训 11：Pallet 资金池账户向外转账要用 AllowDeath，否则余额为 0 时报 NotExpendable

**踩坑过程**：
- `fmc.submit_bill` 把账单金额恰好等于全部存款时报错：`Token(NotExpendable)`
- 原因：pallet pot account 余额降到 0，但 `ExistenceRequirement::KeepAlive` 不允许账户余额低于 existential deposit
- `KeepAlive` 和 `NotExpendable` 是不同错误：`KeepAlive` 发生在一般账户，`NotExpendable` 发生在系统账户触碰存活边界

**规则**：Pallet 资金托管账户（PalletId 派生）向用户转账必须用 `AllowDeath`：
```rust
T::Currency::transfer(
    &Self::account_id(),
    recipient,
    amount,
    ExistenceRequirement::AllowDeath,  // ← 不是 KeepAlive
)?;
```
只有 user → pallet 的充值方向才用 `KeepAlive`（防止用户账户被清空）。

---

### 教训 12：跨 pallet 接口用 blanket impl，不要在 runtime 里写手动适配

**踩坑过程**：
- `pallet-fmc` 定义了 `CcmcInterface<AccountId>` trait，runtime 里要把 `Ccmc` 类型传给 `type CcmcPallet`
- `type CcmcPallet = Ccmc;` 报错：`cannot find type Ccmc in this scope`（`Ccmc` 是 `lib.rs` 里的 runtime 别名，`configs/mod.rs` 里不可见）
- 即使修正了 import，`pallet_ccmc::Pallet<Runtime>` 也不实现 `CcmcInterface`，还要写 impl

**正确做法**：在 `pallet-fmc/src/lib.rs`（最外层，pallet 模块外）提供 blanket impl：
```rust
impl<T> pallet::CcmcInterface<T::AccountId> for pallet_ccmc::Pallet<T>
where
    T: frame_system::Config + pallet_ccmc::Config,
{
    fn is_miner(chain_id: ChainId, who: &T::AccountId) -> bool {
        pallet_ccmc::Pallet::<T>::is_miner(chain_id, who)
    }
    fn miner_count(chain_id: ChainId) -> u32 {
        pallet_ccmc::Pallet::<T>::miner_count(chain_id)
    }
}
```
然后 runtime 里直接写：
```rust
type CcmcPallet = pallet_ccmc::Pallet<Runtime>;
```
这是孤儿规则允许的（本地 trait + 外部类型），runtime 侧零样板代码。

---

### 教训 13：`--chain main-local` 是 LOCAL_TESTNET preset，单节点无法出块

**踩坑过程**：
- 用 `--chain main-local --alice --validator` 启动节点，`chain_getHeader` 返回 block 0
- 节点日志只有两行，没有出块日志
- E2E 脚本提交 extrinsic 卡住等待 InBlock

**根因**：`main-local` 使用 `LOCAL_TESTNET_RUNTIME_PRESET`，GRANDPA 需要 Alice + Bob 两个验证人达成 finality，单节点下 AURA 也不会出块（无 `--force-authoring`）。

**规则**：
- **单节点本地测试**：用 `--dev`（等价于 `--chain=dev --alice --validator --force-authoring --tmp`）
- **多节点本地测试**：用 `scripts/start-network.sh` 同时起 Alice + Bob
- **E2E 自动化脚本**：固定连 `--dev` 节点，不依赖多节点环境

---

### 教训 14：用固定 base-path 启动节点时必须显式提供 --node-key

**踩坑过程**：
- `fishbone-node --chain main-local --alice --base-path /tmp/fishbone/main` 报错：
  `NetworkKeyNotFound("/tmp/fishbone/main/chains/fishbone_main/network/secret_ed25519")`
- 第一次启动时还没有网络密钥文件

**规则**：有两种解决方案：
```bash
# 方案A：指定固定测试密钥（推荐，可复现）
--node-key 0000000000000000000000000000000000000000000000000000000000000001

# 方案B：先用 subkey 生成密钥，但 --tmp 避免了这个问题
./fishbone-node --dev --tmp  # --tmp 自动处理密钥生成
```

---

### Phase 1 有效工具和命令备忘

```bash
# 单元测试（跳过 WASM 构建，极快）
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc -p pallet-fmc

# 单独测试某个 pallet
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc -- tests::two_of_three_miners_confirm_epoch

# 只编译 runtime（不启动节点，验证集成）
SKIP_WASM_BUILD=1 cargo build -p fishbone-runtime

# 启动 dev 节点用于 E2E 测试
./target/release/fishbone-node --dev --base-path /tmp/fb-dev \
  --node-key 0000000000000000000000000000000000000000000000000000000000000001 \
  --rpc-port 9944 --rpc-cors all

# 运行 E2E 验证脚本
node --input-type=module < scripts/e2e-verify.js

# 快速检查元数据中的 pallet 名称
curl -s -X POST http://127.0.0.1:9944 \
  -H "Content-Type: application/json" \
  -d '{"id":1,"jsonrpc":"2.0","method":"state_getMetadata","params":[]}' \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
b = bytes.fromhex(d['result'][2:]).decode('utf-8', errors='ignore')
for name in ['Ccmc','Fmc','register_child_chain','submit_bill']:
    print(f'[{\"✓\" if name in b else \"✗\"}] {name}')
"
```

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
