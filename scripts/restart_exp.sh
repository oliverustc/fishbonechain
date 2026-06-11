#!/usr/bin/env bash
# 原子化重启资金流动性实验（6 条子链完整版）
# 充值 → 终止所有任务 → 重新激活 → 记录 EPOCH_OFFSET → 启动 metrics → 等待 30s → 启动 bridges + workers
#
# 子链布局：
#   child1 (ws://127.0.0.1:9945)       task=0  miners=f1,f2,f3          scenario a  workers=300
#   child2 (ws://10.2.2.14:9946)       task=1  miners=f4,f5,f6          scenario b  workers=2000
#   child3 (ws://10.2.2.17:9947)       task=2  miners=f7,f8,f9          scenario c  workers=200
#   child4 (ws://127.0.0.1:9948)       task=3  miners=f1-f6(6/8)        scenario d  workers=100
#   child5 (ws://10.2.2.20:9949)       task=4  miners=f10,f11,f12       scenario e  workers=500 512MB
#   child6 (ws://127.0.0.1:9950)       task=5  miners=f1-f4(4/6)        scenario f  workers=50  512MB
#
# 用法：cd /home/debian/fishbone && bash scripts/restart_exp.sh

set -euo pipefail

MAIN_WS="${MAIN_WS:-ws://127.0.0.1:9944}"
ALICE="5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"

# ── 验证人私钥（与 setup_experiment.js VALIDATORS 完全一致）─────────────────
F1="0x52390bf081065e3ff296ab72c42bc234cedbdf9ddc40b6c7b6aee5fd01e08880"
F2="0x17f21fff17006faad4fa003c3db215718fc4d3bbc86054a435666affe704e71b"
F3="0x4151713ff93e1333474f1380cb2bc4fce9183942790830c1e8f48d3752e232fc"
F4="0x8b7aeb4590e1607db466c3cea45b4096f0b912364da41689f1be5166df3fee83"
F5="0x69416e5975a353736603d14d57e13efdadab8dd11667498d7892588abf50e70a"
F6="0x91bd2803edfcbb7e8d7f06c7df94f98d26300e56b968ee51855d994e4308d1e8"
F7="0x92ed7c0c05a5b080b5193514043e2bbd33401e2428e50e85cfbd2a20558b5652"
F8="0xb9b4d65352af6ab4f5c7bf0b765d17053cb5e1c39868a8ff7b5600340f114d56"
F9="0xdd20f92b0d61c5dd4ba76aac2c7d2e9957746adc5dd45d4dc3339f42f7ae1c4b"
F10="0xcb829a57912d649c46808a673d2f466b9f954208ab15ddd748567af6bbf81082"
F11="0x6b006d6f22d84f120c61d9f4366bc0d2390472aad7b0345c17a51ccf3a1538d4"
F12="0xf20ecd8e0f4aabc67e991ca6be62522b37cdf256819498d84bafea34d5146817"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOG_DIR=~/exp_fund_logs
mkdir -p "$LOG_DIR"

echo "=== FishboneChain 资金流动性实验（6 链完整重启）==="
echo "  child1(9945)/child2(f4:9946)/child3(f7:9947)/child4(9948)/child5(f10:9949)/child6(9950)"

# ── Step 1：停止所有残留进程 ──────────────────────────────────────────────────
echo "[1/5] 停止残留进程..."
pgrep -f "metrics_fund.js" | xargs kill 2>/dev/null || true
pgrep -f "bridge.js"        | xargs kill 2>/dev/null || true
pgrep -f "worker.js"        | xargs kill 2>/dev/null || true
sleep 2

# ── Step 2：充值 + 终止所有任务 + 激活所有任务 + 记录 EPOCH_OFFSET ─────────
echo "[2/5] 充值并重置 6 个任务..."
rm -f /tmp/exp_epoch_offset.txt

node --input-type=module << 'EOF'
import { ApiPromise, WsProvider, Keyring } from "/home/debian/fishbone/scripts/node_modules/@polkadot/api/index.js";
import { writeFileSync } from "fs";

const MAIN_WS = process.env.MAIN_WS || "ws://127.0.0.1:9944";
const UNIT    = 1_000_000_000_000n;
const TASK_IDS = [0, 1, 2, 3, 4, 5];
// Alice native ≈ 251,721 UNIT，存入 200,000 UNIT（保留 ~50k 给 gas）
// terminate 后 locked 50,000 返回 free → 实际可用约 250,000 UNIT
const DEPOSIT_AMOUNT = 200_000n * UNIT;

function log(m) { console.log(`[reset] ${m}`); }

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const { name } = api.registry.findMetaError(dispatchError.asModule);
          log(`  [skip] ${label}: ${name}`); resolve(null); return;
        }
        reject(new Error(`${label}: ${dispatchError}`));
      } else if (status.isInBlock) {
        log(`  ✓ ${label}`); resolve(status.asInBlock);
      }
    }).catch(reject);
  });
}

const api   = await ApiPromise.create({ provider: new WsProvider(MAIN_WS) });
const kr    = new Keyring({ type: "sr25519" });
const alice = kr.addFromUri("//Alice");

log(`充值 300,000 UNIT...`);
await sendTx(api, api.tx.fmc.deposit(DEPOSIT_AMOUNT), alice, "fmc.deposit(300000 UNIT)");
await new Promise(r => setTimeout(r, 2000));

log("terminateTask 0-5...");
for (const id of TASK_IDS) {
  await sendTx(api, api.tx.fmc.terminateTask(id), alice, `terminateTask(${id})`);
  await new Promise(r => setTimeout(r, 800));
}
await new Promise(r => setTimeout(r, 2000));

log("activateTask 0-5...");
for (const id of TASK_IDS) {
  await sendTx(api, api.tx.fmc.activateTask(id), alice, `activateTask(${id})`);
  await new Promise(r => setTimeout(r, 800));
}
await new Promise(r => setTimeout(r, 2000));

// EPOCH_OFFSET = child4 (task 3) 的当前 epoch（进度基准链）
const task3 = await api.query.fmc.tasks(alice.address, 3);
const offset = task3.isSome ? Number(task3.unwrap().currentEpoch ?? 0) : 0;
log(`  EPOCH_OFFSET（child4 基准）= ${offset}`);

const poolRaw = await api.query.fmc.fundPools(alice.address);
const pool    = poolRaw.isSome ? poolRaw.unwrap() : poolRaw;
const freeU   = BigInt((pool.free ?? 0).toString()) / UNIT;
const lockU   = BigInt((pool.locked ?? 0).toString()) / UNIT;
log(`  pool: free=${freeU} UNIT  locked=${lockU} UNIT`);

writeFileSync("/tmp/exp_epoch_offset.txt", String(offset));
await api.disconnect();
process.exit(0);
EOF

if [[ ! -f /tmp/exp_epoch_offset.txt ]]; then
  echo "[ERROR] 未能获取 EPOCH_OFFSET，终止。"; exit 1
fi
EPOCH_OFFSET=$(cat /tmp/exp_epoch_offset.txt)
echo "[2/5] 完成  EPOCH_OFFSET=${EPOCH_OFFSET}  T_PLANNED=3（相对 child4 epoch）"

# ── Step 3：清理旧数据 ────────────────────────────────────────────────────────
echo "[3/5] 清理旧 CSV..."
[[ -f /tmp/exp_e_fund_state.csv ]] && \
  cp /tmp/exp_e_fund_state.csv "/tmp/exp_e_fund_state_backup_$(date +%Y%m%d_%H%M%S).csv"
rm -f /tmp/exp_e_fund_state.csv
rm -f "$LOG_DIR/metrics_fund.log"

# ── Step 4：立即启动 metrics（先于 bridge/worker，捕获 epoch=0 基线）─────────
echo "[4/5] 启动 metrics_fund（6 任务，interval=10s，REFERENCE_TASK_ID=3）..."
nohup env \
  MAIN_WS="${MAIN_WS}" \
  REQUESTER="${ALICE}" \
  TASK_IDS="0,1,2,3,4,5" \
  T_PLANNED="3" \
  EPOCH_OFFSET="${EPOCH_OFFSET}" \
  REFERENCE_TASK_ID="3" \
  node "${SCRIPT_DIR}/metrics_fund.js" \
    --out /tmp/exp_e_fund \
    --interval 10 \
  > "$LOG_DIR/metrics_fund.log" 2>&1 &
METRICS_PID=$!
echo "  metrics PID=${METRICS_PID}"

echo "  等待 30s 采集初始基线（epoch=0 状态）..."
sleep 30

# ── Step 5：启动 6 条链的 bridges + workers ────────────────────────────────
echo "[5/5] 启动 bridges 和 workers..."

# ── bridges ──────────────────────────────────────────────────────────────────
# child1  task=0  chain=0  miners: f1,f2,f3（4 miners registered，threshold=3）
nohup env CHILD_WS="ws://127.0.0.1:9945" MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F1},${F2},${F3}" REQUESTER="${ALICE}" \
  TASK_ID="0" CHAIN_ID="0" \
  node "${SCRIPT_DIR}/bridge.js" > "$LOG_DIR/bridge_child1.log" 2>&1 &
echo "  bridge@child1 PID=$!"

# child2  task=1  chain=1  miners: f4,f5,f6（4 miners registered，threshold=3）
nohup env CHILD_WS="ws://10.2.2.14:9946" MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F4},${F5},${F6}" REQUESTER="${ALICE}" \
  TASK_ID="1" CHAIN_ID="1" \
  node "${SCRIPT_DIR}/bridge.js" > "$LOG_DIR/bridge_child2.log" 2>&1 &
echo "  bridge@child2 PID=$!"

# child3  task=2  chain=2  miners: f7,f8,f9（4 miners registered，threshold=3）
nohup env CHILD_WS="ws://10.2.2.17:9947" MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F7},${F8},${F9}" REQUESTER="${ALICE}" \
  TASK_ID="2" CHAIN_ID="2" \
  node "${SCRIPT_DIR}/bridge.js" > "$LOG_DIR/bridge_child3.log" 2>&1 &
echo "  bridge@child3 PID=$!"

# child4  task=3  chain=3  miners: f1-f6（8 miners registered，threshold=6，提供6个）
nohup env CHILD_WS="ws://127.0.0.1:9948" MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F1},${F2},${F3},${F4},${F5},${F6}" REQUESTER="${ALICE}" \
  TASK_ID="3" CHAIN_ID="3" \
  node "${SCRIPT_DIR}/bridge.js" > "$LOG_DIR/bridge_child4.log" 2>&1 &
echo "  bridge@child4 PID=$!"

# child5  task=4  chain=4  miners: f10,f11,f12（4 miners registered，threshold=3）
nohup env CHILD_WS="ws://10.2.2.20:9949" MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F10},${F11},${F12}" REQUESTER="${ALICE}" \
  TASK_ID="4" CHAIN_ID="4" \
  node "${SCRIPT_DIR}/bridge.js" > "$LOG_DIR/bridge_child5.log" 2>&1 &
echo "  bridge@child5 PID=$!"

# child6  task=5  chain=5  miners: f1-f4（6 miners registered，threshold=4，提供4个）
nohup env CHILD_WS="ws://127.0.0.1:9950" MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F1},${F2},${F3},${F4}" REQUESTER="${ALICE}" \
  TASK_ID="5" CHAIN_ID="5" \
  node "${SCRIPT_DIR}/bridge.js" > "$LOG_DIR/bridge_child6.log" 2>&1 &
echo "  bridge@child6 PID=$!"

sleep 1

# ── workers ───────────────────────────────────────────────────────────────────
# child1: scenario a，300 workers，本机
nohup node "${SCRIPT_DIR}/worker.js" --scenario a \
  --ws ws://127.0.0.1:9945 --task-id 0 \
  > "$LOG_DIR/worker_child1.log" 2>&1 &
echo "  worker@child1 (300w) PID=$!"

# child2: scenario b，2000 workers，连接 f4
nohup node "${SCRIPT_DIR}/worker.js" --scenario b \
  --ws ws://10.2.2.14:9946 --task-id 1 \
  > "$LOG_DIR/worker_child2.log" 2>&1 &
echo "  worker@child2 (2000w) PID=$!"

# child3: scenario c，200 workers，连接 f7
nohup node "${SCRIPT_DIR}/worker.js" --scenario c \
  --ws ws://10.2.2.17:9947 --task-id 2 \
  > "$LOG_DIR/worker_child3.log" 2>&1 &
echo "  worker@child3 (200w) PID=$!"

# child4: scenario d，100 workers，本机
nohup node "${SCRIPT_DIR}/worker.js" --scenario d \
  --ws ws://127.0.0.1:9948 --task-id 3 \
  > "$LOG_DIR/worker_child4.log" 2>&1 &
echo "  worker@child4 (100w) PID=$!"

# child5: scenario e，500 workers（原5000，限内存），连接 f10
nohup node --max-old-space-size=512 "${SCRIPT_DIR}/worker.js" --scenario e --workers 500 \
  --ws ws://10.2.2.20:9949 --task-id 4 \
  > "$LOG_DIR/worker_child5.log" 2>&1 &
echo "  worker@child5 (500w, 512MB) PID=$!"

# child6: scenario f，50 workers（限内存），本机
nohup node --max-old-space-size=512 "${SCRIPT_DIR}/worker.js" --scenario f --workers 50 \
  --ws ws://127.0.0.1:9950 --task-id 5 \
  > "$LOG_DIR/worker_child6.log" 2>&1 &
echo "  worker@child6 (50w, 512MB) PID=$!"

echo ""
echo "=== 实验已启动（6 链）==="
echo "  EPOCH_OFFSET=${EPOCH_OFFSET}  T_PLANNED=3（相对 child4 epoch）"
echo "  预计 ~60min 完成 3 个 child4 周期"
echo "  CSV：/tmp/exp_e_fund_state.csv"
echo "  监控：tail -f $LOG_DIR/metrics_fund.log"
echo "  事件：grep BillSettled $LOG_DIR/metrics_fund.log"
echo ""
echo "  停止全部：pgrep -f 'metrics_fund.js|bridge.js|worker.js' | xargs kill"
