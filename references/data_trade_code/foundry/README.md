# solidity project for paper `A Customizable and Verifiable Masking Data Trading Scheme based on zk-SNARK and Smart Contract`

## 依赖

### [foundry](https://book.getfoundry.sh/)
  使用安装脚本即可

```shell
# 安装foundry
curl -L https://foundry.paradigm.xyz | bash
# 运行foundryup，安装foundry工具链
foundryup
```

此安装脚本会在`${HOME}/.foundry/bin`文件夹中安装四个二进制文件，需将此路径添加到环境变量。

- `forge`: 负责编译、测试、生成文档、生成包、发布等
- `anvil`: 负责本地开发
- `cast`: 负责调用
- `chisel`: 负责部署

### [foundry-zkSync](https://github.com/matter-labs/foundry-zksync/releases/tag/foundry-zksync-v0.0.10)
  安装好 `foundry` 后，不能再使用 `foundry-zkSync` 的脚本安装`foundry-zksync`，`foundry-zksync`会覆盖 `foundry` 的二进制文件，因此选择手动安装

```shell
# 下载压缩包，自行核定最新版本
wget https://github.com/matter-labs/foundry-zksync/releases/download/foundry-zksync-v0.0.10/foundry_zksync_v0.0.10_linux_amd64.tar.gz
# 解压，得到两个二进制文件 forge和cast
tar -xzvf foundry_zksync_v0.0.10_linux_amd64.tar.gz
# 安装
install ./forge ~/.foundry/bin/zforge
install ./cast ~/.foundry/bin/zcast

# 下载anvil-zksync压缩包，自行核定最新版本
wget https://github.com/matter-labs/anvil-zksync/releases/download/v0.3.2/anvil-zksync-v0.3.2-x86_64-unknown-linux-gnu.tar.gz
# 解压，得到二进制文件 anvil
tar -xzvf anvil-zksync-v0.3.2-x86_64-unknown-linux-gnu.tar.gz
# 安装
install ./anvil ~/.foundry/bin/zanvil
```
