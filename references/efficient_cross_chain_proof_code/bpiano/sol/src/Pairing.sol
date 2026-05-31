// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title Pairing
/// @notice BN254（alt_bn128）曲线 G1/G2 点运算及多配对检验封装。
///         使用 EIP-196/197 预编译合约：
///           0x06 = ecAdd, 0x07 = ecMul, 0x08 = ecPairing
library Pairing {
    // BN254 基域模数（Fp），用于计算 G1 点的 Y 取反。
    uint256 internal constant FP_MOD =
        21888242871839275222246405745257275088696311157297823662689037894645226208583;

    /// @notice G1 仿射点（坐标均为 Fp 元素）。
    struct G1Point {
        uint256 x;
        uint256 y;
    }

    /// @notice G2 仿射点（坐标为 Fp2 元素）。
    ///         EIP-197 编码顺序：(x_im, x_re, y_im, y_re)。
    struct G2Point {
        uint256 xIm; // X.A1（虚部）
        uint256 xRe; // X.A0（实部）
        uint256 yIm; // Y.A1（虚部）
        uint256 yRe; // Y.A0（实部）
    }

    /// @dev G1 无穷远点（0,0）。
    function infinity() internal pure returns (G1Point memory) {
        return G1Point(0, 0);
    }

    /// @notice G1 点加。使用预编译 0x06。
    function ecAdd(G1Point memory p, G1Point memory q)
        internal
        view
        returns (G1Point memory r)
    {
        uint256[4] memory inp = [p.x, p.y, q.x, q.y];
        bool ok;
        assembly ("memory-safe") {
            ok := staticcall(sub(gas(), 2000), 0x06, inp, 0x80, r, 0x40)
        }
        require(ok, "Pairing: ecAdd failed");
    }

    /// @notice G1 标量乘。使用预编译 0x07。
    function ecMul(G1Point memory p, uint256 s)
        internal
        view
        returns (G1Point memory r)
    {
        uint256[3] memory inp = [p.x, p.y, s];
        bool ok;
        assembly ("memory-safe") {
            ok := staticcall(sub(gas(), 2000), 0x07, inp, 0x60, r, 0x40)
        }
        require(ok, "Pairing: ecMul failed");
    }

    /// @notice G1 点取反（等效于 (x, Fp - y)）。
    function ecNeg(G1Point memory p) internal pure returns (G1Point memory) {
        if (p.x == 0 && p.y == 0) return p;
        return G1Point(p.x, FP_MOD - p.y);
    }

    /// @notice 多配对检验：∏ e(g1s[i], g2s[i]) == 1。
    ///         使用预编译 0x08。
    /// @param g1s G1 点数组
    /// @param g2s G2 点数组（长度须与 g1s 相同）
    /// @return true 当且仅当乘积为 GT 单位元
    function ecPairing(G1Point[] memory g1s, G2Point[] memory g2s)
        internal
        view
        returns (bool)
    {
        require(g1s.length == g2s.length, "Pairing: length mismatch");
        uint256 k = g1s.length;
        uint256[] memory inp = new uint256[](k * 6);
        for (uint256 i = 0; i < k; i++) {
            inp[i * 6 + 0] = g1s[i].x;
            inp[i * 6 + 1] = g1s[i].y;
            inp[i * 6 + 2] = g2s[i].xIm;
            inp[i * 6 + 3] = g2s[i].xRe;
            inp[i * 6 + 4] = g2s[i].yIm;
            inp[i * 6 + 5] = g2s[i].yRe;
        }
        uint256[1] memory out;
        bool ok;
        assembly ("memory-safe") {
            ok := staticcall(
                sub(gas(), 2000),
                0x08,
                add(inp, 0x20),
                mul(k, 0xC0),
                out,
                0x20
            )
        }
        require(ok, "Pairing: ecPairing precompile failed");
        return out[0] == 1;
    }

    /// @notice 固定 4 配对检验（节省动态数组开销）：
    ///         e(p0, g2_0) · e(p1, g2_1) · e(p2, g2_2) · e(p3, g2_3) == 1
    function ecPairing4(
        G1Point memory p0, G2Point memory g2_0,
        G1Point memory p1, G2Point memory g2_1,
        G1Point memory p2, G2Point memory g2_2,
        G1Point memory p3, G2Point memory g2_3
    ) internal view returns (bool) {
        uint256[24] memory inp = [
            p0.x,    p0.y,    g2_0.xIm, g2_0.xRe, g2_0.yIm, g2_0.yRe,
            p1.x,    p1.y,    g2_1.xIm, g2_1.xRe, g2_1.yIm, g2_1.yRe,
            p2.x,    p2.y,    g2_2.xIm, g2_2.xRe, g2_2.yIm, g2_2.yRe,
            p3.x,    p3.y,    g2_3.xIm, g2_3.xRe, g2_3.yIm, g2_3.yRe
        ];
        uint256[1] memory out;
        bool ok;
        assembly ("memory-safe") {
            ok := staticcall(
                sub(gas(), 2000),
                0x08,
                inp,
                0x300,  // 4 × 192 = 768 = 0x300 字节
                out,
                0x20
            )
        }
        require(ok, "Pairing: ecPairing4 precompile failed");
        return out[0] == 1;
    }
}
