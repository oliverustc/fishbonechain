// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Test, console} from "forge-std/Test.sol";
import {stdJson} from "forge-std/StdJson.sol";
import {PianoVerifier} from "../src/PianoVerifier.sol";
import {Pairing} from "../src/Pairing.sol";

/// @title PianoVerifierTest
/// @notice Forge 集成测试：读取 Go 生成的 fixture_piano.json，验证 Piano 证明。
contract PianoVerifierTest is Test {
    using stdJson for string;

    PianoVerifier internal verifier;

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
    // 测试前部署
    // ────────────────────────────────────────────────────────────────────────

    function setUp() public {
        string memory json = vm.readFile("test/fixture_piano.json");

        PianoVerifier.VerifyingKey memory _vk;
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

        verifier = new PianoVerifier(_vk);
    }

    // ────────────────────────────────────────────────────────────────────────
    // 辅助：从 JSON 构建 PianoProof
    // ────────────────────────────────────────────────────────────────────────

    function _loadProof(string memory json) internal view returns (PianoVerifier.PianoProof memory proof) {
        proof.lro0 = _g1(json, ".proof.LRO0");
        proof.lro1 = _g1(json, ".proof.LRO1");
        proof.lro2 = _g1(json, ".proof.LRO2");
        proof.z    = _g1(json, ".proof.Z");
        proof.hx0  = _g1(json, ".proof.Hx0");
        proof.hx1  = _g1(json, ".proof.Hx1");
        proof.hx2  = _g1(json, ".proof.Hx2");
        proof.hy0  = _g1(json, ".proof.Hy0");
        proof.hy1  = _g1(json, ".proof.Hy1");
        proof.hy2  = _g1(json, ".proof.Hy2");
        proof.batchXH  = _g1(json, ".proof.BatchXH");
        proof.cd0  = _g1(json, ".proof.CD0");
        proof.cd1  = _g1(json, ".proof.CD1");
        proof.cd2  = _g1(json, ".proof.CD2");
        proof.cd3  = _g1(json, ".proof.CD3");
        proof.cd4  = _g1(json, ".proof.CD4");
        proof.cd5  = _g1(json, ".proof.CD5");
        proof.cd6  = _g1(json, ".proof.CD6");
        proof.cd7  = _g1(json, ".proof.CD7");
        proof.cd8  = _g1(json, ".proof.CD8");
        proof.cd9  = _g1(json, ".proof.CD9");
        proof.cd10 = _g1(json, ".proof.CD10");
        proof.cd11 = _g1(json, ".proof.CD11");
        proof.cd12 = _g1(json, ".proof.CD12");
        proof.zsH     = _g1(json, ".proof.ZsH");
        proof.zsComVF = _g1(json, ".proof.ZsComVF");
        proof.batchYH = _g1(json, ".proof.BatchYH");

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

        proof.byv0  = _u(json, ".proof.BatchYVal0");
        proof.byv1  = _u(json, ".proof.BatchYVal1");
        proof.byv2  = _u(json, ".proof.BatchYVal2");
        proof.byv3  = _u(json, ".proof.BatchYVal3");
        proof.byv4  = _u(json, ".proof.BatchYVal4");
        proof.byv5  = _u(json, ".proof.BatchYVal5");
        proof.byv6  = _u(json, ".proof.BatchYVal6");
        proof.byv7  = _u(json, ".proof.BatchYVal7");
        proof.byv8  = _u(json, ".proof.BatchYVal8");
        proof.byv9  = _u(json, ".proof.BatchYVal9");
        proof.byv10 = _u(json, ".proof.BatchYVal10");
        proof.byv11 = _u(json, ".proof.BatchYVal11");
        proof.byv12 = _u(json, ".proof.BatchYVal12");
        proof.byv13 = _u(json, ".proof.BatchYVal13");
        proof.byv14 = _u(json, ".proof.BatchYVal14");
    }

    // ────────────────────────────────────────────────────────────────────────
    // 主测试：证明应通过验证
    // ────────────────────────────────────────────────────────────────────────

    function test_Verify() public {
        string memory json = vm.readFile("test/fixture_piano.json");

        PianoVerifier.PianoProof memory proof = _loadProof(json);
        Pairing.G2Point memory tauYBetaG2 = _g2(json, ".tauYBetaG2");

        uint256[] memory pi = new uint256[](0);

        bool ok = verifier.verify(proof, tauYBetaG2, pi);
        assertTrue(ok, "PianoVerifier.verify should return true");
    }

    // ────────────────────────────────────────────────────────────────────────
    // 负例：篡改一个 eval 应导致验证失败
    // ────────────────────────────────────────────────────────────────────────

    function test_VerifyTampered() public {
        string memory json = vm.readFile("test/fixture_piano.json");

        PianoVerifier.PianoProof memory proof = _loadProof(json);

        // 篡改 evalA：+1 mod FR
        uint256 FR = 21888242871839275222246405745257275088548364400416034343698204186575808495617;
        proof.evalA = addmod(proof.evalA, 1, FR);

        Pairing.G2Point memory tauYBetaG2 = _g2(json, ".tauYBetaG2");

        uint256[] memory pi = new uint256[](0);

        bool ok = verifier.verify(proof, tauYBetaG2, pi);
        assertFalse(ok, "tampered proof should fail verification");
    }
}
