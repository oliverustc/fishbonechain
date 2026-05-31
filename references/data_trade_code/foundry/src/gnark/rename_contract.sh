#!/bin/bash

# gnark生成的验证合约的contract名都相同，这里需要对其进行重命名才能在一个合约中调用多个gnark验证合约
# 仅需运行一次

# 遍历所有sol文件
for file in $(find . -name "*.sol"); do
    # 获取basename
    basename=$(basename -s .sol "$file")
    # 如果basename中包含groth16
    if [[ $basename == *"groth16"* ]]; then
        # 将文件中的 "contract Verifier {" 替换为 "contract $basename {"
        sed -i "s/contract Verifier {/contract $basename {/g" "$file"
        echo "Renamed Verifier to $basename in $file"
    fi 
    # 如果basename中包含plonk
    if [[ $basename == *"plonk"* ]]; then
        # 将文件中的 "contract PlonkVerifier {" 替换为 "contract $basename {"
        sed -i "s/contract PlonkVerifier {/contract $basename {/g" "$file"
        echo "Renamed PlonkVerifier to $basename in $file"
    fi
done