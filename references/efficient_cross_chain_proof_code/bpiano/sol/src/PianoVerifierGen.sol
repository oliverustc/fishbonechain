// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./Pairing.sol";

/// @title PianoVerifierGen
/// @notice Piano 协议的链上 Solidity 验证合约（VK 常量硬编码版）。
///
/// 验证流程（对应 bpiano/piano/verify.go）：
///   1. 重放 Fiat-Shamir 转录（SHA-256），推导 5 个挑战：γ, η, λ, α, β
///   2. 代数约束检验（纯域运算）
///   3. X 轴 DKZG 批量验证（1 次 2-pairing）
///   4. Y 轴 DKZG 批量验证（1 次 2-pairing）
///
/// EVM 缺少 G2 标量乘预编译，tauYBetaG2 须由 Go 侧（solgen.GeneratePianoCalldata）
/// 链下预计算并作为 calldata 传入。
contract PianoVerifierGen {
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

    // ── VK 常量（由电路 Setup 确定，硬编码于合约字节码中）───────────────────────
    uint256 internal constant VK_QL_X          = 0x17bb1895ec19e858567b189c97c95af05cc788d4301f2299b259c54f9267a4c5;
    uint256 internal constant VK_QL_Y          = 0x2c9195e2eefe71f3108424e76e07c973972a39327d7d54f4e38ea8cc04eb8062;
    uint256 internal constant VK_QR_X          = 0x17bb1895ec19e858567b189c97c95af05cc788d4301f2299b259c54f9267a4c5;
    uint256 internal constant VK_QR_Y          = 0x3d2b88ff2332e36a7cc20cf13798eea0057315eeaf475985891e34ad3917ce5;
    uint256 internal constant VK_QM_X          = 0x0;
    uint256 internal constant VK_QM_Y          = 0x0;
    uint256 internal constant VK_QO_X          = 0x0;
    uint256 internal constant VK_QO_Y          = 0x0;
    uint256 internal constant VK_QK_X          = 0x0;
    uint256 internal constant VK_QK_Y          = 0x0;
    uint256 internal constant VK_S1_X          = 0x104b9282799e76d0979d1662106d307efd62b879b3868aedf3b6a809c1709cfe;
    uint256 internal constant VK_S1_Y          = 0x1f2ad4d247b59b2781d5b6fb275771acef42f680906347c2635e356e6814052e;
    uint256 internal constant VK_S2_X          = 0x2250843f3a026847e0afa999f42e2c76c9c1f44827540f4e328d688278667f34;
    uint256 internal constant VK_S2_Y          = 0x18e39f33a8c7172cdfef729e2fa9b8a5c38b6b4ecc61b577a54608d121938f75;
    uint256 internal constant VK_S3_X          = 0x20ccc87421de498801f61ca5f558593355e917d547adeb7db98c3ec548d3aab8;
    uint256 internal constant VK_S3_Y          = 0x197f683fd6fb1baabf85c952a7c17d6f78d25fe4ba681a560d3bbeb7a407dffa;
    uint256 internal constant VK_G2_0_XI       = 0x198e9393920d483a7260bfb731fb5d25f1aa493335a9e71297e485b7aef312c2;
    uint256 internal constant VK_G2_0_XR       = 0x1800deef121f1e76426a00665e5c4479674322d4f75edadd46debd5cd992f6ed;
    uint256 internal constant VK_G2_0_YI       = 0x90689d0585ff075ec9e99ad690c3395bc4b313370b38ef355acdadcd122975b;
    uint256 internal constant VK_G2_0_YR       = 0x12c85ea5db8c6deb4aab71808dcb408fe3d1e7690c43d37b4ce6cc0166fa7daa;
    uint256 internal constant VK_G2_1_XI       = 0x227071bba5ff3b47ed8b504bb5b215bc701d7a3259b933bff1a4164eae499c2c;
    uint256 internal constant VK_G2_1_XR       = 0xc51a367b61d3119677b29739ddccbb78002b5558d8f49ff16e299c1b41f8098;
    uint256 internal constant VK_G2_1_YI       = 0x8bb188b2a6187bb1e87834c85a6a917763d65b98febf2c45ea339dd77fac415;
    uint256 internal constant VK_G2_1_YR       = 0x18fd2fd13be8494c39e8a91325d1ef3ba7d1a205d10788e38bc9e09d9be87769;
    uint256 internal constant VK_G2Y_0_XI      = 0x198e9393920d483a7260bfb731fb5d25f1aa493335a9e71297e485b7aef312c2;
    uint256 internal constant VK_G2Y_0_XR      = 0x1800deef121f1e76426a00665e5c4479674322d4f75edadd46debd5cd992f6ed;
    uint256 internal constant VK_G2Y_0_YI      = 0x90689d0585ff075ec9e99ad690c3395bc4b313370b38ef355acdadcd122975b;
    uint256 internal constant VK_G2Y_0_YR      = 0x12c85ea5db8c6deb4aab71808dcb408fe3d1e7690c43d37b4ce6cc0166fa7daa;
    uint256 internal constant VK_SIZE_X        = 0x8;
    uint256 internal constant VK_SIZE_Y        = 0x2;
    uint256 internal constant VK_GENERATOR_X   = 0x2b337de1c8c14f22ec9b9e2f96afef3652627366f8170a0a948dad4ac1bd5e80;
    uint256 internal constant VK_COSET_SHIFT   = 0x5;
    uint256 internal constant VK_NB_PUBLIC_INPUTS = 0x0;

    // ────────────────────────────────────────────────────────────────────────
    // 类型定义
    // ────────────────────────────────────────────────────────────────────────

    /// @notice Piano 证明。
    struct PianoProof {
        // G1 承诺
        Pairing.G1Point lro0;    // com_A
        Pairing.G1Point lro1;    // com_B
        Pairing.G1Point lro2;    // com_O
        Pairing.G1Point z;       // com_Z
        Pairing.G1Point hx0;
        Pairing.G1Point hx1;
        Pairing.G1Point hx2;
        Pairing.G1Point hy0;
        Pairing.G1Point hy1;
        Pairing.G1Point hy2;
        Pairing.G1Point batchXH;  // BatchedProofX.H
        // BatchedProofX.ClaimedDigests[13]
        Pairing.G1Point cd0;
        Pairing.G1Point cd1;
        Pairing.G1Point cd2;
        Pairing.G1Point cd3;
        Pairing.G1Point cd4;
        Pairing.G1Point cd5;
        Pairing.G1Point cd6;
        Pairing.G1Point cd7;
        Pairing.G1Point cd8;
        Pairing.G1Point cd9;
        Pairing.G1Point cd10;
        Pairing.G1Point cd11;
        Pairing.G1Point cd12;
        Pairing.G1Point zsH;     // ZShiftedProofX.H
        Pairing.G1Point zsComVF; // ZShiftedProofX.ComVF
        Pairing.G1Point batchYH; // BatchedProofY.H

        // Fr 标量求值
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
        // BatchedProofY.ClaimedValues[15]
        uint256 byv0;
        uint256 byv1;
        uint256 byv2;
        uint256 byv3;
        uint256 byv4;
        uint256 byv5;
        uint256 byv6;
        uint256 byv7;
        uint256 byv8;
        uint256 byv9;
        uint256 byv10;
        uint256 byv11;
        uint256 byv12;
        uint256 byv13;
        uint256 byv14;
    }

    /// @dev 内部 FS 挑战缓存。
    struct Challenges {
        uint256 gamma;
        uint256 eta;
        uint256 lambda_;    // lambda 是 Solidity 保留字
        uint256 alpha;
        uint256 beta;
        uint256 alphaShifted; // ω_X · α mod Fr
    }

    // ────────────────────────────────────────────────────────────────────────
    // 主入口
    // ────────────────────────────────────────────────────────────────────────

    /// @notice 验证 Piano 证明。
    /// @param proof          Piano 证明（27 G1 + 30 Fr）
    /// @param tauYBetaG2     [τ_Y - β]₂，由 Go 链下预计算
    /// @param publicInputsFlat 展平的公开输入（M × nbPublicInputs 个 Fr 元素）
    /// @return true 当且仅当证明合法
    function verify(
        PianoProof calldata proof,
        Pairing.G2Point calldata tauYBetaG2,
        uint256[] calldata publicInputsFlat
    ) external view returns (bool) {
        // 步骤 1：重放 Fiat-Shamir 挑战
        Challenges memory ch = _replayFS(proof, publicInputsFlat);

        // 步骤 2：代数约束检验
        if (!_algebraicCheck(proof, ch)) return false;

        // 步骤 3：X 轴 DKZG 验证
        if (!_verifyX(proof, ch)) return false;

        // 步骤 4：Y 轴 DKZG 验证
        if (!_verifyY(proof, ch, tauYBetaG2)) return false;

        return true;
    }

    // ────────────────────────────────────────────────────────────────────────
    // Fiat-Shamir 转录
    // ────────────────────────────────────────────────────────────────────────

    function _replayFS(
        PianoProof calldata p,
        uint256[] calldata piFlat
    ) internal view returns (Challenges memory ch) {
        // ── gamma ────────────────────────────────────────────────────────────
        bytes32 gammaHash;
        {
            bytes memory buf = abi.encodePacked(
                "gamma",
                _g1Compressed(VK_QL_X, VK_QL_Y),
                _g1Compressed(VK_QR_X, VK_QR_Y),
                _g1Compressed(VK_QM_X, VK_QM_Y),
                _g1Compressed(VK_QO_X, VK_QO_Y),
                _g1Compressed(VK_QK_X, VK_QK_Y),
                _g1Compressed(VK_S1_X, VK_S1_Y),
                _g1Compressed(VK_S2_X, VK_S2_Y),
                _g1Compressed(VK_S3_X, VK_S3_Y)
            );
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

        // ── beta ─────────────────────────────────────────────────────────────
        // 绑定: batchXH + claimedDigs[0..12] + hy0..hy2
        {
            bytes memory buf2 = abi.encodePacked(
                "beta", alphaHash,
                _g1Compressed(p.batchXH.x, p.batchXH.y),
                _g1Compressed(p.cd0.x, p.cd0.y),
                _g1Compressed(p.cd1.x, p.cd1.y),
                _g1Compressed(p.cd2.x, p.cd2.y),
                _g1Compressed(p.cd3.x, p.cd3.y),
                _g1Compressed(p.cd4.x, p.cd4.y),
                _g1Compressed(p.cd5.x, p.cd5.y),
                _g1Compressed(p.cd6.x, p.cd6.y)
            );
            buf2 = abi.encodePacked(
                buf2,
                _g1Compressed(p.cd7.x, p.cd7.y),
                _g1Compressed(p.cd8.x, p.cd8.y),
                _g1Compressed(p.cd9.x, p.cd9.y),
                _g1Compressed(p.cd10.x, p.cd10.y),
                _g1Compressed(p.cd11.x, p.cd11.y),
                _g1Compressed(p.cd12.x, p.cd12.y),
                _g1Compressed(p.hy0.x, p.hy0.y),
                _g1Compressed(p.hy1.x, p.hy1.y),
                _g1Compressed(p.hy2.x, p.hy2.y)
            );
            bytes32 betaHash = sha256(buf2);
            ch.beta = uint256(betaHash) % FR;
        }

        // ── alphaShifted = α · ω_X mod Fr ────────────────────────────────────
        ch.alphaShifted = mulmod(ch.alpha, VK_GENERATOR_X, FR);
    }

    // ────────────────────────────────────────────────────────────────────────
    // 代数约束
    // ────────────────────────────────────────────────────────────────────────

    function _algebraicCheck(
        PianoProof calldata p,
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
        uint256 alphaPowT = _modexp(alpha, VK_SIZE_X, FR);
        uint256 vanX      = addmod(alphaPowT, FR - 1, FR); // α^T - 1
        uint256 l0;
        {
            uint256 denom = mulmod(
                VK_SIZE_X % FR,
                addmod(alpha, FR - 1, FR),
                FR
            );
            l0 = mulmod(vanX, _modInv(denom), FR);
        }

        // boundary = L0 · (z - 1)
        uint256 boundary = mulmod(l0, addmod(z, FR - 1, FR), FR);

        // 置换项
        uint256 u    = VK_COSET_SHIFT;
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
        uint256 betaPowM = _modexp(beta, VK_SIZE_Y, FR);
        uint256 vanY     = addmod(betaPowM, FR - 1, FR); // β^M - 1
        uint256 rhs      = addmod(mulmod(vanX, hx, FR), mulmod(vanY, hy, FR), FR);

        return lhs == rhs;
    }

    // ────────────────────────────────────────────────────────────────────────
    // X 轴 DKZG 验证
    // ────────────────────────────────────────────────────────────────────────

    function _verifyX(
        PianoProof calldata p,
        Challenges memory ch
    ) internal view returns (bool) {
        // foldedHxDig = hx0 + α^T·hx1 + α^{2T}·hx2
        uint256 alphaPowT  = _modexp(ch.alpha, VK_SIZE_X, FR);
        uint256 alphaPow2T = mulmod(alphaPowT, alphaPowT, FR);

        Pairing.G1Point memory foldedHxDig = Pairing.ecAdd(
            p.hx0,
            Pairing.ecAdd(
                Pairing.ecMul(p.hx1, alphaPowT),
                Pairing.ecMul(p.hx2, alphaPow2T)
            )
        );

        // gammaX = SHA256(bytes32(alpha) || compressed(batchComFs[0..12]))
        uint256 gammaX = _deriveGammaX(p, foldedHxDig, ch.alpha);

        // foldedComF, foldedComVF
        Pairing.G1Point memory foldedComF;
        Pairing.G1Point memory foldedComVF;
        (foldedComF, foldedComVF) = _computeFoldedXPoints(p, foldedHxDig, gammaX);

        // r = SHA256(bytes32(alpha) || compressed(foldedComF) || compressed(z))
        uint256 r;
        {
            bytes32 rRaw = sha256(abi.encodePacked(
                bytes32(ch.alpha),
                _g1Compressed(foldedComF.x, foldedComF.y),
                _g1Compressed(p.z.x, p.z.y)
            ));
            r = uint256(rRaw) % FR;
            if (r == 0) r = 1;
        }

        // LHS1 = foldedComF - foldedComVF + α·batchXH
        // LHS2 = z - zsComVF + ω·α·zsH
        // LHS  = LHS1 + r·LHS2
        // RHS  = batchXH + r·zsH
        Pairing.G1Point memory LHS;
        Pairing.G1Point memory RHS;
        {
            Pairing.G1Point memory lhs1 = Pairing.ecAdd(
                Pairing.ecAdd(foldedComF, Pairing.ecNeg(foldedComVF)),
                Pairing.ecMul(p.batchXH, ch.alpha)
            );
            Pairing.G1Point memory lhs2 = Pairing.ecAdd(
                Pairing.ecAdd(p.z, Pairing.ecNeg(p.zsComVF)),
                Pairing.ecMul(p.zsH, ch.alphaShifted)
            );
            LHS = Pairing.ecAdd(lhs1, Pairing.ecMul(lhs2, r));
            RHS = Pairing.ecAdd(p.batchXH, Pairing.ecMul(p.zsH, r));
        }

        // e(LHS, g2_0) · e(-RHS, g2_1) = 1
        return _pairing2(LHS, Pairing.G2Point(VK_G2_0_XI, VK_G2_0_XR, VK_G2_0_YI, VK_G2_0_YR),
                         Pairing.ecNeg(RHS), Pairing.G2Point(VK_G2_1_XI, VK_G2_1_XR, VK_G2_1_YI, VK_G2_1_YR));
    }

    function _deriveGammaX(
        PianoProof calldata p,
        Pairing.G1Point memory foldedHxDig,
        uint256 alpha
    ) internal view returns (uint256 gammaX) {
        bytes memory buf = abi.encodePacked(
            bytes32(alpha),
            _g1Compressed(foldedHxDig.x, foldedHxDig.y),
            _g1Compressed(p.lro0.x, p.lro0.y),
            _g1Compressed(p.lro1.x, p.lro1.y),
            _g1Compressed(p.lro2.x, p.lro2.y),
            _g1Compressed(VK_QL_X, VK_QL_Y),
            _g1Compressed(VK_QR_X, VK_QR_Y),
            _g1Compressed(VK_QM_X, VK_QM_Y),
            _g1Compressed(VK_QO_X, VK_QO_Y)
        );
        buf = abi.encodePacked(
            buf,
            _g1Compressed(VK_QK_X, VK_QK_Y),
            _g1Compressed(VK_S1_X, VK_S1_Y),
            _g1Compressed(VK_S2_X, VK_S2_Y),
            _g1Compressed(VK_S3_X, VK_S3_Y),
            _g1Compressed(p.z.x, p.z.y)
        );
        bytes32 h = sha256(buf);
        gammaX = uint256(h) % FR;
        if (gammaX == 0) gammaX = 1;
    }

    function _computeFoldedXPoints(
        PianoProof calldata p,
        Pairing.G1Point memory foldedHxDig,
        uint256 gammaX
    ) internal view returns (
        Pairing.G1Point memory foldedComF,
        Pairing.G1Point memory foldedComVF
    ) {
        uint256[13] memory gammaPow;
        gammaPow[0] = 1;
        for (uint256 i = 1; i < 13; i++) {
            gammaPow[i] = mulmod(gammaPow[i-1], gammaX, FR);
        }

        // batchComFs = [foldedHxDig, lro0,lro1,lro2, ql,qr,qm,qo,qk, s1,s2,s3, z]
        foldedComF = Pairing.ecMul(foldedHxDig, gammaPow[0]);
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(p.lro0, gammaPow[1]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(p.lro1, gammaPow[2]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(p.lro2, gammaPow[3]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_QL_X, VK_QL_Y), gammaPow[4]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_QR_X, VK_QR_Y), gammaPow[5]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_QM_X, VK_QM_Y), gammaPow[6]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_QO_X, VK_QO_Y), gammaPow[7]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_QK_X, VK_QK_Y), gammaPow[8]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_S1_X, VK_S1_Y), gammaPow[9]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_S2_X, VK_S2_Y), gammaPow[10]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(Pairing.G1Point(VK_S3_X, VK_S3_Y), gammaPow[11]));
        foldedComF = Pairing.ecAdd(foldedComF, Pairing.ecMul(p.z, gammaPow[12]));

        // claimedDigs = [cd0..cd12]
        foldedComVF = Pairing.ecMul(p.cd0, gammaPow[0]);
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd1, gammaPow[1]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd2, gammaPow[2]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd3, gammaPow[3]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd4, gammaPow[4]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd5, gammaPow[5]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd6, gammaPow[6]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd7, gammaPow[7]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd8, gammaPow[8]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd9, gammaPow[9]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd10, gammaPow[10]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd11, gammaPow[11]));
        foldedComVF = Pairing.ecAdd(foldedComVF, Pairing.ecMul(p.cd12, gammaPow[12]));
    }

    // ────────────────────────────────────────────────────────────────────────
    // Y 轴 DKZG 验证
    // ────────────────────────────────────────────────────────────────────────

    function _verifyY(
        PianoProof calldata p,
        Challenges memory ch,
        Pairing.G2Point calldata tauYBetaG2
    ) internal view returns (bool) {
        // foldedHyDig = hy0 + β^M·hy1 + β^{2M}·hy2
        uint256 betaPowM  = _modexp(ch.beta, VK_SIZE_Y, FR);
        uint256 betaPow2M = mulmod(betaPowM, betaPowM, FR);

        Pairing.G1Point memory foldedHyDig = Pairing.ecAdd(
            p.hy0,
            Pairing.ecAdd(
                Pairing.ecMul(p.hy1, betaPowM),
                Pairing.ecMul(p.hy2, betaPow2M)
            )
        );

        // gammaY = SHA256(bytes32(beta) || compressed(comVFs[0..14]))
        // comVFs = [cd0..cd12, zsComVF, foldedHyDig]
        uint256 gammaY = _deriveGammaY(p, foldedHyDig, ch.beta);

        // foldedComVF_Y, foldedValue
        Pairing.G1Point memory foldedComVFY;
        uint256 foldedValue;
        (foldedComVFY, foldedValue) = _computeFoldedYPoints(p, foldedHyDig, gammaY);

        // diff = foldedComVF_Y - foldedValue·G1
        Pairing.G1Point memory diff = Pairing.ecAdd(
            foldedComVFY,
            Pairing.ecNeg(Pairing.ecMul(Pairing.G1Point(G1X, G1Y), foldedValue))
        );

        // e(diff, g2y_0) · e(-batchYH, tauYBetaG2) = 1
        return _pairing2(diff, Pairing.G2Point(VK_G2Y_0_XI, VK_G2Y_0_XR, VK_G2Y_0_YI, VK_G2Y_0_YR),
                         Pairing.ecNeg(p.batchYH), tauYBetaG2);
    }

    function _deriveGammaY(
        PianoProof calldata p,
        Pairing.G1Point memory foldedHyDig,
        uint256 beta
    ) internal pure returns (uint256 gammaY) {
        bytes memory buf = abi.encodePacked(
            bytes32(beta),
            _g1Compressed(p.cd0.x, p.cd0.y),
            _g1Compressed(p.cd1.x, p.cd1.y),
            _g1Compressed(p.cd2.x, p.cd2.y),
            _g1Compressed(p.cd3.x, p.cd3.y),
            _g1Compressed(p.cd4.x, p.cd4.y),
            _g1Compressed(p.cd5.x, p.cd5.y),
            _g1Compressed(p.cd6.x, p.cd6.y),
            _g1Compressed(p.cd7.x, p.cd7.y)
        );
        buf = abi.encodePacked(
            buf,
            _g1Compressed(p.cd8.x, p.cd8.y),
            _g1Compressed(p.cd9.x, p.cd9.y),
            _g1Compressed(p.cd10.x, p.cd10.y),
            _g1Compressed(p.cd11.x, p.cd11.y),
            _g1Compressed(p.cd12.x, p.cd12.y),
            _g1Compressed(p.zsComVF.x, p.zsComVF.y),
            _g1Compressed(foldedHyDig.x, foldedHyDig.y)
        );
        bytes32 h = sha256(buf);
        gammaY = uint256(h) % FR;
        if (gammaY == 0) gammaY = 1;
    }

    function _computeFoldedYPoints(
        PianoProof calldata p,
        Pairing.G1Point memory foldedHyDig,
        uint256 gammaY
    ) internal view returns (
        Pairing.G1Point memory foldedComVFY,
        uint256 foldedValue
    ) {
        uint256[15] memory gammaPow;
        gammaPow[0] = 1;
        for (uint256 i = 1; i < 15; i++) {
            gammaPow[i] = mulmod(gammaPow[i-1], gammaY, FR);
        }

        // comVFs = [cd0..cd12, zsComVF, foldedHyDig]
        foldedComVFY = Pairing.ecMul(p.cd0, gammaPow[0]);
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd1,      gammaPow[1]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd2,      gammaPow[2]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd3,      gammaPow[3]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd4,      gammaPow[4]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd5,      gammaPow[5]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd6,      gammaPow[6]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd7,      gammaPow[7]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd8,      gammaPow[8]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd9,      gammaPow[9]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd10,     gammaPow[10]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd11,     gammaPow[11]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.cd12,     gammaPow[12]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(p.zsComVF,  gammaPow[13]));
        foldedComVFY = Pairing.ecAdd(foldedComVFY, Pairing.ecMul(foldedHyDig, gammaPow[14]));

        // batchYVals = [byv0..byv14]
        foldedValue = mulmod(gammaPow[0],  p.byv0,  FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[1],  p.byv1,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[2],  p.byv2,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[3],  p.byv3,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[4],  p.byv4,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[5],  p.byv5,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[6],  p.byv6,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[7],  p.byv7,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[8],  p.byv8,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[9],  p.byv9,  FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[10], p.byv10, FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[11], p.byv11, FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[12], p.byv12, FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[13], p.byv13, FR), FR);
        foldedValue = addmod(foldedValue, mulmod(gammaPow[14], p.byv14, FR), FR);
    }

    // ────────────────────────────────────────────────────────────────────────
    // 2-配对检验辅助
    // ────────────────────────────────────────────────────────────────────────

    /// @dev 2 配对检验：e(p0, g2_0) · e(p1, g2_1) == 1。
    function _pairing2(
        Pairing.G1Point memory p0, Pairing.G2Point memory g2_0,
        Pairing.G1Point memory p1, Pairing.G2Point memory g2_1
    ) internal view returns (bool) {
        uint256[12] memory inp = [
            p0.x,     p0.y,     g2_0.xIm, g2_0.xRe, g2_0.yIm, g2_0.yRe,
            p1.x,     p1.y,     g2_1.xIm, g2_1.xRe, g2_1.yIm, g2_1.yRe
        ];
        uint256[1] memory out;
        bool ok;
        assembly ("memory-safe") {
            ok := staticcall(
                sub(gas(), 2000),
                0x08,
                inp,
                0x180,
                out,
                0x20
            )
        }
        require(ok, "Pairing: ecPairing2 precompile failed");
        return out[0] == 1;
    }

    // ────────────────────────────────────────────────────────────────────────
    // 内部辅助
    // ────────────────────────────────────────────────────────────────────────

    /// @dev G1 点压缩为 32 字节（gnark-crypto Bytes() 格式）。
    function _g1Compressed(uint256 x, uint256 y)
        internal
        pure
        returns (bytes32)
    {
        if (x == 0 && y == 0) {
            return bytes32(uint256(0x40) << 248);
        }
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
