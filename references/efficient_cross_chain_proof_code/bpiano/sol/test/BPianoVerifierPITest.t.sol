// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Test, console} from "forge-std/Test.sol";
import {stdJson} from "forge-std/StdJson.sol";
import {BPianoVerifier} from "../src/BPianoVerifier.sol";
import {Pairing} from "../src/Pairing.sol";

/// @title BPianoVerifierPITest
/// @notice Forge 集成测试：读取含公开输入的 fixture_pi.json，验证 BPiano 证明。
contract BPianoVerifierPITest is Test {
    using stdJson for string;

    BPianoVerifier internal verifier;

    // ────────────────────────────────────────────────────────────────────────
    // 辅助：解析 JSON 中的 0x-前缀十六进制字符串为 uint256
    // ────────────────────────────────────────────────────────────────────────

    function _u(string memory json, string memory key) internal view returns (uint256) {
        return vm.parseUint(json.readString(key));
    }

    function _g1(string memory json, string memory prefix) internal view returns (Pairing.G1Point memory) {
        string[] memory arr = json.readStringArray(prefix);
        return Pairing.G1Point(vm.parseUint(arr[0]), vm.parseUint(arr[1]));
    }

    function _g2(string memory json, string memory prefix) internal view returns (Pairing.G2Point memory) {
        string[] memory arr = json.readStringArray(prefix);
        return Pairing.G2Point(
            vm.parseUint(arr[0]), // xIm
            vm.parseUint(arr[1]), // xRe
            vm.parseUint(arr[2]), // yIm
            vm.parseUint(arr[3])  // yRe
        );
    }

    // ────────────────────────────────────────────────────────────────────────
    // 测试前部署（含公开输入的电路 VK）
    // ────────────────────────────────────────────────────────────────────────

    function setUp() public {
        string memory json = vm.readFile("test/fixture_pi.json");

        BPianoVerifier.VerifyingKey memory _vk;
        _vk.ql  = _g1(json, ".vk.Ql");
        _vk.qr  = _g1(json, ".vk.Qr");
        _vk.qm  = _g1(json, ".vk.Qm");
        _vk.qo  = _g1(json, ".vk.Qo");
        _vk.qk  = _g1(json, ".vk.Qk");
        _vk.s1  = _g1(json, ".vk.S1");
        _vk.s2  = _g1(json, ".vk.S2");
        _vk.s3  = _g1(json, ".vk.S3");
        _vk.g2_0  = _g2(json, ".vk.G2_0");
        _vk.g2_1  = _g2(json, ".vk.G2_1");
        _vk.g2y_0 = _g2(json, ".vk.G2Y_0");
        _vk.sizeX       = _u(json, ".vk.SizeX");
        _vk.sizeY       = _u(json, ".vk.SizeY");
        _vk.generatorX  = _u(json, ".vk.GeneratorX");
        _vk.cosetShift  = _u(json, ".vk.CosetShift");
        _vk.nbPublicInputs = _u(json, ".vk.NbPublicInputs");

        verifier = new BPianoVerifier(_vk);
    }

    // ────────────────────────────────────────────────────────────────────────
    // 主测试：含公开输入的证明应通过验证
    // ────────────────────────────────────────────────────────────────────────

    function test_VerifyWithPI() public {
        string memory json = vm.readFile("test/fixture_pi.json");

        BPianoVerifier.CompressedProof memory proof;
        proof.lro0 = _g1(json, ".proof.LRO0");
        proof.lro1 = _g1(json, ".proof.LRO1");
        proof.lro2 = _g1(json, ".proof.LRO2");
        proof.z    = _g1(json, ".proof.Z");
        proof.hx0  = _g1(json, ".proof.Hx0");
        proof.hx1  = _g1(json, ".proof.Hx1");
        proof.hx2  = _g1(json, ".proof.Hx2");
        proof.comQX      = _g1(json, ".proof.ComQX");
        proof.comVFAlpha = _g1(json, ".proof.ComVFAlpha");
        proof.comVFZS    = _g1(json, ".proof.ComVFZS");
        proof.comGY      = _g1(json, ".proof.ComGY");
        proof.pi1AggH    = _g1(json, ".proof.Pi1AggH");

        proof.evalA  = _u(json, ".proof.EvalA");
        proof.evalB  = _u(json, ".proof.EvalB");
        proof.evalO  = _u(json, ".proof.EvalO");
        proof.evalZ  = _u(json, ".proof.EvalZ");
        proof.evalZS = _u(json, ".proof.EvalZS");
        proof.evalHx = _u(json, ".proof.EvalHx");
        proof.evalHy = _u(json, ".proof.EvalHy");
        proof.evalQl = _u(json, ".proof.EvalQl");
        proof.evalQr = _u(json, ".proof.EvalQr");
        proof.evalQm = _u(json, ".proof.EvalQm");
        proof.evalQo = _u(json, ".proof.EvalQo");
        proof.evalQk = _u(json, ".proof.EvalQk");
        proof.evalS1 = _u(json, ".proof.EvalS1");
        proof.evalS2 = _u(json, ".proof.EvalS2");
        proof.evalS3 = _u(json, ".proof.EvalS3");

        Pairing.G2Point memory zTG2      = _g2(json, ".zTG2");
        Pairing.G2Point memory tauYBetaG2 = _g2(json, ".tauYBetaG2");

        // 从 fixture 读取公开输入（M × nbPublicInputs 个 Fr 值）
        string[] memory piStrs = json.readStringArray(".publicInputs");
        uint256[] memory pi = new uint256[](piStrs.length);
        for (uint256 i = 0; i < piStrs.length; i++) {
            pi[i] = vm.parseUint(piStrs[i]);
        }

        bool ok = verifier.verify(proof, zTG2, tauYBetaG2, pi);
        assertTrue(ok, "BPianoVerifier.verify (with PI) should return true");
    }

    // ────────────────────────────────────────────────────────────────────────
    // 负例：篡改公开输入应导致验证失败
    // ────────────────────────────────────────────────────────────────────────

    function test_VerifyWrongPI() public {
        string memory json = vm.readFile("test/fixture_pi.json");

        BPianoVerifier.CompressedProof memory proof;
        proof.lro0 = _g1(json, ".proof.LRO0");
        proof.lro1 = _g1(json, ".proof.LRO1");
        proof.lro2 = _g1(json, ".proof.LRO2");
        proof.z    = _g1(json, ".proof.Z");
        proof.hx0  = _g1(json, ".proof.Hx0");
        proof.hx1  = _g1(json, ".proof.Hx1");
        proof.hx2  = _g1(json, ".proof.Hx2");
        proof.comQX      = _g1(json, ".proof.ComQX");
        proof.comVFAlpha = _g1(json, ".proof.ComVFAlpha");
        proof.comVFZS    = _g1(json, ".proof.ComVFZS");
        proof.comGY      = _g1(json, ".proof.ComGY");
        proof.pi1AggH    = _g1(json, ".proof.Pi1AggH");

        proof.evalA  = _u(json, ".proof.EvalA");
        proof.evalB  = _u(json, ".proof.EvalB");
        proof.evalO  = _u(json, ".proof.EvalO");
        proof.evalZ  = _u(json, ".proof.EvalZ");
        proof.evalZS = _u(json, ".proof.EvalZS");
        proof.evalHx = _u(json, ".proof.EvalHx");
        proof.evalHy = _u(json, ".proof.EvalHy");
        proof.evalQl = _u(json, ".proof.EvalQl");
        proof.evalQr = _u(json, ".proof.EvalQr");
        proof.evalQm = _u(json, ".proof.EvalQm");
        proof.evalQo = _u(json, ".proof.EvalQo");
        proof.evalQk = _u(json, ".proof.EvalQk");
        proof.evalS1 = _u(json, ".proof.EvalS1");
        proof.evalS2 = _u(json, ".proof.EvalS2");
        proof.evalS3 = _u(json, ".proof.EvalS3");

        Pairing.G2Point memory zTG2      = _g2(json, ".zTG2");
        Pairing.G2Point memory tauYBetaG2 = _g2(json, ".tauYBetaG2");

        // 篡改公开输入：将 42 改为 43
        uint256[] memory pi = new uint256[](2); // M=2 instances, 1 PI each
        pi[0] = 43;
        pi[1] = 43;

        bool ok = verifier.verify(proof, zTG2, tauYBetaG2, pi);
        assertFalse(ok, "wrong public input should fail verification");
    }
}
