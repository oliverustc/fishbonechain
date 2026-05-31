// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./Pairing.sol";

/// @title AggBPianoVerifier
/// @notice 聚合 BPiano 证明链上验证合约，对应 Go bpiano.VerifyBatch。
///
/// 验证流程（常数 4 次 ecPairing）：
///   1. 重推共享 α（SHA256 coord-alpha）和 β（coord-beta）
///   2. 对每个 k：FS 推导 γ_k/η_k/λ_k → 代数约束检验
///   3. 推导 ν_k（coord-nu）、μ（coord-mu，与 k 无关）
///   4. 推导聚合系数 r_k（agg-rk）
///   5. 验证 ComQXTotal/Pi1Total 一致性（Σ r_k·com^{(k)} 与传入值相等）
///   6. 构建 C1_total, C2_total, D_{Y,total}
///   7. 推导聚合 ρ（agg-rho），组装 4 个配对点
///   8. ecPairing4 检验
///
/// EVM 缺少 G2 标量乘预编译；ZTG2 和 TauYBetaG2 须由 Go 侧链下预计算后传入。
contract AggBPianoVerifier {

    // ────────────────────────────────────────────────────────────────────────
    // 常量
    // ────────────────────────────────────────────────────────────────────────

    uint256 internal constant FR =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;
    uint256 internal constant FP =
        21888242871839275222246405745257275088696311157297823662689037894645226208583;
    uint256 internal constant HALF_FP =
        10944121435919637611123202872628637544348155578648911831344518947322613104291;
    uint256 internal constant G1X = 1;
    uint256 internal constant G1Y = 2;

    // ────────────────────────────────────────────────────────────────────────
    // 类型定义
    // ────────────────────────────────────────────────────────────────────────

    struct VerifyingKey {
        Pairing.G1Point ql;
        Pairing.G1Point qr;
        Pairing.G1Point qm;
        Pairing.G1Point qo;
        Pairing.G1Point qk;
        Pairing.G1Point s1;
        Pairing.G1Point s2;
        Pairing.G1Point s3;
        Pairing.G2Point g2_0;
        Pairing.G2Point g2_1;
        Pairing.G2Point g2y_0;
        uint256 sizeX;
        uint256 sizeY;
        uint256 generatorX;
        uint256 cosetShift;
        uint256 nbPublicInputs;
    }

    struct CompressedProof {
        Pairing.G1Point lro0;
        Pairing.G1Point lro1;
        Pairing.G1Point lro2;
        Pairing.G1Point z;
        Pairing.G1Point hx0;
        Pairing.G1Point hx1;
        Pairing.G1Point hx2;
        Pairing.G1Point comQX;
        Pairing.G1Point comVFAlpha;
        Pairing.G1Point comVFZS;
        Pairing.G1Point comGY;
        Pairing.G1Point pi1AggH;
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

    // ────────────────────────────────────────────────────────────────────────
    // 状态
    // ────────────────────────────────────────────────────────────────────────

    VerifyingKey public vk;

    constructor(VerifyingKey memory _vk) {
        vk = _vk;
    }

    // ────────────────────────────────────────────────────────────────────────
    // 主入口
    // ────────────────────────────────────────────────────────────────────────

    /// @notice 验证 K 个共享 α/β 的聚合 BPiano 压缩证明。
    /// @param proofs         K 个压缩证明（共享 α/β，由 CoordinateChallenges 生成）
    /// @param comQXTotal     Σ r_k·ComQX^{(k)}（由 AggregateProofs 计算）
    /// @param pi1Total       Σ r_k·Pi1AggH^{(k)}
    /// @param zTG2           [Z_T(τ_X)]₂，Go 链下预计算（依赖共享 α）
    /// @param tauYBetaG2     [τ_Y - β]₂，Go 链下预计算（依赖共享 β）
    /// @param publicInputsFlat 所有证明共享的公开输入（无 PI 时为空）
    function verify(
        CompressedProof[] calldata proofs,
        Pairing.G1Point calldata comQXTotal,
        Pairing.G1Point calldata pi1Total,
        Pairing.G2Point calldata zTG2,
        Pairing.G2Point calldata tauYBetaG2,
        uint256[] calldata publicInputsFlat
    ) external view returns (bool) {
        uint256 K = proofs.length;
        if (K == 0) return false;

        // ── 步骤 1：重推共享 α / β ──────────────────────────────────────────
        uint256 sharedAlpha = _deriveSharedAlpha(proofs);
        uint256 sharedBeta  = _deriveSharedBeta(sharedAlpha, proofs);
        uint256 alphaPowT   = _modexp(sharedAlpha, vk.sizeX, FR);
        uint256 alphaShifted = mulmod(sharedAlpha, vk.generatorX, FR);

        // ── 步骤 2：推导 μ（与 k 无关）──────────────────────────────────────
        uint256 mu = _deriveMuCoord(sharedBeta);

        // ── 步骤 3：推导聚合系数 r_k ─────────────────────────────────────────
        uint256[] memory rk = _deriveAggCoeffs(proofs);

        // ── 步骤 4：逐证明 loop ──────────────────────────────────────────────
        Pairing.G1Point memory c1Total;
        Pairing.G1Point memory c2Total;
        Pairing.G1Point memory dYTotal;
        Pairing.G1Point memory comQXExpected;
        Pairing.G1Point memory pi1Expected;

        for (uint256 k = 0; k < K; k++) {
            CompressedProof calldata p = proofs[k];

            // per-proof FS 挑战
            (uint256 gamma_k, uint256 eta_k, uint256 lambda_k) =
                _derivePerProofFS(p, publicInputsFlat);

            // 代数约束检验（使用共享 α/β）
            if (!_algebraicCheck(p, sharedAlpha, sharedBeta,
                                  gamma_k, eta_k, lambda_k)) return false;

            // foldedHx_k = Hx[0] + α^T·Hx[1] + α^{2T}·Hx[2]
            Pairing.G1Point memory foldedHx = _computeFoldedHx(p, alphaPowT);

            // ν_k（coord-nu）
            uint256 nu_k = _deriveNuCoord(sharedAlpha, foldedHx, p);

            // C1_k, C2_k
            (Pairing.G1Point memory c1_k, Pairing.G1Point memory c2_k) =
                _buildC1C2(p, foldedHx, nu_k);

            // D_{Y,k} = ComGY_k - G_Y(β)·g1
            Pairing.G1Point memory dY_k = _buildDY(p, mu);

            // r_k 加权累加
            uint256 r = rk[k];
            c1Total        = Pairing.ecAdd(c1Total,        Pairing.ecMul(c1_k,       r));
            c2Total        = Pairing.ecAdd(c2Total,        Pairing.ecMul(c2_k,       r));
            dYTotal        = Pairing.ecAdd(dYTotal,        Pairing.ecMul(dY_k,       r));
            comQXExpected  = Pairing.ecAdd(comQXExpected,  Pairing.ecMul(p.comQX,    r));
            pi1Expected    = Pairing.ecAdd(pi1Expected,    Pairing.ecMul(p.pi1AggH,  r));
        }

        // ── 步骤 5：一致性校验 ────────────────────────────────────────────────
        if (!_g1Equal(comQXExpected, comQXTotal)) return false;
        if (!_g1Equal(pi1Expected,   pi1Total))   return false;

        // ── 步骤 6：推导聚合 ρ ───────────────────────────────────────────────
        uint256 rho = _deriveRhoBatch(comQXTotal, dYTotal, pi1Total);

        // ── 步骤 7：组装 4 个配对点 ───────────────────────────────────────────
        // P0 = ComQXTotal
        // P1 = ρ·Pi1Total
        // P2 = ωα·C1_total + α·C2_total - ρ·D_{Y,total}
        // P3 = -(C1_total + C2_total)
        Pairing.G1Point memory p1 = Pairing.ecMul(pi1Total, rho);

        Pairing.G1Point memory p2;
        {
            Pairing.G1Point memory t1 = Pairing.ecMul(c1Total, alphaShifted);
            Pairing.G1Point memory t2 = Pairing.ecMul(c2Total, sharedAlpha);
            Pairing.G1Point memory t3 = Pairing.ecMul(dYTotal, rho);
            p2 = Pairing.ecAdd(Pairing.ecAdd(t1, t2), Pairing.ecNeg(t3));
        }

        Pairing.G1Point memory p3 = Pairing.ecNeg(Pairing.ecAdd(c1Total, c2Total));

        // ── 步骤 8：4 配对检验 ────────────────────────────────────────────────
        return Pairing.ecPairing4(
            comQXTotal, zTG2,
            p1,         tauYBetaG2,
            p2,         vk.g2_0,
            p3,         vk.g2_1
        );
    }

    // ────────────────────────────────────────────────────────────────────────
    // 共享挑战派生
    // ────────────────────────────────────────────────────────────────────────

    /// @dev SHA256("coord-alpha" || Hx[0..2]^{(0)} || ... || Hx[0..2]^{(K-1)}) mod Fr
    ///      预分配 K×96 字节，用 assembly 逐项写入，避免循环累加导致 O(K²) 内存复制。
    function _deriveSharedAlpha(CompressedProof[] calldata proofs)
        internal pure returns (uint256)
    {
        uint256 K = proofs.length;
        bytes memory pts = new bytes(K * 96);
        for (uint256 k = 0; k < K; k++) {
            bytes32 c0 = _g1Compressed(proofs[k].hx0.x, proofs[k].hx0.y);
            bytes32 c1 = _g1Compressed(proofs[k].hx1.x, proofs[k].hx1.y);
            bytes32 c2 = _g1Compressed(proofs[k].hx2.x, proofs[k].hx2.y);
            uint256 off = k * 96;
            assembly ("memory-safe") {
                let base := add(add(pts, 0x20), off)
                mstore(base,      c0)
                mstore(add(base, 32), c1)
                mstore(add(base, 64), c2)
            }
        }
        return uint256(sha256(abi.encodePacked("coord-alpha", pts))) % FR;
    }

    /// @dev SHA256("coord-beta" || alpha || ComQX^{(k)} || ComVFAlpha^{(k)} || ComVFZS^{(k)} || ...) mod Fr
    ///      预分配 K×96 字节，用 assembly 逐项写入，避免循环累加导致 O(K²) 内存复制。
    function _deriveSharedBeta(uint256 alpha, CompressedProof[] calldata proofs)
        internal pure returns (uint256)
    {
        uint256 K = proofs.length;
        bytes memory pts = new bytes(K * 96);
        for (uint256 k = 0; k < K; k++) {
            bytes32 c0 = _g1Compressed(proofs[k].comQX.x,      proofs[k].comQX.y);
            bytes32 c1 = _g1Compressed(proofs[k].comVFAlpha.x, proofs[k].comVFAlpha.y);
            bytes32 c2 = _g1Compressed(proofs[k].comVFZS.x,    proofs[k].comVFZS.y);
            uint256 off = k * 96;
            assembly ("memory-safe") {
                let base := add(add(pts, 0x20), off)
                mstore(base,      c0)
                mstore(add(base, 32), c1)
                mstore(add(base, 64), c2)
            }
        }
        return uint256(sha256(abi.encodePacked("coord-beta", bytes32(alpha), pts))) % FR;
    }

    // ────────────────────────────────────────────────────────────────────────
    // Per-proof Fiat-Shamir（γ / η / λ）
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 重放与 VerifyCompressed 完全相同的 FS 转录前三步（gamma、eta、lambda）。
    ///      alpha/nu/beta/mu 在聚合路径下由协调哈希推导，此处不推导。
    function _derivePerProofFS(
        CompressedProof calldata p,
        uint256[] calldata piFlat
    ) internal view returns (uint256 gamma, uint256 eta, uint256 lambda_) {
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
            for (uint256 i = 0; i < piFlat.length; i++) {
                buf = abi.encodePacked(buf, bytes32(piFlat[i]));
            }
            buf = abi.encodePacked(buf,
                _g1Compressed(p.lro0.x, p.lro0.y),
                _g1Compressed(p.lro1.x, p.lro1.y),
                _g1Compressed(p.lro2.x, p.lro2.y)
            );
            gammaHash = sha256(buf);
        }
        gamma = uint256(gammaHash) % FR;

        bytes32 etaHash = sha256(abi.encodePacked("eta", gammaHash));
        eta = uint256(etaHash) % FR;

        lambda_ = uint256(sha256(abi.encodePacked(
            "lambda", etaHash, _g1Compressed(p.z.x, p.z.y)
        ))) % FR;
    }

    // ────────────────────────────────────────────────────────────────────────
    // 协调哈希（ν / μ）
    // ────────────────────────────────────────────────────────────────────────

    /// @dev ν_k = SHA256("coord-nu" || alpha || foldedHx || LRO[0..2] || Z) mod Fr
    function _deriveNuCoord(
        uint256 alpha,
        Pairing.G1Point memory foldedHx,
        CompressedProof calldata p
    ) internal pure returns (uint256) {
        return uint256(sha256(abi.encodePacked(
            "coord-nu",
            bytes32(alpha),
            _g1Compressed(foldedHx.x, foldedHx.y),
            _g1Compressed(p.lro0.x,   p.lro0.y),
            _g1Compressed(p.lro1.x,   p.lro1.y),
            _g1Compressed(p.lro2.x,   p.lro2.y),
            _g1Compressed(p.z.x,      p.z.y)
        ))) % FR;
    }

    /// @dev μ = SHA256("coord-mu" || beta) mod Fr（与 k 无关）
    function _deriveMuCoord(uint256 beta) internal pure returns (uint256) {
        return uint256(sha256(abi.encodePacked("coord-mu", bytes32(beta)))) % FR;
    }

    // ────────────────────────────────────────────────────────────────────────
    // 聚合系数 r_k
    // ────────────────────────────────────────────────────────────────────────

    /// @dev r_k = SHA256("agg-rk" || k_4BE || serialize(all_proofs)) mod Fr
    ///      预构建 hashBuf 一次（"agg-rk" + k(4B) + allData），每轮仅就地更新 4 字节 k，
    ///      避免 K 次 abi.encodePacked 各自复制 K×864 字节导致 O(K²) 开销。
    function _deriveAggCoeffs(CompressedProof[] calldata proofs)
        internal pure returns (uint256[] memory rk)
    {
        bytes memory allData = _serializeAllProofs(proofs);
        uint256 K = proofs.length;
        rk = new uint256[](K);

        // hashBuf 布局：[0..5]="agg-rk"  [6..9]=k(big-endian)  [10..]=allData
        uint256 allDataLen = allData.length;
        bytes memory hashBuf = new bytes(10 + allDataLen);
        assembly ("memory-safe") {
            // 写 "agg-rk"（6字节）到 hashBuf[0..5]
            // hex: 61 67 67 2d 72 6b = "agg-rk"，高 6 字节置入，其余 0
            mstore(add(hashBuf, 0x20),
                0x6167672d726b0000000000000000000000000000000000000000000000000000)
            // 将 allData 复制到 hashBuf[10..]
            let dst := add(add(hashBuf, 0x20), 10)
            let src := add(allData, 0x20)
            for { let i := 0 } lt(i, allDataLen) { i := add(i, 32) } {
                mstore(add(dst, i), mload(add(src, i)))
            }
        }

        for (uint256 k = 0; k < K; k++) {
            // 就地更新 hashBuf[6..9] = uint32(k) big-endian
            // bytes 6-9 对应 256-bit word 的 bits 207-176，即 shl(176, ...)
            assembly ("memory-safe") {
                let ptr := add(hashBuf, 0x20)
                let w   := mload(ptr)
                w := or(and(w, not(shl(176, 0xFFFFFFFF))), shl(176, k))
                mstore(ptr, w)
            }
            rk[k] = uint256(sha256(hashBuf)) % FR;
        }
    }

    /// @dev 将 K 个压缩证明序列化为字节（用于推导聚合系数）。
    ///      格式：K × (12 G1 compressed + 15 Fr BE) = K × 864 bytes。
    ///      预分配一次，逐字段写入（每次仅 2-3 个栈变量），避免 abi.encodePacked 累加与栈溢出。
    function _serializeAllProofs(CompressedProof[] calldata proofs)
        internal pure returns (bytes memory allData)
    {
        uint256 K = proofs.length;
        allData = new bytes(K * 864);
        for (uint256 k = 0; k < K; k++) {
            CompressedProof calldata p = proofs[k];
            bytes32 v;
            uint256 o = k * 864;  // 当前写入位置相对于 allData 数据区的偏移
            // 12 个 G1 compressed 字段，顺序与原 abi.encodePacked 完全一致
            v = _g1Compressed(p.lro0.x,       p.lro0.y);      assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.lro1.x,       p.lro1.y);      assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.lro2.x,       p.lro2.y);      assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.z.x,          p.z.y);          assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.hx0.x,        p.hx0.y);        assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.hx1.x,        p.hx1.y);        assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.hx2.x,        p.hx2.y);        assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.comQX.x,      p.comQX.y);      assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.comVFAlpha.x, p.comVFAlpha.y); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.comVFZS.x,    p.comVFZS.y);    assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.comGY.x,      p.comGY.y);      assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = _g1Compressed(p.pi1AggH.x,    p.pi1AggH.y);    assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            // 15 个 Fr 标量字段
            v = bytes32(p.evalA);  assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalB);  assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalO);  assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalZ);  assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalZS); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalHx); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalHy); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalQl); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalQr); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalQm); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalQo); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalQk); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalS1); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalS2); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
            v = bytes32(p.evalS3); assembly ("memory-safe") { mstore(add(add(allData, 32), o), v) } o += 32;
        }
    }

    // ────────────────────────────────────────────────────────────────────────
    // 代数约束检验
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 与 BPianoVerifier._algebraicCheck 相同，但使用共享 α/β 而非 FS 推导值。
    function _algebraicCheck(
        CompressedProof calldata p,
        uint256 alpha,
        uint256 beta,
        uint256 gamma,
        uint256 eta,
        uint256 lambda_
    ) internal view returns (bool) {
        // gate = ql·a + qr·b + qm·a·b + qo·o + qk
        uint256 gate;
        {
            uint256 t;
            gate = mulmod(p.evalQl, p.evalA, FR);
            t    = mulmod(p.evalQr, p.evalB, FR); gate = addmod(gate, t, FR);
            t    = mulmod(mulmod(p.evalQm, p.evalA, FR), p.evalB, FR); gate = addmod(gate, t, FR);
            t    = mulmod(p.evalQo, p.evalO, FR); gate = addmod(gate, t, FR);
            gate = addmod(gate, p.evalQk, FR);
        }

        // L0(α) = (α^T - 1) / (T · (α - 1))
        uint256 alphaPowT = _modexp(alpha, vk.sizeX, FR);
        uint256 vanX      = addmod(alphaPowT, FR - 1, FR);
        uint256 l0;
        {
            uint256 denom = mulmod(vk.sizeX % FR, addmod(alpha, FR - 1, FR), FR);
            l0 = mulmod(vanX, _modInv(denom), FR);
        }

        uint256 boundary = mulmod(l0, addmod(p.evalZ, FR - 1, FR), FR);

        uint256 u   = vk.cosetShift;
        uint256 uSq = mulmod(u, u, FR);

        uint256 F;
        {
            uint256 f0 = addmod(addmod(p.evalA, mulmod(eta, alpha,               FR), FR), gamma, FR);
            uint256 f1 = addmod(addmod(p.evalB, mulmod(eta, mulmod(u,   alpha, FR), FR), FR), gamma, FR);
            uint256 f2 = addmod(addmod(p.evalO, mulmod(eta, mulmod(uSq, alpha, FR), FR), FR), gamma, FR);
            F = mulmod(mulmod(mulmod(f0, f1, FR), f2, FR), p.evalZ, FR);
        }
        uint256 G;
        {
            uint256 g0 = addmod(addmod(p.evalA, mulmod(eta, p.evalS1, FR), FR), gamma, FR);
            uint256 g1 = addmod(addmod(p.evalB, mulmod(eta, p.evalS2, FR), FR), gamma, FR);
            uint256 g2 = addmod(addmod(p.evalO, mulmod(eta, p.evalS3, FR), FR), gamma, FR);
            G = mulmod(mulmod(mulmod(g0, g1, FR), g2, FR), p.evalZS, FR);
        }
        uint256 perm = addmod(G, FR - F, FR);

        uint256 inner = addmod(mulmod(lambda_, boundary, FR), perm, FR);
        uint256 lhs   = addmod(gate, mulmod(lambda_, inner, FR), FR);

        uint256 betaPowM = _modexp(beta, vk.sizeY, FR);
        uint256 vanY     = addmod(betaPowM, FR - 1, FR);
        uint256 rhs      = addmod(mulmod(vanX, p.evalHx, FR), mulmod(vanY, p.evalHy, FR), FR);

        return lhs == rhs;
    }

    // ────────────────────────────────────────────────────────────────────────
    // G1 线性组合：foldedHx / C1 / C2 / D_Y
    // ────────────────────────────────────────────────────────────────────────

    /// @dev foldedHx_k = Hx[0] + α^T·Hx[1] + α^{2T}·Hx[2]
    function _computeFoldedHx(
        CompressedProof calldata p,
        uint256 alphaPowT
    ) internal view returns (Pairing.G1Point memory) {
        uint256 alphaPow2T = mulmod(alphaPowT, alphaPowT, FR);
        Pairing.G1Point memory r = p.hx0;
        r = Pairing.ecAdd(r, Pairing.ecMul(p.hx1, alphaPowT));
        r = Pairing.ecAdd(r, Pairing.ecMul(p.hx2, alphaPow2T));
        return r;
    }

    /// @dev 构建 C1_k 和 C2_k（与 BPianoVerifier._buildPairingPoints 相同公式，使用 ν_k）。
    ///      C1 = Σ_{i=0..12} ν^i·com_i - (Σ_{i=5..12} ν^i·eval_i)·g1 - ComVFAlpha
    ///      C2 = ν^13·(Z - ComVFZS)
    function _buildC1C2(
        CompressedProof calldata p,
        Pairing.G1Point memory foldedHx,
        uint256 nu
    ) internal view returns (Pairing.G1Point memory c1, Pairing.G1Point memory c2) {
        // ν^0..ν^13
        uint256[14] memory nuPow;
        nuPow[0] = 1;
        for (uint256 i = 1; i < 14; i++) {
            nuPow[i] = mulmod(nuPow[i - 1], nu, FR);
        }

        // C1 承诺部分：Σ_{i=0..12} ν^i · com_i
        c1 = Pairing.ecMul(foldedHx,  nuPow[0]);
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.lro0,  nuPow[1]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.lro1,  nuPow[2]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.lro2,  nuPow[3]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(p.z,     nuPow[4]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.ql,   nuPow[5]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qr,   nuPow[6]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qm,   nuPow[7]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qo,   nuPow[8]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.qk,   nuPow[9]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.s1,   nuPow[10]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.s2,   nuPow[11]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(vk.s3,   nuPow[12]));

        // 减去共享多项式 eval 之和（k=5..12）
        uint256 sharedEvalSum;
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[5],  p.evalQl, FR), FR);
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[6],  p.evalQr, FR), FR);
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[7],  p.evalQm, FR), FR);
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[8],  p.evalQo, FR), FR);
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[9],  p.evalQk, FR), FR);
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[10], p.evalS1, FR), FR);
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[11], p.evalS2, FR), FR);
        sharedEvalSum = addmod(sharedEvalSum, mulmod(nuPow[12], p.evalS3, FR), FR);
        c1 = Pairing.ecAdd(c1, Pairing.ecNeg(
            Pairing.ecMul(Pairing.G1Point(G1X, G1Y), sharedEvalSum)));

        // 减去 ComVFAlpha
        c1 = Pairing.ecAdd(c1, Pairing.ecNeg(p.comVFAlpha));

        // C2 = ν^13 · (Z - ComVFZS)
        c2 = Pairing.ecAdd(
            Pairing.ecMul(p.z,       nuPow[13]),
            Pairing.ecNeg(Pairing.ecMul(p.comVFZS, nuPow[13]))
        );
    }

    /// @dev D_{Y,k} = ComGY_k - G_Y(β)·g1
    ///      G_Y(β) = Σ_{j=0..6} μ^j · [EvalHx, EvalA, EvalB, EvalO, EvalZ, EvalZS, EvalHy]
    function _buildDY(
        CompressedProof calldata p,
        uint256 mu
    ) internal view returns (Pairing.G1Point memory dY) {
        uint256 gYBeta;
        uint256 muK = 1;
        gYBeta = addmod(gYBeta, mulmod(muK, p.evalHx,  FR), FR); muK = mulmod(muK, mu, FR);
        gYBeta = addmod(gYBeta, mulmod(muK, p.evalA,   FR), FR); muK = mulmod(muK, mu, FR);
        gYBeta = addmod(gYBeta, mulmod(muK, p.evalB,   FR), FR); muK = mulmod(muK, mu, FR);
        gYBeta = addmod(gYBeta, mulmod(muK, p.evalO,   FR), FR); muK = mulmod(muK, mu, FR);
        gYBeta = addmod(gYBeta, mulmod(muK, p.evalZ,   FR), FR); muK = mulmod(muK, mu, FR);
        gYBeta = addmod(gYBeta, mulmod(muK, p.evalZS,  FR), FR); muK = mulmod(muK, mu, FR);
        gYBeta = addmod(gYBeta, mulmod(muK, p.evalHy,  FR), FR);

        dY = Pairing.ecAdd(
            p.comGY,
            Pairing.ecNeg(Pairing.ecMul(Pairing.G1Point(G1X, G1Y), gYBeta))
        );
    }

    // ────────────────────────────────────────────────────────────────────────
    // 聚合 ρ 派生
    // ────────────────────────────────────────────────────────────────────────

    /// @dev ρ = SHA256("agg-rho" || ComQXTotal || D_{Y,total} || Pi1Total) mod Fr
    function _deriveRhoBatch(
        Pairing.G1Point memory comQXTotal,
        Pairing.G1Point memory dYTotal,
        Pairing.G1Point memory pi1Total
    ) internal pure returns (uint256) {
        return uint256(sha256(abi.encodePacked(
            "agg-rho",
            _g1Compressed(comQXTotal.x, comQXTotal.y),
            _g1Compressed(dYTotal.x,    dYTotal.y),
            _g1Compressed(pi1Total.x,   pi1Total.y)
        ))) % FR;
    }

    // ────────────────────────────────────────────────────────────────────────
    // 内部辅助
    // ────────────────────────────────────────────────────────────────────────

    function _g1Equal(Pairing.G1Point memory a, Pairing.G1Point memory b)
        internal pure returns (bool)
    {
        return a.x == b.x && a.y == b.y;
    }

    /// @dev G1 点压缩为 32 字节（gnark-crypto Bytes() 格式）。
    function _g1Compressed(uint256 x, uint256 y) internal pure returns (bytes32) {
        if (x == 0 && y == 0) {
            return bytes32(uint256(0x40) << 248);
        }
        uint256 flag = (y > HALF_FP) ? 0xC0 : 0x80;
        return bytes32(x | (flag << 248));
    }

    /// @dev 模幂：base^exp mod mod，使用预编译 0x05。
    function _modexp(uint256 base, uint256 exponent, uint256 modulus)
        internal view returns (uint256 result)
    {
        assembly ("memory-safe") {
            let ptr := mload(0x40)
            mstore(ptr,            0x20)
            mstore(add(ptr, 0x20), 0x20)
            mstore(add(ptr, 0x40), 0x20)
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
