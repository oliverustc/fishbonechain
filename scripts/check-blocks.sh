#!/bin/bash
# 检查三个节点的出块状态
echo "=== FishboneChain 节点状态 ==="
echo ""

for name_port in "主链:9944" "子链1:9945" "子链2:9947"; do
    name="${name_port%%:*}"
    port="${name_port##*:}"

    result=$(curl -s -X POST \
      -H "Content-Type: application/json" \
      -d '{"id":1,"jsonrpc":"2.0","method":"chain_getHeader","params":[]}' \
      "http://127.0.0.1:$port" 2>/dev/null)

    if [ -z "$result" ]; then
        echo "  $name (port $port): 节点未响应"
    else
        block_num=$(echo "$result" | python3 -c "import sys,json; h=json.load(sys.stdin); print(int(h['result']['number'],16))" 2>/dev/null)
        if [ -z "$block_num" ]; then
            echo "  $name (port $port): 响应异常"
        else
            echo "  $name (port $port): 块高 = $block_num"
        fi
    fi
done
