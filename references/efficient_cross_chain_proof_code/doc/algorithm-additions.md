# 算法补充草稿

本文件包含三个算法块的内容及其在论文中的插入位置说明。
算法格式参照 LaTeX `algorithm2e` 或 `algorithmicx`，可直接转换。

---

## 算法1：BPiano.Compress

**插入位置：§4.2.3 末尾**，在"经过上述X轴多点聚合……"段落之前。

在 §4.2.3 末尾、§4.3 开始之前，加一句引导语：

> 以上三阶段的完整证明生成流程总结为算法1。

---

**算法 1：BPiano.Compress（压缩证明生成）**

**输入：**
- 公共参数 $pp = (g,\, g^{\tau_X},\, g^{\tau_Y},\, \mathbf{U},\, \mathbf{V})$，验证密钥 $vk$，公共多项式承诺 $com_{S_{pp}}$
- 主节点证明密钥 $pk_0$，子节点证明密钥 $\{pk_i\}_{i=1}^{M-1}$
- 各子节点本地见证向量 $\{\mathbf{w}_i\}_{i=0}^{M-1}$

**输出：** 压缩证明 $\pi_c$

---

**// ── 承诺阶段 ──**

**1.**  $\quad$ **[子节点并行]** 由 $\mathbf{w}_i$ 插值得到本地见证多项式 $a_i(X),b_i(X),o_i(X)$；
$\quad\quad$ 计算本地承诺 $com_{a_i},com_{b_i},com_{o_i}$，发送至主节点 $\mathcal{P}_0$

**2.**  $\quad$ **[主节点]** 聚合：$com_A \leftarrow \prod_i com_{a_i}$，$com_B,com_O$ 同理
$\quad\quad$ 派生挑战：$\eta \leftarrow \mathcal{H}(pp, com_{S_{pp}}, com_A, com_B, com_O)$
$\quad\quad\quad\quad\quad\quad\quad\quad$ $\gamma \leftarrow \mathcal{H}(pp, com_{S_{pp}}, com_A, com_B, com_O, \eta)$
$\quad\quad$ 将 $\eta,\gamma$ 分发至各子节点

**3.**  $\quad$ **[子节点并行]** 计算累加多项式 $z_i(X)$，生成 $com_{z_i}$，发送至 $\mathcal{P}_0$

**4.**  $\quad$ **[主节点]** 聚合：$com_Z \leftarrow \prod_i com_{z_i}$
$\quad\quad$ 派生挑战：$\lambda \leftarrow \mathcal{H}(pp, com_A, com_B, com_O, com_Z, \eta, \gamma)$，分发

**5.**  $\quad$ **[子节点并行]** 计算 X 轴商多项式 $h_i(X)$，生成 $com_{h_i}$，发送至 $\mathcal{P}_0$

**6.**  $\quad$ **[主节点]** 聚合：$com_{H_X} \leftarrow \prod_i com_{h_i}$

---

**// ── X 轴多点聚合阶段 ──**

**7.**  $\quad$ **[主节点]** 派生挑战：
$\quad\quad$ $\alpha \leftarrow \mathcal{H}(pp, com_{S_{pp}}, com_A, com_B, com_O, com_Z, com_{H_X}, \eta, \gamma, \lambda)$
$\quad\quad$ $\nu \leftarrow \mathcal{H}(\alpha)$
$\quad\quad$ 将 $\alpha,\nu$ 分发至各子节点

**8.**  $\quad$ **[子节点并行]** 对 $S \in \mathcal{S}_\alpha = \{a,b,o,z,h\}$ 计算 $S_i(\alpha)$；计算 $z_i(\omega_X\alpha)$
$\quad\quad$ 计算本地 X 轴聚合商多项式分片：
$$Q_{X,i}(X) \leftarrow \sum_{S \in \mathcal{S}_\alpha} \nu^{\mathrm{idx}(S)} \frac{S_i(X)-S_i(\alpha)}{X-\alpha} \;+\; \nu^{\mathrm{idx}(Z')} \frac{z_i(X)-z_i(\omega_X\alpha)}{X-\omega_X\alpha}$$
$\quad\quad$ 生成 $com_{Q_{X,i}} \leftarrow \mathrm{CommitLocal}(i,\, Q_{X,i})$，发送至 $\mathcal{P}_0$

**9.**  $\quad$ **[主节点]** 聚合：$com_{Q_X} \leftarrow \prod_i com_{Q_{X,i}}$
$\quad\quad$ 重建：$v_S(Y) \leftarrow \sum_i R_i(Y)\cdot S_i(\alpha)$ 对 $S \in \mathcal{S}_\alpha$；$v_{Z}'(Y) \leftarrow \sum_i R_i(Y)\cdot z_i(\omega_X\alpha)$
$\quad\quad$ 由主约束方程在 $X=\alpha$ 处求解 Y 轴商多项式：
$$H_Y(Y,\alpha) \leftarrow \frac{G(Y,\alpha) + \lambda P_0(Y,\alpha) + \lambda^2 P_1(Y,\alpha) - V_X(\alpha)\cdot v_{H_X}(Y)}{V_Y(Y)}$$

---

**// ── Y 轴聚合阶段 ──**

**10.** $\quad$ **[主节点]** 派生挑战：
$\quad\quad$ $\beta \leftarrow \mathcal{H}\!\left(\alpha,\; com_{Q_X},\; \{v_S(Y)\}_{S\in\mathcal{S}_\alpha},\; v_{Z}'(Y)\right)$
$\quad\quad$ $\mu \leftarrow \mathcal{H}(\beta)$

**11.** $\quad$ **[主节点]** 令 $\mathcal{V}_Y \leftarrow \{v_S(Y)\}_{S\in\mathcal{S}_\alpha} \cup \{v_{Z}'(Y)\} \cup \{H_Y(Y,\alpha)\}$
$\quad\quad$ 构造折叠多项式：$G_Y(Y) \leftarrow \sum_{P \in \mathcal{V}_Y} \mu^{\mathrm{idx}(P)} \cdot P(Y)$
$\quad\quad$ 计算 Y 轴商多项式：$Q_Y(Y) \leftarrow \dfrac{G_Y(Y) - G_Y(\beta)}{Y - \beta}$
$\quad\quad$ 生成：$\pi_{1,\mathrm{agg}} \leftarrow g^{Q_Y(\tau_Y)}$，$com_{G_Y} \leftarrow g^{G_Y(\tau_Y)}$

---

**// ── 组装压缩证明 ──**

**12.** $\quad$ 计算最终求值：对 $S \in \mathcal{S}_\alpha$，$S(\beta,\alpha) \leftarrow v_S(\beta)$；$Z(\beta,\omega_X\alpha) \leftarrow v_{Z}'(\beta)$；$H_Y(\beta,\alpha)$

**13.** $\quad$ **返回**：
$$\pi_c \;\leftarrow\; \Bigl(\underbrace{com_A,\, com_B,\, com_O,\, com_Z,\, com_{H_X},\, com_{Q_X},\, com_{G_Y}}_{\text{承诺部分}},\;\underbrace{\pi_{1,\mathrm{agg}}}_{\text{开启证明}},\;\underbrace{\{S(\beta,\alpha)\}_{S\in\mathcal{S}_\alpha},\; Z(\beta,\omega_X\alpha),\; H_Y(\beta,\alpha)}_{\text{求值部分}}\Bigr)$$

---

## 算法2：BPiano.Aggregate

**插入位置：§4.3.2 末尾**，在"经过上述挑战协调和承诺聚合后……"段落之前。

加引导语：

> $K$ 个压缩证明的协调与聚合完整流程如算法2所示。

---

**算法 2：BPiano.Aggregate（批量证明聚合）**

**输入：**
- $K$ 套证明者数据 $\bigl\{(C^{(k)},\, \{\mathbf{w}_i^{(k)}\},\, pp,\, \{pk_i^{(k)}\})\bigr\}_{k=0}^{K-1}$
- 协调主节点 $\mathcal{P}_0^{(0)}$

**输出：** 批量证明 $\pi_{\mathrm{batch}}$

---

**// ── 各证明者独立执行承诺阶段 ──**

**1.**  $\quad$ **[对 $k=0,\ldots,K{-}1$ 并行]** 执行算法1步骤 1–6
$\quad\quad$ 得到：$com_A^{(k)},\, com_B^{(k)},\, com_O^{(k)},\, com_Z^{(k)},\, com_{H_X}^{(k)}$，以及 $\eta^{(k)},\gamma^{(k)},\lambda^{(k)}$

---

**// ── X 轴挑战协调 ──**

**2.**  $\quad$ **[$\mathcal{P}_0^{(0)}$]** 收集 $\{com_{H_X}^{(k)}\}_{k=0}^{K-1}$，派生共享挑战：
$$\alpha \leftarrow \mathcal{H}\!\left(pp,\; com_{H_X}^{(0)},\; com_{H_X}^{(1)},\; \ldots,\; com_{H_X}^{(K-1)}\right)$$
$\quad\quad$ 对 $k=0,\ldots,K{-}1$ 派生各证明的聚合随机数 $\nu^{(k)} \leftarrow \mathcal{H}(\alpha, k)$；将 $\alpha,\{\nu^{(k)}\}$ 分发

**3.**  $\quad$ **[对 $k=0,\ldots,K{-}1$ 并行]** 使用共享 $\alpha$ 和各自的 $\nu^{(k)}$ 执行算法1步骤 8–9
$\quad\quad$ 得到：$com_{Q_X}^{(k)}$，$\{v_S^{(k)}(Y)\}_{S\in\mathcal{S}_\alpha}$，$v_Z^{\prime(k)}(Y)$，$H_Y^{(k)}(Y,\alpha)$；发送至 $\mathcal{P}_0^{(0)}$

---

**// ── Y 轴挑战协调 ──**

**4.**  $\quad$ **[$\mathcal{P}_0^{(0)}$]** 收集全部 X 轴结果，派生共享挑战：
$$\beta \leftarrow \mathcal{H}\!\left(\alpha,\; com_{Q_X}^{(0)},\; \mathcal{V}_X^{(0)},\; \ldots,\; com_{Q_X}^{(K-1)},\; \mathcal{V}_X^{(K-1)}\right)$$
$\quad\quad$ 其中 $\mathcal{V}_X^{(k)} = \{v_S^{(k)}(Y)\}_{S\in\mathcal{S}_\alpha} \cup \{v_Z^{\prime(k)}(Y)\}$
$\quad\quad$ 对 $k=0,\ldots,K{-}1$ 派生 $\mu^{(k)} \leftarrow \mathcal{H}(\beta, k)$；将 $\beta,\{\mu^{(k)}\}$ 分发

**5.**  $\quad$ **[对 $k=0,\ldots,K{-}1$ 并行]** 使用共享 $\beta$ 和各自的 $\mu^{(k)}$ 执行算法1步骤 11–13
$\quad\quad$ 得到各自完整压缩证明 $\pi_c^{(k)}$；发送至 $\mathcal{P}_0^{(0)}$

---

**// ── 承诺聚合 ──**

**6.**  $\quad$ **[$\mathcal{P}_0^{(0)}$]** 收集 $\{\pi_c^{(k)}\}$，派生聚合系数：
$\quad\quad$ $r_k \leftarrow \mathcal{H}(\pi_c^{(0)}, \pi_c^{(1)}, \ldots, \pi_c^{(K-1)},\; k)$，$k=0,\ldots,K{-}1$

**7.**  $\quad$ **[$\mathcal{P}_0^{(0)}$]** 计算聚合 X 轴量：
$\quad\quad$ $com_{Q_X,\mathrm{total}} \leftarrow \sum_{k=0}^{K-1} r_k \cdot com_{Q_X}^{(k)}$
$\quad\quad$ $C_{1,\mathrm{total}} \leftarrow \sum_{k} r_k \cdot C_1^{(k)}$，$C_{2,\mathrm{total}} \leftarrow \sum_{k} r_k \cdot C_2^{(k)}$
$\quad\quad$ 计算聚合 Y 轴量：
$\quad\quad$ $\pi_{1,\mathrm{total}} \leftarrow \sum_{k=0}^{K-1} r_k \cdot \pi_{1,\mathrm{agg}}^{(k)}$
$\quad\quad$ $D_{Y,\mathrm{total}} \leftarrow \sum_{k} r_k \cdot \bigl(com_{G_Y}^{(k)} - [G_Y^{(k)}(\beta)]_1\bigr)$

**8.**  $\quad$ **返回**：
$\quad\quad$ $\pi_{\mathrm{batch}} \leftarrow \Bigl(\{\pi_c^{(k)}\}_{k=0}^{K-1},\quad com_{Q_X,\mathrm{total}},\quad \pi_{1,\mathrm{total}}\Bigr)$

---

## 算法3：BPiano.BatchVerify

**插入位置：§4.3.3 开头**，替换原有的四个编号段落（步骤(1)-(4)），改为一句引导语 + 算法块。

建议修改 §4.3.3 的开头段落为：

> 验证者持有批量证明 $\pi_{\mathrm{batch}}$、公共参数 $pp$ 及验证密钥 $vk$，执行算法3完成批量验证。

---

**算法 3：BPiano.BatchVerify（批量验证）**

**输入：**
- 批量证明 $\pi_{\mathrm{batch}} = \bigl(\{\pi_c^{(k)}\}_{k=0}^{K-1},\; com_{Q_X,\mathrm{total}},\; \pi_{1,\mathrm{total}}\bigr)$
- 公共参数 $pp$，验证密钥 $vk = (g^{\tau_X}, g^{\tau_Y}, com_{S_{pp}})$

**输出：** $\mathsf{accept}$ 或 $\mathsf{reject}$

---

**// ── 步骤1：Fiat-Shamir 挑战重算 ──**

**1.**  $\quad$ 从 $\{com_{H_X}^{(k)}\}$ 重算共享 $\alpha$；从 X 轴聚合结果重算共享 $\beta$
$\quad\quad$ 对各 $k$ 重算 $\eta^{(k)},\gamma^{(k)},\lambda^{(k)},\nu^{(k)},\mu^{(k)}$；重算聚合系数 $\{r_k\}$

---

**// ── 步骤2：约束满足性检查（$O(K)$ 次域运算）──**

**2.**  $\quad$ **对 $k=0,\ldots,K{-}1$**，验证：
$$G^{(k)}(\beta,\alpha) + \lambda^{(k)} P_0^{(k)}(\beta,\alpha) + \lambda^{(k)2} P_1^{(k)}(\beta,\alpha) \;\stackrel{?}{=}\; V_X(\alpha)\,H_X^{(k)}(\beta,\alpha) + V_Y(\beta)\,H_Y^{(k)}(\beta,\alpha)$$
$\quad\quad$ 若任一 $k$ 不成立，返回 $\mathsf{reject}$

---

**// ── 步骤3：聚合一致性验证（$O(K)$ 次 $\mathbb{G}_1$ 标量乘法）──**

**3.**  $\quad$ 验证协调主节点提交的预聚合值：
$\quad\quad$ $com_{Q_X,\mathrm{total}} \;\stackrel{?}{=}\; \sum_k r_k \cdot com_{Q_X}^{(k)}$
$\quad\quad$ $\pi_{1,\mathrm{total}} \;\stackrel{?}{=}\; \sum_k r_k \cdot \pi_{1,\mathrm{agg}}^{(k)}$
$\quad\quad$ 链上重计算：
$\quad\quad$ $C_{1,\mathrm{total}} \leftarrow \sum_k r_k \cdot C_1^{(k)}$，其中 $C_1^{(k)} = \sum_{S\in\mathcal{S}_\alpha} \nu^{(k)\,\mathrm{idx}(S)} \bigl(com_S^{(k)} - [v_S^{(k)}(\beta)]_1\bigr)$
$\quad\quad$ $C_{2,\mathrm{total}} \leftarrow \sum_k r_k \cdot C_2^{(k)}$，其中 $C_2^{(k)} = \nu^{(k)\,\mathrm{idx}(Z')} \bigl(com_Z^{(k)} - [v_Z^{\prime(k)}(\beta)]_1\bigr)$
$\quad\quad$ $D_{Y,\mathrm{total}} \leftarrow \sum_k r_k \cdot \bigl(com_{G_Y}^{(k)} - [G_Y^{(k)}(\beta)]_1\bigr)$
$\quad\quad$ 若不一致，返回 $\mathsf{reject}$

---

**// ── 步骤4：配对检查（常数 4 次配对）──**

**4.**  $\quad$ 派生随机数 $\rho \leftarrow \mathcal{H}(\pi_{\mathrm{batch}})$
$\quad\quad$ 计算线性化项：
$\quad\quad$ $D_{\mathrm{lin}} \leftarrow \omega_X\alpha \cdot C_{1,\mathrm{total}} + \alpha \cdot C_{2,\mathrm{total}} + \rho \cdot D_{Y,\mathrm{total}}$
$\quad\quad$ $D_\tau \leftarrow -(C_{1,\mathrm{total}} + C_{2,\mathrm{total}}) + \rho \cdot \pi_{1,\mathrm{total}}$
$\quad\quad$ 执行配对等式验证（其中 $[Z_T(\tau_X)]_2 = g_2^{(\tau_X-\alpha)(\tau_X-\omega_X\alpha)}$ 由证明者链下预计算后以 calldata 传入）：
$$e\!\bigl(com_{Q_X,\mathrm{total}},\; [Z_T(\tau_X)]_2\bigr) \cdot e\!\bigl(\pi_{1,\mathrm{total}},\; g_2^{\tau_Y-\beta}\bigr)^\rho \;\stackrel{?}{=}\; e\!\bigl(D_{\mathrm{lin}},\; g_2\bigr) \cdot e\!\bigl(D_\tau,\; g_2^{\tau_X}\bigr)$$
$\quad\quad$ 若成立，返回 $\mathsf{accept}$；否则返回 $\mathsf{reject}$
