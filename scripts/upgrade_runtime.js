/**
 * 通过 sudo 升级各子链的 WASM runtime（不重置链状态）
 *
 * 用法：
 *   node scripts/upgrade_runtime.js [--chain ws://...]
 *   WASM_PATH=... node scripts/upgrade_runtime.js --chain ws://localhost:19945
 *
 * 默认升级所有 6 条子链。
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { readFileSync } from "fs";
import { resolve } from "path";
import { fileURLToPath } from "url";
import { dirname } from "path";

const __dir = dirname(fileURLToPath(import.meta.url));
const WASM_PATH = process.env.WASM_PATH ||
  resolve(__dir, "../target/release/wbuild/fishbone-runtime/fishbone_runtime.compact.compressed.wasm");

const ONLY_CHAIN = process.argv.includes("--chain")
  ? process.argv[process.argv.indexOf("--chain") + 1]
  : null;

const CHAINS = [
  { name: "child1", ws: process.env.CHILD1_WS || "ws://localhost:19945" },
  { name: "child2", ws: process.env.CHILD2_WS || "ws://localhost:19946" },
  { name: "child3", ws: process.env.CHILD3_WS || "ws://localhost:19947" },
  { name: "child4", ws: process.env.CHILD4_WS || "ws://localhost:19948" },
  { name: "child5", ws: process.env.CHILD5_WS || "ws://localhost:19949" },
  { name: "child6", ws: process.env.CHILD6_WS || "ws://localhost:19950" },
];

function log(msg) { console.log(`[upgrade ${new Date().toISOString()}] ${msg}`); }

async function upgradeChain(ws, name, wasm, alice) {
  log(`连接 ${name} (${ws})...`);
  const api = await ApiPromise.create({ provider: new WsProvider(ws) });

  const currentSpec = (await api.rpc.state.getRuntimeVersion()).specVersion.toNumber();
  log(`  当前 specVersion = ${currentSpec}`);

  // setCodeWithoutChecks 跳过 spec_version 递增校验，适合实验环境
  const tx = api.tx.sudo.sudo(
    api.tx.system.setCodeWithoutChecks(wasm)
  );

  await new Promise((resolve, reject) => {
    tx.signAndSend(alice, ({ status, dispatchError, events }) => {
      if (dispatchError) {
        reject(new Error(`setCode failed: ${dispatchError}`));
      } else if (status.isInBlock) {
        // Check for sudo event
        for (const { event } of events) {
          if (api.events.sudo?.Sudid?.is(event)) {
            const result = event.data[0];
            if (result.isErr) {
              reject(new Error(`sudo failed: ${result.asErr}`));
              return;
            }
          }
        }
        log(`  ✓ setCode 已上链 block=${status.asInBlock}`);
        resolve();
      }
    }).catch(reject);
  });

  // 等待运行时生效（下一个块）
  log(`  等待新块以确认升级生效...`);
  await new Promise(resolve => {
    const unsub = api.rpc.chain.subscribeNewHeads(() => {
      unsub.then(u => u());
      resolve();
    }).catch(resolve);
  });

  const newSpec = (await api.rpc.state.getRuntimeVersion()).specVersion.toNumber();
  log(`  新 specVersion = ${newSpec}`);

  const pallets = Object.keys(api.tx).join(", ");
  const hasCrowdsource = pallets.includes("crowdsource");
  log(`  crowdsource pallet: ${hasCrowdsource ? "✅" : "❌"}`);

  await api.disconnect();
  return hasCrowdsource;
}

async function main() {
  log("=== Runtime 升级脚本 ===");
  log(`WASM: ${WASM_PATH}`);

  const wasmBytes = readFileSync(WASM_PATH);
  const wasm = "0x" + wasmBytes.toString("hex");
  log(`WASM 大小: ${(wasmBytes.length / 1024).toFixed(0)} KB`);

  const keyring = new Keyring({ type: "sr25519" });
  const alice   = keyring.addFromUri("//Alice");
  log(`Alice = ${alice.address}`);

  const chains = ONLY_CHAIN
    ? CHAINS.filter(c => c.ws === ONLY_CHAIN)
    : CHAINS;

  let ok = 0;
  for (const chain of chains) {
    try {
      const success = await upgradeChain(chain.ws, chain.name, wasm, alice);
      if (success) ok++;
    } catch (e) {
      log(`  [错误] ${chain.name}: ${e.message}`);
    }
  }

  log(`\n升级完成: ${ok}/${chains.length} 条链已包含 crowdsource pallet`);
}

main().catch(e => {
  console.error("[upgrade 致命错误]", e.message);
  process.exit(1);
});
