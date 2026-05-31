// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./Pairing.sol";

/// @title BPianoVerifier
/// @notice BPiano 压缩证明的链上 Solidity 验证合约。
///
/// 验证流程（对应 bpiano/bpiano/verify.go）：
///   1. 重放 Fiat-Shamir 转录（SHA-256），推导 7 个挑战：γ, η, λ, α, ν, β, μ
///   2. 代数约束检验（纯域运算）
///   3. 推导随机挑战 ρ（链下哈希）
///   4. 构建 4 个 G1 点（用 ecMul + ecAdd）
///   5. 4 配对检验（ecPairing）
///
/// 由于 EVM 缺少 G2 标量乘预编译，以下两个 G2 点须由 Go 侧（solgen.GenerateBPianoCalldata）
/// 链下预计算并作为 calldata 传入：
///   - zTG2      = [Z_T(τ_X)]₂ = G2[2] - (α+ωα)·G2[1] + α·ωα·G2[0]
///   - tauYBetaG2 = [τ_Y - β]₂  = G2Y[1] - β·G2Y[0]
contract BPianoVerifier {
    using Pairing for Pairing.G1Point;

    // ────────────────────────────────────────────────────────────────────────
    // 常量
    // ────────────────────────────────────────────────────────────────────────

    /// @dev BN254 标量域模数 Fr。
    uint256 internal constant FR =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    /// @dev BN254 基域模数 Fp（用于 G1 压缩格式判断）。
    uint256 internal constant FP =
        21888242871839275222246405745257275088696311157297823662689037894645226208583;

    /// @dev (Fp - 1) / 2：LexicographicallyLargest 阈值。
    uint256 internal constant HALF_FP =
        10944121435919637611123202872628637544348155578648911831344518947322613104291;

    /// @dev G1 生成元坐标。
    uint256 internal constant G1X = 1;
    uint256 internal constant G1Y = 2;

    // ────────────────────────────────────────────────────────────────────────
    // 类型定义
    // ────────────────────────────────────────────────────────────────────────

    /// @notice 验证密钥，包含 VK G1/G2 承诺及电路参数。
    struct VerifyingKey {
        // 选择子多项式全局承诺（G1）
        Pairing.G1Point ql;
        Pairing.G1Point qr;
        Pairing.G1Point qm;
        Pairing.G1Point qo;
        Pairing.G1Point qk;
        // 置换多项式全局承诺（G1）
        Pairing.G1Point s1;
        Pairing.G1Point s2;
        Pairing.G1Point s3;
        // DKZG SRS G2 点（X 轴配对用）
        Pairing.G2Point g2_0; // srs.G2[0]
        Pairing.G2Point g2_1; // srs.G2[1]
        // DKZG SRS G2Y 点（Y 轴配对用）
        Pairing.G2Point g2y_0; // srs.G2Y[0]
        // 电路参数（Fr 元素）
        uint256 sizeX;      // T = X 轴域大小
        uint256 sizeY;      // M = 子电路数
        uint256 generatorX; // ω_X：T 次本原单位根
        uint256 cosetShift; // u：陪集偏移
        uint256 nbPublicInputs;
    }

    /// @notice BPiano 压缩证明。
    struct CompressedProof {
        // 12 个 G1 承诺
        Pairing.G1Point lro0; // com_A
        Pairing.G1Point lro1; // com_B
        Pairing.G1Point lro2; // com_O
        Pairing.G1Point z;
        Pairing.G1Point hx0;
        Pairing.G1Point hx1;
        Pairing.G1Point hx2;
        Pairing.G1Point comQX;
        Pairing.G1Point comVFAlpha;
        Pairing.G1Point comVFZS;
        Pairing.G1Point comGY;
        Pairing.G1Point pi1AggH;
        // 15 个标量求值（Fr）
        uint256 evalA;
        uint256 evalB;
        uint256 evalO;
        uint256 evalZ;
        uint256 evalZS;
        uint256 evalHx;
        uint256 evalHy;
        uint256 evalQl;
        uint256 evalQr;
        uint256 evalQm;
        uint256 evalQo;
        uint256 evalQk;
        uint256 evalS1;
        uint256 evalS2;
        uint256 evalS3;
    }

    /// @dev 内部 FS 挑战缓存。
    struct Challenges {
        uint256 gamma;
        uint256 eta;
        uint256 lambda_;  // lambda 是 Solidity 保留字，故用 lambda_
        uint256 alpha;
        uint256 nu;
        uint256 beta;
        uint256 mu;
        uint256 alphaShifted; // ω_X · α mod Fr
    }

    // ────────────────────────────────────────────────────────────────────────
    // 状态
    // ────────────────────────────────────────────────────────────────────────

    VerifyingKey public vk;

    // ────────────────────────────────────────────────────────────────────────
    // 构造函数
    // ────────────────────────────────────────────────────────────────────────

    constructor(VerifyingKey memory _vk) {
        vk = _vk;
    }

    // ────────────────────────────────────────────────────────────────────────
    // 主入口
    // ────────────────────────────────────────────────────────────────────────

    /// @notice 验证 BPiano 压缩证明。
    /// @param proof          压缩证明（12 G1 + 15 Fr）
    /// @param zTG2           [Z_T(τ_X)]₂，由 Go 链下预计算
    /// @param tauYBetaG2     [τ_Y - β]₂，由 Go 链下预计算
    /// @param publicInputsFlat 展平的公开输入（M × nbPublicInputs 个 Fr 元素）
    /// @return true 当且仅当证明合法
    function verify(
        CompressedProof calldata proof,
        Pairing.G2Point calldata zTG2,
        Pairing.G2Point calldata tauYBetaG2,
        uint256[] calldata publicInputsFlat
    ) external view returns (bool) {
        // 步骤 1：重放 Fiat-Shamir 挑战
        Challenges memory ch = _replayFS(proof, publicInputsFlat);

        // 步骤 2：代数约束检验
        require(_algebraicCheck(proof, ch), "BPiano: algebraic constraint failed");

        // 步骤 3：推导 ρ
        uint256 rho = _deriveRho(proof);

        // 步骤 4：构建 4 个 G1 配对点
        (
            Pairing.G1Point memory p0,
            Pairing.G1Point memory p1,
            Pairing.G1Point memory p2,
            Pairing.G1Point memory p3
        ) = _buildPairingPoints(proof, ch, rho);

        // 步骤 5：4 配对检验
        return Pairing.ecPairing4(
            p0, zTG2,
            p1, tauYBetaG2,
            p2, vk.g2_0,
            p3, vk.g2_1
        );
    }

    // ────────────────────────────────────────────────────────────────────────
    // Fiat-Shamir 转录
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 重放与 bpiano.VerifyCompressed 完全相同的 FS 转录。
    ///      格式：SHA256(name_bytes || prev_hash? || bound_G1_or_Fr...)
    ///      G1 点以压缩格式（32 字节）绑定；Fr 元素以大端序（32 字节）绑定。
    function _replayFS(
        CompressedProof calldata p,
        uint256[] calldata piFlat
    ) internal view returns (Challenges memory ch) {
        // ── gamma ────────────────────────────────────────────────────────────
        // 绑定：VK.{Ql,Qr,Qm,Qo,Qk,S1,S2,S3}（压缩 G1）+ 公开输入 + LRO（压缩 G1）
        bytes32 gammaHash;
        {
            bytes memory buf = abi.encodePacked(
                "gamma",
                _g1Compressed(vk.ql.x, vk.ql.y),
                _g1Compressed(vk.qr.x, vk.qr.y),
                _g1Compressed(vk.qm.x, vk.qm.y),
                _g1Compressed(vk.qo.x, vk.qo.y),
                _g1Compressed(vk.qk.x, vk.qk.y),
                _g1Compressed(vk.s1.x, vk.s1.y),
                _g1Compressed(vk.s2.x, vk.s2.y),
                _g1Compressed(vk.s3.x, vk.s3.y)
            );
            // 公开输入（Fr 大端序）
            for (uint256 i = 0; i < piFlat.length; i++) {
                buf = abi.encodePacked(buf, bytes32(piFlat[i]));
            }
            buf = abi.encodePacked(
                buf,
                _g1Compressed(p.lro0.x, p.lro0.y),
                _g1Compressed(p.lro1.x, p.lro1.y),
                _g1Compressed(p.lro2.x, p.lro2.y)
            );
            gammaHash = sha256(buf);
        }
        ch.gamma = uint256(gammaHash) % FR;

        // ── eta ─────────────────────────────────────────────────────────────
        bytes32 etaHash = sha256(abi.encodePacked("eta", gammaHash));
        ch.eta = uint256(etaHash) % FR;

        // ── lambda ───────────────────────────────────────────────────────────
        bytes32 lambdaHash = sha256(abi.encodePacked(
            "lambda", etaHash, _g1Compressed(p.z.x, p.z.y)
        ));
        ch.lambda_ = uint256(lambdaHash) % FR;

        // ── alpha ────────────────────────────────────────────────────────────
        bytes32 alphaHash = sha256(abi.encodePacked(
            "alpha", lambdaHash,
            _g1Compressed(p.hx0.x, p.hx0.y),
            _g1Compressed(p.hx1.x, p.hx1.y),
            _g1Compressed(p.hx2.x, p.hx2.y)
        ));
        ch.alpha = uint256(alphaHash) % FR;

        // ── foldedHxDig（G1 运算，为 nu 绑定所需）────────────────────────────
        Pairing.G1Point memory foldedHx = _computeFoldedHx(p, ch.alpha);

        // ── nu ───────────────────────────────────────────────────────────────
        bytes32 nuHash = sha256(abi.encodePacked(
            "nu", alphaHash,
            _g1Compressed(foldedHx.x, foldedHx.y),
            _g1Compressed(p.lro0.x, p.lro0.y),
            _g1Compressed(p.lro1.x, p.lro1.y),
            _g1Compressed(p.lro2.x, p.lro2.y),
            _g1Compressed(p.z.x, p.z.y)
        ));
        ch.nu = uint256(nuHash) % FR;

        // ── beta ─────────────────────────────────────────────────────────────
        bytes32 betaHash = sha256(abi.encodePacked(
            "beta", nuHash,
            _g1Compressed(p.comQX.x, p.comQX.y),
            _g1Compressed(p.comVFAlpha.x, p.comVFAlpha.y),
            _g1Compressed(p.comVFZS.x, p.comVFZS.y)
        ));
        ch.beta = uint256(betaHash) % FR;

        // ── mu ───────────────────────────────────────────────────────────────
        bytes32 muHash = sha256(abi.encodePacked("mu", betaHash));
        ch.mu = uint256(muHash) % FR;

        // ── alphaShifted = α · ω_X mod Fr ────────────────────────────────────
        ch.alphaShifted = mulmod(ch.alpha, vk.generatorX, FR);
    }

    // ────────────────────────────────────────────────────────────────────────
    // 代数约束
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 检验主代数恒等式：
    ///   gate + λ·(λ·boundary + perm) == (α^T-1)·hx + (β^M-1)·hy
    function _algebraicCheck(
        CompressedProof calldata p,
        Challenges memory ch
    ) internal view returns (bool) {
        uint256 alpha = ch.alpha;
        uint256 beta  = ch.beta;
        uint256 gamma = ch.gamma;
        uint256 eta   = ch.eta;
        uint256 lam   = ch.lambda_;

        uint256 a  = p.evalA;  uint256 b  = p.evalB;  uint256 o  = p.evalO;
        uint256 z  = p.evalZ;  uint256 zs = p.evalZS;
        uint256 hx = p.evalHx; uint256 hy = p.evalHy;
        uint256 ql = p.evalQl; uint256 qr = p.evalQr; uint256 qm = p.evalQm;
        uint256 qo = p.evalQo; uint256 qk = p.evalQk;
        uint256 s1 = p.evalS1; uint256 s2 = p.evalS2; uint256 s3 = p.evalS3;

        // gate = ql·a + qr·b + qm·a·b + qo·o + qk
        uint256 gate;
        {
            uint256 t;
            gate = mulmod(ql, a, FR);
            t    = mulmod(qr, b, FR); gate = addmod(gate, t, FR);
            t    = mulmod(mulmod(qm, a, FR), b, FR); gate = addmod(gate, t, FR);
            t    = mulmod(qo, o, FR); gate = addmod(gate, t, FR);
            gate = addmod(gate, qk, FR);
        }

        // L0(α) = (α^T - 1) / (T · (α - 1))
        uint256 alphaPowT = _modexp(alpha, vk.sizeX, FR);
        uint256 vanX      = addmod(alphaPowT, FR - 1, FR); // α^T - 1
        uint256 l0;
        {
            // denom = T · (α - 1)
            uint256 denom = mulmod(
                vk.sizeX % FR,
                addmod(alpha, FR - 1, FR),
                FR
            );
            l0 = mulmod(vanX, _modInv(denom), FR);
        }

        // boundary = L0 · (z - 1)
        uint256 boundary = mulmod(l0, addmod(z, FR - 1, FR), FR);

        // 置换项
        uint256 u    = vk.cosetShift;
        uint256 uSq  = mulmod(u, u, FR);
        uint256 idA  = alpha;
        uint256 idB  = mulmod(u, alpha, FR);
        uint256 idC  = mulmod(uSq, alpha, FR);

        uint256 F;
        {
            uint256 f0 = addmod(addmod(a, mulmod(eta, idA, FR), FR), gamma, FR);
            uint256 f1 = addmod(addmod(b, mulmod(eta, idB, FR), FR), gamma, FR);
            uint256 f2 = addmod(addmod(o, mulmod(eta, idC, FR), FR), gamma, FR);
            F = mulmod(mulmod(mulmod(f0, f1, FR), f2, FR), z, FR);
        }
        uint256 G;
        {
            uint256 g0 = addmod(addmod(a, mulmod(eta, s1, FR), FR), gamma, FR);
            uint256 g1 = addmod(addmod(b, mulmod(eta, s2, FR), FR), gamma, FR);
            uint256 g2 = addmod(addmod(o, mulmod(eta, s3, FR), FR), gamma, FR);
            G = mulmod(mulmod(mulmod(g0, g1, FR), g2, FR), zs, FR);
        }
        uint256 perm = addmod(G, FR - F, FR); // G - F

        // lhs = gate + λ·(λ·boundary + perm)
        uint256 inner = addmod(mulmod(lam, boundary, FR), perm, FR);
        uint256 lhs   = addmod(gate, mulmod(lam, inner, FR), FR);

        // rhs = (α^T-1)·hx + (β^M-1)·hy
        uint256 betaPowM = _modexp(beta, vk.sizeY, FR);
        uint256 vanY     = addmod(betaPowM, FR - 1, FR); // β^M - 1
        uint256 rhs      = addmod(mulmod(vanX, hx, FR), mulmod(vanY, hy, FR), FR);

        return lhs == rhs;
    }

    // ────────────────────────────────────────────────────────────────────────
    // ρ 推导
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 链下 SHA-256 哈希推导 ρ（对应 bpiano.deriveRhoBP）。
    ///      哈希输入：压缩格式 ComQX || ComGY || Pi1AggH || 7 个 Fr 求值
    function _deriveRho(CompressedProof calldata p)
        internal
        pure
        returns (uint256)
    {
        bytes32 h = sha256(abi.encodePacked(
            _g1Compressed(p.comQX.x,    p.comQX.y),
            _g1Compressed(p.comGY.x,    p.comGY.y),
            _g1Compressed(p.pi1AggH.x,  p.pi1AggH.y),
            bytes32(p.evalHx),
            bytes32(p.evalA),
            bytes32(p.evalB),
            bytes32(p.evalO),
            bytes32(p.evalZ),
            bytes32(p.evalZS),
            bytes32(p.evalHy)
        ));
        return uint256(h) % FR;
    }

    // ────────────────────────────────────────────────────────────────────────
    // G1 线性组合（构建 4 配对点）
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 计算 4 配对点 P0, P1, P2, P3（对应 bpiano/bpiano/verify.go 步骤 5-9）。
    ///
    ///   P0 = ComQX                              （与 ZTG2 配对）
    ///   P1 = ρ·Pi1AggH                          （与 TauYBetaG2 配对）
    ///   P2 = ωα·C1 + α·C2 - ρ·(ComGY-gYβ·g1)  （与 G2[0] 配对）
    ///   P3 = -(C1 + C2)                         （与 G2[1] 配对）
    ///
    /// 化简后（消去等价 g1 项）：
    ///   C1 = Σ_{k=0..12} ν^k·com_k - Σ_{k=5..12} ν^k·eval_k·g1 - ComVFAlpha
    ///   C2 = ν^13·(Z - ComVFZS)
    function _buildPairingPoints(
        CompressedProof calldata p,
        Challenges memory ch,
        uint256 rho
    )
        internal
        view
        returns (
            Pairing.G1Point memory p0,
            Pairing.G1Point memory p1,
            Pairing.G1Point memory p2,
            Pairing.G1Point memory p3
        )
    {
        // P0 = ComQX
        p0 = p.comQX;

        // P1 = ρ·Pi1AggH
        p1 = Pairing.ecMul(p.pi1AggH, rho);

        // ── ν 幂次 ν^0..ν^13 ─────────────────────────────────────────────────
        uint256[14] memory nuPow;
        nuPow[0] = 1;
        for (uint256 i = 1; i < 14; i++) {
            nuPow[i] = mulmod(nuPow[i - 1], ch.nu, FR);
        }

        // ── foldedHxDig（用于 C1 的第 0 项承诺）────────────────────────────────
        Pairing.G1Point memory foldedHx = _computeFoldedHx(p, ch.alpha);

        // ── sAlphaComs：13 个承诺 ─────────────────────────────────────────────
        // [foldedHx, lro0, lro1, lro2, z, ql, qr, qm, qo, qk, s1, s2, s3]
        // ── sAlphaEvals：13 个标量 ────────────────────────────────────────────
        // [evalHx, evalA, evalB, evalO, evalZ, evalQl, evalQr, evalQm, evalQo, evalQk, evalS1, evalS2, evalS3]

        // C1 = Σ_{k=0..12} ν^k·com_k  （先求和承诺项）
        Pairing.G1Point memory c1 = _buildC1ComSum(p, foldedHx, nuPow);

        // 减去 Σ_{k=5..12} ν^k·eval_k·g1（仅共享多项式项）
        uint256 sharedEvalSum;
        {
            uint256[8] memory sharedEvals = [
                p.evalQl, p.evalQr, p.evalQm, p.evalQo, p.evalQk,
                p.evalS1, p.evalS2, p.evalS3
            ];
            for (uint256 k = 0; k < 8; k++) {
                sharedEvalSum = addmod(
                    sharedEvalSum,
                    mulmod(nuPow[k + 5], sharedEvals[k], FR),
                    FR
                );
            }
        }
        // c1 -= sharedEvalSum · g1
        c1 = Pairing.ecAdd(c1, Pairing.ecNeg(Pairing.ecMul(
            Pairing.G1Point(G1X, G1Y), sharedEvalSum
        )));
        // c1 -= ComVFAlpha
        c1 = Pairing.ecAdd(c1, Pairing.ecNeg(p.comVFAlpha));

        // C2 = ν^13 · (Z - ComVFZS)
        Pairing.G1Point memory c2 = Pairing.ecAdd(
            Pairing.ecMul(p.z,       nuPow[13]),
            Pairing.ecNeg(Pairing.ecMul(p.comVFZS, nuPow[13]))
        );

        // ── G_Y(β) = Σ_k μ^k · yEvals[k]  (k=0..6) ─────────────────────────
        // yEvals = [evalHx, evalA, evalB, evalO, evalZ, evalZS, evalHy]
        uint256 gYBeta;
        {
            uint256 muK = 1;
            uint256[7] memory yEvals = [
                p.evalHx, p.evalA, p.evalB, p.evalO, p.evalZ, p.evalZS, p.evalHy
            ];
            for (uint256 k = 0; k < 7; k++) {
                gYBeta = addmod(gYBeta, mulmod(muK, yEvals[k], FR), FR);
                muK = mulmod(muK, ch.mu, FR);
            }
        }

        // ── P2 = ωα·C1 + α·C2 - ρ·(ComGY - gYBeta·g1) ──────────────────────
        {
            Pairing.G1Point memory c1s = Pairing.ecMul(c1, ch.alphaShifted);
            Pairing.G1Point memory c2a = Pairing.ecMul(c2, ch.alpha);
            // ComGY - gYBeta·g1
            Pairing.G1Point memory gYG1 = Pairing.ecMul(
                Pairing.G1Point(G1X, G1Y), gYBeta
            );
            Pairing.G1Point memory comGYDiff = Pairing.ecAdd(
                p.comGY, Pairing.ecNeg(gYG1)
            );
            // ρ·(ComGY - gYBeta·g1)
            Pairing.G1Point memory rhoDiff = Pairing.ecMul(comGYDiff, rho);
            p2 = Pairing.ecAdd(Pairing.ecAdd(c1s, c2a), Pairing.ecNeg(rhoDiff));
        }

        // ── P3 = -(C1 + C2) ──────────────────────────────────────────────────
        p3 = Pairing.ecNeg(Pairing.ecAdd(c1, c2));
    }

    // ────────────────────────────────────────────────────────────────────────
    // 内部辅助
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 计算 foldedHxDig = Hx[0] + α^T·Hx[1] + α^{2T}·Hx[2]（G1 线性组合）。
    function _computeFoldedHx(CompressedProof calldata p, uint256 alpha)
        internal
        view
        returns (Pairing.G1Point memory)
    {
        uint256 alphaPowT  = _modexp(alpha, vk.sizeX, FR);
        uint256 alphaPow2T = mulmod(alphaPowT, alphaPowT, FR);
        return Pairing.ecAdd(
            p.hx0,
            Pairing.ecAdd(
                Pairing.ecMul(p.hx1, alphaPowT),
                Pairing.ecMul(p.hx2, alphaPow2T)
            )
        );
    }

    /// @dev 计算 C1 的承诺部分：Σ_{k=0..12} ν^k · com_k。
    function _buildC1ComSum(
        CompressedProof calldata p,
        Pairing.G1Point memory foldedHx,
        uint256[14] memory nuPow
    ) internal view returns (Pairing.G1Point memory c1) {
        // k=0: foldedHx
        c1 = Pairing.ecMul(foldedHx, nuPow[0]);
        // k=1..3: lro0, lro1, lro2
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.lro0, nuPow[1]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.lro1, nuPow[2]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.lro2, nuPow[3]));
        // k=4: z
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.z, nuPow[4]));
        // k=5..9: vk 选择子承诺
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.ql, nuPow[5]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qr, nuPow[6]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qm, nuPow[7]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qo, nuPow[8]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qk, nuPow[9]));
        // k=10..12: vk 置换承诺
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.s1, nuPow[10]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.s2, nuPow[11]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.s3, nuPow[12]));
    }

    /// @dev G1 点压缩为 32 字节（gnark-crypto Bytes() 格式）：
    ///      X 大端序 + MSB 置位（0x80：Y ≤ (Fp-1)/2；0xC0：Y > (Fp-1)/2）
    function _g1Compressed(uint256 x, uint256 y)
        internal
        pure
        returns (bytes32)
    {
        uint256 flag = (y > HALF_FP) ? 0xC0 : 0x80;
        return bytes32(x | (flag << 248));
    }

    /// @dev 模幂：base^exponent mod modulus，使用预编译 0x05（BigModExp）。
    function _modexp(uint256 base, uint256 exponent, uint256 modulus)
        internal
        view
        returns (uint256 result)
    {
        assembly ("memory-safe") {
            let ptr := mload(0x40)
            mstore(ptr,            0x20) // base 长度（32 字节）
            mstore(add(ptr, 0x20), 0x20) // exponent 长度
            mstore(add(ptr, 0x40), 0x20) // modulus 长度
            mstore(add(ptr, 0x60), base)
            mstore(add(ptr, 0x80), exponent)
            mstore(add(ptr, 0xa0), modulus)
            let ok := staticcall(sub(gas(), 2000), 0x05, ptr, 0xc0, ptr, 0x20)
            if iszero(ok) { revert(0, 0) }
            result := mload(ptr)
        }
    }

    /// @dev 模逆：a^{Fr-2} mod Fr（Fermat 小定理）。
    function _modInv(uint256 a) internal view returns (uint256) {
        return _modexp(a, FR - 2, FR);
    }
}
