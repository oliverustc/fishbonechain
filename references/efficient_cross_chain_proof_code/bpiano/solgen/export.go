package solgen

import (
	"bytes"
	"math/big"
	"text/template"

	"github.com/oliverustc/bpiano/piano"
)

// vkTemplateData holds all VK field values as hex strings for template rendering.
type vkTemplateData struct {
	ContractName string
	// G1 points (hex strings with 0x prefix)
	VkQlX, VkQlY string
	VkQrX, VkQrY string
	VkQmX, VkQmY string
	VkQoX, VkQoY string
	VkQkX, VkQkY string
	VkS1X, VkS1Y string
	VkS2X, VkS2Y string
	VkS3X, VkS3Y string
	// G2 points (xIm, xRe, yIm, yRe - hex strings with 0x prefix)
	VkG2_0XI, VkG2_0XR, VkG2_0YI, VkG2_0YR string
	VkG2_1XI, VkG2_1XR, VkG2_1YI, VkG2_1YR string
	VkG2Y0XI, VkG2Y0XR, VkG2Y0YI, VkG2Y0YR string
	// Scalar params (hex strings with 0x prefix)
	VkSizeX, VkSizeY, VkGeneratorX, VkCosetShift, VkNbPublicInputs string
}

// bigIntToHex converts a *big.Int to a "0x"-prefixed lowercase hex string.
func bigIntToHex(b *big.Int) string {
	if b == nil || b.Sign() == 0 {
		return "0x0"
	}
	return "0x" + b.Text(16)
}

// fillVKData populates a vkTemplateData struct from a piano.VerifyingKey.
func fillVKData(vk *piano.VerifyingKey, contractName string) vkTemplateData {
	qlX, qlY := G1AffineToSolidity(vk.Ql)
	qrX, qrY := G1AffineToSolidity(vk.Qr)
	qmX, qmY := G1AffineToSolidity(vk.Qm)
	qoX, qoY := G1AffineToSolidity(vk.Qo)
	qkX, qkY := G1AffineToSolidity(vk.Qk)
	s1X, s1Y := G1AffineToSolidity(vk.S1)
	s2X, s2Y := G1AffineToSolidity(vk.S2)
	s3X, s3Y := G1AffineToSolidity(vk.S3)

	g2_0XI, g2_0XR, g2_0YI, g2_0YR := G2AffineToSolidity(vk.DKZGSRS.G2[0])
	g2_1XI, g2_1XR, g2_1YI, g2_1YR := G2AffineToSolidity(vk.DKZGSRS.G2[1])
	g2y0XI, g2y0XR, g2y0YI, g2y0YR := G2AffineToSolidity(vk.DKZGSRS.G2Y[0])

	generatorX := FrElementToSolidity(vk.GeneratorX)
	cosetShift := FrElementToSolidity(vk.CosetShift)

	sizeXBig := new(big.Int).SetUint64(vk.SizeX)
	sizeYBig := new(big.Int).SetUint64(vk.SizeY)
	nbPIBig := new(big.Int).SetUint64(uint64(vk.NbPublicInputs))

	return vkTemplateData{
		ContractName: contractName,
		VkQlX:        bigIntToHex(qlX),
		VkQlY:        bigIntToHex(qlY),
		VkQrX:        bigIntToHex(qrX),
		VkQrY:        bigIntToHex(qrY),
		VkQmX:        bigIntToHex(qmX),
		VkQmY:        bigIntToHex(qmY),
		VkQoX:        bigIntToHex(qoX),
		VkQoY:        bigIntToHex(qoY),
		VkQkX:        bigIntToHex(qkX),
		VkQkY:        bigIntToHex(qkY),
		VkS1X:        bigIntToHex(s1X),
		VkS1Y:        bigIntToHex(s1Y),
		VkS2X:        bigIntToHex(s2X),
		VkS2Y:        bigIntToHex(s2Y),
		VkS3X:        bigIntToHex(s3X),
		VkS3Y:        bigIntToHex(s3Y),
		VkG2_0XI:     bigIntToHex(g2_0XI),
		VkG2_0XR:     bigIntToHex(g2_0XR),
		VkG2_0YI:     bigIntToHex(g2_0YI),
		VkG2_0YR:     bigIntToHex(g2_0YR),
		VkG2_1XI:     bigIntToHex(g2_1XI),
		VkG2_1XR:     bigIntToHex(g2_1XR),
		VkG2_1YI:     bigIntToHex(g2_1YI),
		VkG2_1YR:     bigIntToHex(g2_1YR),
		VkG2Y0XI:     bigIntToHex(g2y0XI),
		VkG2Y0XR:     bigIntToHex(g2y0XR),
		VkG2Y0YI:     bigIntToHex(g2y0YI),
		VkG2Y0YR:     bigIntToHex(g2y0YR),
		VkGeneratorX: bigIntToHex(generatorX),
		VkCosetShift: bigIntToHex(cosetShift),
		VkSizeX:      bigIntToHex(sizeXBig),
		VkSizeY:      bigIntToHex(sizeYBig),
		VkNbPublicInputs: bigIntToHex(nbPIBig),
	}
}

// ExportBPianoVerifier generates a deployable BPianoVerifier .sol with VK constants hardcoded.
// contractName: Solidity contract name (e.g. "BPianoVerifier_Keccak")
// Returns the complete .sol source as a string.
func ExportBPianoVerifier(vk *piano.VerifyingKey, contractName string) (string, error) {
	tmpl, err := template.New("bpiano").Parse(bpianoVerifierTemplate)
	if err != nil {
		return "", err
	}
	data := fillVKData(vk, contractName)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ExportPianoVerifier generates a deployable PianoVerifier .sol with VK constants hardcoded.
func ExportPianoVerifier(vk *piano.VerifyingKey, contractName string) (string, error) {
	tmpl, err := template.New("piano").Parse(pianoVerifierTemplate)
	if err != nil {
		return "", err
	}
	data := fillVKData(vk, contractName)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// BPiano template
// ─────────────────────────────────────────────────────────────────────────────

var bpianoVerifierTemplate = `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./Pairing.sol";

/// @title {{.ContractName}}
/// @notice BPiano 压缩证明的链上 Solidity 验证合约（VK 常量硬编码版）。
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
contract {{.ContractName}} {
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
    uint256 internal constant VK_QL_X          = {{.VkQlX}};
    uint256 internal constant VK_QL_Y          = {{.VkQlY}};
    uint256 internal constant VK_QR_X          = {{.VkQrX}};
    uint256 internal constant VK_QR_Y          = {{.VkQrY}};
    uint256 internal constant VK_QM_X          = {{.VkQmX}};
    uint256 internal constant VK_QM_Y          = {{.VkQmY}};
    uint256 internal constant VK_QO_X          = {{.VkQoX}};
    uint256 internal constant VK_QO_Y          = {{.VkQoY}};
    uint256 internal constant VK_QK_X          = {{.VkQkX}};
    uint256 internal constant VK_QK_Y          = {{.VkQkY}};
    uint256 internal constant VK_S1_X          = {{.VkS1X}};
    uint256 internal constant VK_S1_Y          = {{.VkS1Y}};
    uint256 internal constant VK_S2_X          = {{.VkS2X}};
    uint256 internal constant VK_S2_Y          = {{.VkS2Y}};
    uint256 internal constant VK_S3_X          = {{.VkS3X}};
    uint256 internal constant VK_S3_Y          = {{.VkS3Y}};
    uint256 internal constant VK_G2_0_XI       = {{.VkG2_0XI}};
    uint256 internal constant VK_G2_0_XR       = {{.VkG2_0XR}};
    uint256 internal constant VK_G2_0_YI       = {{.VkG2_0YI}};
    uint256 internal constant VK_G2_0_YR       = {{.VkG2_0YR}};
    uint256 internal constant VK_G2_1_XI       = {{.VkG2_1XI}};
    uint256 internal constant VK_G2_1_XR       = {{.VkG2_1XR}};
    uint256 internal constant VK_G2_1_YI       = {{.VkG2_1YI}};
    uint256 internal constant VK_G2_1_YR       = {{.VkG2_1YR}};
    uint256 internal constant VK_SIZE_X        = {{.VkSizeX}};
    uint256 internal constant VK_SIZE_Y        = {{.VkSizeY}};
    uint256 internal constant VK_GENERATOR_X   = {{.VkGeneratorX}};
    uint256 internal constant VK_COSET_SHIFT   = {{.VkCosetShift}};
    uint256 internal constant VK_NB_PUBLIC_INPUTS = {{.VkNbPublicInputs}};

    // ────────────────────────────────────────────────────────────────────────
    // 类型定义
    // ────────────────────────────────────────────────────────────────────────

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
        if (!_algebraicCheck(proof, ch)) return false;

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
            p2, Pairing.G2Point(VK_G2_0_XI, VK_G2_0_XR, VK_G2_0_YI, VK_G2_0_YR),
            p3, Pairing.G2Point(VK_G2_1_XI, VK_G2_1_XR, VK_G2_1_YI, VK_G2_1_YR)
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
                _g1Compressed(VK_QL_X, VK_QL_Y),
                _g1Compressed(VK_QR_X, VK_QR_Y),
                _g1Compressed(VK_QM_X, VK_QM_Y),
                _g1Compressed(VK_QO_X, VK_QO_Y),
                _g1Compressed(VK_QK_X, VK_QK_Y),
                _g1Compressed(VK_S1_X, VK_S1_Y),
                _g1Compressed(VK_S2_X, VK_S2_Y),
                _g1Compressed(VK_S3_X, VK_S3_Y)
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
        ch.alphaShifted = mulmod(ch.alpha, VK_GENERATOR_X, FR);
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
        uint256 alphaPowT = _modexp(alpha, VK_SIZE_X, FR);
        uint256 vanX      = addmod(alphaPowT, FR - 1, FR); // α^T - 1
        uint256 l0;
        {
            // denom = T · (α - 1)
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
        uint256 alphaPowT  = _modexp(alpha, VK_SIZE_X, FR);
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
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_QL_X, VK_QL_Y), nuPow[5]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_QR_X, VK_QR_Y), nuPow[6]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_QM_X, VK_QM_Y), nuPow[7]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_QO_X, VK_QO_Y), nuPow[8]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_QK_X, VK_QK_Y), nuPow[9]));
        // k=10..12: vk 置换承诺
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_S1_X, VK_S1_Y), nuPow[10]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_S2_X, VK_S2_Y), nuPow[11]));
        c1 = Pairing.ecAdd(c1, Pairing.ecMul(Pairing.G1Point(VK_S3_X, VK_S3_Y), nuPow[12]));
    }

    /// @dev G1 点压缩为 32 字节（gnark-crypto Bytes() 格式）：
    ///      无穷远点：MSB = 0x40（mCompressedInfinity）
    ///      其余点：X 大端序 + MSB 置位（0x80：Y ≤ (Fp-1)/2；0xC0：Y > (Fp-1)/2）
    function _g1Compressed(uint256 x, uint256 y)
        internal
        pure
        returns (bytes32)
    {
        if (x == 0 && y == 0) {
            // 无穷远点（gnark-crypto mCompressedInfinity = 0x40）
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
`

// ─────────────────────────────────────────────────────────────────────────────
// Piano template
// ─────────────────────────────────────────────────────────────────────────────

var pianoVerifierTemplate = `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./Pairing.sol";

/// @title {{.ContractName}}
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
contract {{.ContractName}} {
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
    uint256 internal constant VK_QL_X          = {{.VkQlX}};
    uint256 internal constant VK_QL_Y          = {{.VkQlY}};
    uint256 internal constant VK_QR_X          = {{.VkQrX}};
    uint256 internal constant VK_QR_Y          = {{.VkQrY}};
    uint256 internal constant VK_QM_X          = {{.VkQmX}};
    uint256 internal constant VK_QM_Y          = {{.VkQmY}};
    uint256 internal constant VK_QO_X          = {{.VkQoX}};
    uint256 internal constant VK_QO_Y          = {{.VkQoY}};
    uint256 internal constant VK_QK_X          = {{.VkQkX}};
    uint256 internal constant VK_QK_Y          = {{.VkQkY}};
    uint256 internal constant VK_S1_X          = {{.VkS1X}};
    uint256 internal constant VK_S1_Y          = {{.VkS1Y}};
    uint256 internal constant VK_S2_X          = {{.VkS2X}};
    uint256 internal constant VK_S2_Y          = {{.VkS2Y}};
    uint256 internal constant VK_S3_X          = {{.VkS3X}};
    uint256 internal constant VK_S3_Y          = {{.VkS3Y}};
    uint256 internal constant VK_G2_0_XI       = {{.VkG2_0XI}};
    uint256 internal constant VK_G2_0_XR       = {{.VkG2_0XR}};
    uint256 internal constant VK_G2_0_YI       = {{.VkG2_0YI}};
    uint256 internal constant VK_G2_0_YR       = {{.VkG2_0YR}};
    uint256 internal constant VK_G2_1_XI       = {{.VkG2_1XI}};
    uint256 internal constant VK_G2_1_XR       = {{.VkG2_1XR}};
    uint256 internal constant VK_G2_1_YI       = {{.VkG2_1YI}};
    uint256 internal constant VK_G2_1_YR       = {{.VkG2_1YR}};
    uint256 internal constant VK_G2Y_0_XI      = {{.VkG2Y0XI}};
    uint256 internal constant VK_G2Y_0_XR      = {{.VkG2Y0XR}};
    uint256 internal constant VK_G2Y_0_YI      = {{.VkG2Y0YI}};
    uint256 internal constant VK_G2Y_0_YR      = {{.VkG2Y0YR}};
    uint256 internal constant VK_SIZE_X        = {{.VkSizeX}};
    uint256 internal constant VK_SIZE_Y        = {{.VkSizeY}};
    uint256 internal constant VK_GENERATOR_X   = {{.VkGeneratorX}};
    uint256 internal constant VK_COSET_SHIFT   = {{.VkCosetShift}};
    uint256 internal constant VK_NB_PUBLIC_INPUTS = {{.VkNbPublicInputs}};

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
`
