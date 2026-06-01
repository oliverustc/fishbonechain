# 多节点真实部署计划

**目标**：在 f1-f6（10.2.2.11-16）6 台机器上部署并管理真实多链环境  
**状态**：三链已运行，管理框架完善中

---

## 一、网络拓扑

```
主链（fishbone_main）：f1 f2 f3 f4 f5 f6  — 6 validator，端口 30333 / RPC 9944
子链1（fishbone_child_1）：f1 f2 f3         — 3 validator，端口 30334 / RPC 9945  
子链2（fishbone_child_2）：f4 f5 f6         — 3 validator，端口 30335 / RPC 9946
```

**访问路径**：开发机 → ProxyJump via `bcg`（192.168.8.41）→ f1-f6（10.2.2.11-16）  
**RPC 查询**：需从 bcg 或内网机器发出，开发机不能直连 10.2.2.x

---

## 二、已完成

- [x] 6 台机器清理（/home/debian 只剩 backup/ 和 go/），Docker 全清
- [x] apt upgrade 所有节点（f1/f2 由用户操作，f3-f6 本次执行）
- [x] fishbone-node 二进制推送到所有机器（/home/debian/fishbone/bin/）
- [x] 3 套 chain spec 生成（main/child1/child2，含 6 台机器的真实 validator 密钥）
- [x] chain spec 推送到所有机器（/home/debian/fishbone/specs/）
- [x] 各节点 P2P 密钥生成（node-key 文件）
- [x] Validator 密钥注入 keystore（AURA sr25519 + GRANDPA ed25519）
- [x] systemd service 文件安装（fishbone-main / fishbone-child1 / fishbone-child2）
- [x] 三链全部启动并验证运行（主链 #2062，子链1/2 各 #1995）
- [x] Python 管理框架基础结构（config.py / remote.py / service.py）

---

## 三、待完成

### Step 1：补全 Python 框架细节
- [ ] `cmd/__init__.py` 和 `main.py` 入口
- [ ] `cmd/status.py` 通过 bcg 做 RPC 中转（非直连，因为开发机不在 10.2.2.x 网段）
- [ ] 在 `config.toml` 中记录 bcg 信息
- [ ] 测试 `uv run python3 cmd/status.py`

### Step 2：整理 deploy 目录，清除冗余文件
- [ ] 删除早期的 bash 脚本（scripts/ 下的旧脚本已被 Python 框架取代）
- [ ] 删除 specs/ 下的可读 json（只保留 raw json，大文件）
- [ ] 更新 .gitignore（排除 keys/*.env、specs/*.json 的大文件）

### Step 3：git 提交
- [ ] 提交 deploy/ 目录（排除密钥文件和大 spec 文件）
- [ ] 提交 docs/plan/phase-deploy-multinode.md

### Step 4：验证管理命令可用
- [ ] `uv run python3 cmd/status.py` 显示三链状态
- [ ] `uv run python3 cmd/control.py stop --chains main` 停止主链
- [ ] `uv run python3 cmd/control.py start --chains main` 启动主链
- [ ] `uv run python3 cmd/logs.py main` 实时日志聚合

### Step 5：更新内存文件，记录多节点部署经验

---

## 四、目录结构

```
deploy/
├── config.toml             # 所有节点/链参数（单一真相来源）
├── pyproject.toml          # Python 依赖（uv 管理）
├── fishbone/               # Python 包
│   ├── config.py           # 读 config.toml
│   ├── remote.py           # asyncssh 连接池
│   └── service.py          # systemd service 文件渲染
├── cmd/
│   ├── deploy.py           # 部署到所有节点
│   ├── status.py           # 查看所有节点状态
│   ├── logs.py             # 实时日志聚合
│   └── control.py          # start/stop/restart
├── keys/                   # validator 密钥（不进 git！）
│   └── f{1-6}.env
└── specs/                  # chain spec（raw json，不进 git！）
    ├── main-custom-raw.json
    ├── child1-custom-raw.json
    └── child2-custom-raw.json
```

---

## 五、节点信息

| 机器 | IP | 主链 Peer ID | 子链角色 |
|------|-----|-------------|---------|
| f1 | 10.2.2.11 | 12D3KooWEG8gvUe7RRvrgknZwbzg1snqKQzxhewX5FDaEYGfcDLa | main+child1 |
| f2 | 10.2.2.12 | 12D3KooWFsrUUPRd7oVv4aUQ4oF3oi1zfGYGvhW3TyZvvuWhQqNL | main+child1 |
| f3 | 10.2.2.13 | 12D3KooWAEnd9YjewBvTDXAyc2WDuh89wXoDBLYMuV5RUxoBRDP1 | main+child1 |
| f4 | 10.2.2.14 | 12D3KooWAzKDpDs5jCSX3eC5urRotn4QBsqzhz8pSEqbjsPWYf9k | main+child2 |
| f5 | 10.2.2.15 | 12D3KooWE7erpnv3EzmqhEjkHNB4i2vsDgquJKwG637qPJM9gnur | main+child2 |
| f6 | 10.2.2.16 | 12D3KooWHAASsrtvGcRvMDjLFgyedGkcfp4pphyatVWbpdXp8Het | main+child2 |

---

## 六、执行记录

- [x] 基础环境清理（2026-06-01）
- [x] 三链部署与启动（2026-06-01）
- [ ] Python 管理框架完善（Step 1-4）
