#!/bin/bash
# FishboneChain 本地多链开发网络启动脚本
# 启动三个独立 solo chain 节点：主链 + 子链1 + 子链2
set -e

NODE=./target/release/fishbone-node
BASE=/tmp/fishbone

# 清理旧数据
rm -rf $BASE && mkdir -p $BASE/{main,child1,child2}

echo "============================"
echo " FishboneChain 本地网络"
echo "============================"
echo ""

# 主链（Alice，AURA 出块，6 秒/块，RPC 9944）
echo "[主链]   启动 fishbone_main (Alice, 9944)..."
$NODE \
  --chain main-local \
  --alice \
  --validator \
  --base-path $BASE/main \
  --port 30333 \
  --rpc-port 9944 \
  --prometheus-port 9615 \
  --node-key 0000000000000000000000000000000000000000000000000000000000000001 \
  --rpc-cors all \
  --log info \
  2>&1 | sed 's/^/[MAIN] /' &
MAIN_PID=$!

sleep 3

# 子链1（Bob，众包场景，RPC 9945）
echo "[子链1]  启动 fishbone_child_1 (Bob, 9945)..."
$NODE \
  --chain child1-local \
  --bob \
  --validator \
  --base-path $BASE/child1 \
  --port 30334 \
  --rpc-port 9945 \
  --prometheus-port 9616 \
  --node-key 0000000000000000000000000000000000000000000000000000000000000002 \
  --rpc-cors all \
  --log info \
  2>&1 | sed 's/^/[CH1] /' &
CHILD1_PID=$!

# 子链2（Dave，数据交易场景，RPC 9946）
echo "[子链2]  启动 fishbone_child_2 (Dave, 9946)..."
$NODE \
  --chain child2-local \
  --dave \
  --validator \
  --base-path $BASE/child2 \
  --port 30335 \
  --rpc-port 9947 \
  --prometheus-port 9617 \
  --node-key 0000000000000000000000000000000000000000000000000000000000000003 \
  --rpc-cors all \
  --log info \
  2>&1 | sed 's/^/[CH2] /' &
CHILD2_PID=$!

echo ""
echo "三节点已启动（Ctrl+C 停止所有节点）"
echo ""
echo "  主链  RPC: ws://127.0.0.1:9944  (chain: fishbone_main)"
echo "  子链1 RPC: ws://127.0.0.1:9945  (chain: fishbone_child_1)"
echo "  子链2 RPC: ws://127.0.0.1:9946  (chain: fishbone_child_2)"
echo ""
echo "  Polkadot.js: https://polkadot.js.org/apps"
echo "  连接方式: 自定义端点 → ws://127.0.0.1:9944"
echo ""

# 捕获 Ctrl+C，清理所有子进程
trap "echo ''; echo '正在停止所有节点...'; kill $MAIN_PID $CHILD1_PID $CHILD2_PID 2>/dev/null; exit 0" INT TERM

wait $MAIN_PID $CHILD1_PID $CHILD2_PID
