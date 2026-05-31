# BPiano：基于 Pianist 的批量证明压缩方案

**BPiano（Batched Piano）** 是一个 zk-SNARK 协议实现，基于 Pianist 分布式证明框架，将多个 Piano 证明批量压缩为常数大小的聚合证明。

**核心指标：**
- 单个压缩证明验证：4 次配对，与子节点数 M 无关
- 批量 K 个证明的聚合验证：配对次数仍为 4 次，与 K 无关

**基准测试（Keccak-256, T=2¹⁸, M=2）：**
- Piano 证明大小：1824 B
- BPiano 证明大小：800 B（压缩率 0.44×）

---

完整文档见 **[doc/README.md](doc/README.md)**。

原始论文：[doc/paper.md](doc/paper.md) | [doc/pianist.pdf](doc/pianist.pdf)
