# Gnark abc

简单介绍gnark的使用

## 环境配置

1. go >= 1.23

```shell
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go 
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
```
编辑shell文件，添加环境变量即可

2. solc

```shell
wget https://github.com/ethereum/solidity/releases/download/v0.8.28/solc-static-linux
sudo install solc-static-linux /usr/local/bin/solc
```

3. abigen

```shell
go install github.com/ethereum/go-ethereum/cmd/abigen@latest
```