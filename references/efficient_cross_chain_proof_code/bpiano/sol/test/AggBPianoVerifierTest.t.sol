// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Test, console} from "forge-std/Test.sol";
import {stdJson} from "forge-std/StdJson.sol";
import {AggBPianoVerifier} from "../src/AggBPianoVerifier.sol";
import {Pairing} from "../src/Pairing.sol";

/// @title AggBPianoVerifierTest
/// @notice Forge 集成测试：读取 Go 生成的 fixture_agg_k{K}.json，验证聚合 BPiano 证明。
/// Fixture 格式：{"proofs": [...], "comQXTotal": [...], "pi1Total": [...], ...}
contract AggBPianoVerifierTest is Test {
    using stdJson for string;

    AggBPianoVerifier internal verifier;

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
    // 辅助：从 JSON 数组中读取第 idx 个 CompressedProof
    // 路径格式：.proofs[idx].FieldName
    // ────────────────────────────────────────────────────────────────────────

    function _readProofFromArray(string memory json, uint256 idx)
        internal
        view
        returns (AggBPianoVerifier.CompressedProof memory proof)
    {
        string memory p = string.concat(".proofs[", vm.toString(idx), "]");
        proof.lro0       = _g1(json, string.concat(p, ".LRO0"));
        proof.lro1       = _g1(json, string.concat(p, ".LRO1"));
        proof.lro2       = _g1(json, string.concat(p, ".LRO2"));
        proof.z          = _g1(json, string.concat(p, ".Z"));
        proof.hx0        = _g1(json, string.concat(p, ".Hx0"));
        proof.hx1        = _g1(json, string.concat(p, ".Hx1"));
        proof.hx2        = _g1(json, string.concat(p, ".Hx2"));
        proof.comQX      = _g1(json, string.concat(p, ".ComQX"));
        proof.comVFAlpha = _g1(json, string.concat(p, ".ComVFAlpha"));
        proof.comVFZS    = _g1(json, string.concat(p, ".ComVFZS"));
        proof.comGY      = _g1(json, string.concat(p, ".ComGY"));
        proof.pi1AggH    = _g1(json, string.concat(p, ".Pi1AggH"));

        proof.evalA  = _u(json, string.concat(p, ".EvalA"));
        proof.evalB  = _u(json, string.concat(p, ".EvalB"));
        proof.evalO  = _u(json, string.concat(p, ".EvalO"));
        proof.evalZ  = _u(json, string.concat(p, ".EvalZ"));
        proof.evalZS = _u(json, string.concat(p, ".EvalZS"));
        proof.evalHx = _u(json, string.concat(p, ".EvalHx"));
        proof.evalHy = _u(json, string.concat(p, ".EvalHy"));
        proof.evalQl = _u(json, string.concat(p, ".EvalQl"));
        proof.evalQr = _u(json, string.concat(p, ".EvalQr"));
        proof.evalQm = _u(json, string.concat(p, ".EvalQm"));
        proof.evalQo = _u(json, string.concat(p, ".EvalQo"));
        proof.evalQk = _u(json, string.concat(p, ".EvalQk"));
        proof.evalS1 = _u(json, string.concat(p, ".EvalS1"));
        proof.evalS2 = _u(json, string.concat(p, ".EvalS2"));
        proof.evalS3 = _u(json, string.concat(p, ".EvalS3"));
    }

    // ────────────────────────────────────────────────────────────────────────
    // 辅助：从 JSON 读取 VK 并部署合约
    // ────────────────────────────────────────────────────────────────────────

    function _deployFromFixture(string memory json) internal {
        AggBPianoVerifier.VerifyingKey memory _vk;
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
        _vk.sizeX          = _u(json, ".vk.SizeX");
        _vk.sizeY          = _u(json, ".vk.SizeY");
        _vk.generatorX     = _u(json, ".vk.GeneratorX");
        _vk.cosetShift     = _u(json, ".vk.CosetShift");
        _vk.nbPublicInputs = _u(json, ".vk.NbPublicInputs");
        verifier = new AggBPianoVerifier(_vk);
    }

    // ────────────────────────────────────────────────────────────────────────
    // setUp：使用 K=2 fixture 部署（所有 fixture 共享同一 VK）
    // ────────────────────────────────────────────────────────────────────────

    function setUp() public {
        string memory json = vm.readFile("test/fixture_agg_k2.json");
        _deployFromFixture(json);
    }

    // ────────────────────────────────────────────────────────────────────────
    // 通用测试驱动：加载 fixture，读取 K 个证明，调用 verifier.verify，记录 Gas
    // ────────────────────────────────────────────────────────────────────────

    function _runAggVerify(string memory fixturePath, uint256 K) internal {
        string memory json = vm.readFile(fixturePath);

        AggBPianoVerifier.CompressedProof[] memory proofs =
            new AggBPianoVerifier.CompressedProof[](K);
        for (uint256 i = 0; i < K; i++) {
            proofs[i] = _readProofFromArray(json, i);
        }

        Pairing.G1Point memory comQXTotal = _g1(json, ".comQXTotal");
        Pairing.G1Point memory pi1Total   = _g1(json, ".pi1Total");
        Pairing.G2Point memory zTG2       = _g2(json, ".zTG2");
        Pairing.G2Point memory tauYBetaG2 = _g2(json, ".tauYBetaG2");
        uint256[] memory pi = new uint256[](0);

        uint256 g0 = gasleft();
        bool ok = verifier.verify(proofs, comQXTotal, pi1Total, zTG2, tauYBetaG2, pi);
        uint256 gasUsed = g0 - gasleft();
        emit log_named_uint(string.concat("AggK", vm.toString(K), " gas"), gasUsed);
        assertTrue(ok, string.concat("AggBPianoVerifier K=", vm.toString(K), " should return true"));
    }

    // ────────────────────────────────────────────────────────────────────────
    // 测试用例：K = 2, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100
    // ────────────────────────────────────────────────────────────────────────

    function testVerify_AggK2()   public { _runAggVerify("test/fixture_agg_k2.json",   2);   }
    function testVerify_AggK10()  public { _runAggVerify("test/fixture_agg_k10.json",  10);  }
    function testVerify_AggK20()  public { _runAggVerify("test/fixture_agg_k20.json",  20);  }
    function testVerify_AggK30()  public { _runAggVerify("test/fixture_agg_k30.json",  30);  }
    function testVerify_AggK40()  public { _runAggVerify("test/fixture_agg_k40.json",  40);  }
    function testVerify_AggK50()  public { _runAggVerify("test/fixture_agg_k50.json",  50);  }
    function testVerify_AggK60()  public { _runAggVerify("test/fixture_agg_k60.json",  60);  }
    function testVerify_AggK70()  public { _runAggVerify("test/fixture_agg_k70.json",  70);  }
    function testVerify_AggK80()  public { _runAggVerify("test/fixture_agg_k80.json",  80);  }
    function testVerify_AggK90()  public { _runAggVerify("test/fixture_agg_k90.json",  90);  }
    function testVerify_AggK100() public { _runAggVerify("test/fixture_agg_k100.json", 100); }
}
