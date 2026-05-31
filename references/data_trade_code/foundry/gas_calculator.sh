#!/bin/bash

# 以太坊和zkSync gas计算器
# 计算在以太坊和zkSync上的gas花费

# 检查是否提供了参数
if [ $# -eq 0 ]; then
    echo "使用方法: $0 <gas数量>"
    exit 1
fi

# 获取gas数量
gas_amount=$1

# 检查输入是否为数字
if ! [[ "$gas_amount" =~ ^[0-9]+$ ]]; then
    echo "错误: 请输入有效的gas数量（整数）"
    exit 1
fi

# 固定gas价格（gwei）
eth_gas_price=1.372
zksync_gas_price=0.10

# 计算以太坊ETH花费
# 1 gwei = 10^-9 ETH
eth_cost=$(echo "$gas_amount * $eth_gas_price * 0.000000001" | bc -l)

# 计算zkSync ETH花费
zksync_cost=$(echo "$gas_amount * $zksync_gas_price * 0.000000001" | bc -l)

# 格式化输出，保留18位小数（以太坊的精度）
formatted_eth=$(printf "%.18f" $eth_cost)
formatted_zksync=$(printf "%.18f" $zksync_cost)

# 计算美元价值（假设ETH价格为$1800，您可以根据需要更改）
eth_price=1800
eth_usd_value=$(echo "$eth_cost * $eth_price" | bc -l)
zksync_usd_value=$(echo "$zksync_cost * $eth_price" | bc -l)
formatted_eth_usd=$(printf "%.2f" $eth_usd_value)
formatted_zksync_usd=$(printf "%.2f" $zksync_usd_value)

# 计算节省比例
savings_percent=$(echo "scale=2; (($eth_cost - $zksync_cost) / $eth_cost) * 100" | bc -l)

# 输出结果
echo "==========================================="
echo "      Gas费用计算结果比较"
echo "==========================================="
echo "Gas数量: $gas_amount"
echo ""
echo "以太坊 (L1):"
echo "  Gas价格: $eth_gas_price gwei"
echo "  ETH花费: $formatted_eth ETH"
echo "  美元价值: \$$formatted_eth_usd (基于ETH=$eth_price)"
echo ""
echo "zkSync (L2):"
echo "  Gas价格: $zksync_gas_price gwei"
echo "  ETH花费: $formatted_zksync ETH"
echo "  美元价值: \$$formatted_zksync_usd (基于ETH=$eth_price)"
echo ""
echo "使用zkSync节省: $savings_percent%"
echo "==========================================="
