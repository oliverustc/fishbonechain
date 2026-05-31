// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "forge-std/Test.sol";
import "../../src/Verify.sol";
import "../../src/Fund.sol";

contract VerifyTest is Test {
    Verify public verify;
    Fund public fund;
    address public dataRequester;
    address public dataOwner;
    bytes32 public hashChainEnd;
    uint256 public maxRounds;

    function setUp() public {
        dataRequester = address(1);
        dataOwner = address(2);
        maxRounds = 10;

        // Create a simple hash chain for testing
        bytes memory preImage = "test data";
        hashChainEnd = keccak256(abi.encodePacked(preImage));

        vm.startPrank(dataRequester);
        verify = new Verify(dataOwner);
        fund = new Fund(dataOwner, hashChainEnd, maxRounds, address(verify));
        vm.stopPrank();
    }

    function test_RevertWhen_SetFundAddressNotDataRequester() public {
        vm.prank(dataOwner); // Not data requester
        bool hasError;
        try verify.setFundAddress(address(fund)) {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
    }

    function test_RevertWhen_SetFundAddressZeroAddress() public {
        vm.prank(dataRequester);
        bool hasError;
        try verify.setFundAddress(address(0)) {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
    }

    function test_RevertWhen_SetFundAddressTwice() public {
        vm.startPrank(dataRequester);
        verify.setFundAddress(address(fund));
        bool hasError;
        try verify.setFundAddress(address(fund)) {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
        vm.stopPrank();
    }

    // 测试签名验证失败的情况
    function testRevertWhen_InvalidSignature() public {
        // 设置fund地址
        vm.prank(dataRequester);
        verify.setFundAddress(address(fund));

        // 准备proof和input
        uint256[8] memory proof = [
            uint256(1),
            uint256(2),
            uint256(3),
            uint256(4),
            uint256(5),
            uint256(6),
            uint256(7),
            uint256(8)
        ];
        uint256[3] memory input = [uint256(100), uint256(200), uint256(300)];

        // 创建一个无效的签名（使用错误的私钥）
        uint256 wrongPrivateKey = 0xB0B;
        bytes32 messageHash = keccak256(abi.encodePacked("invalid message"));
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(wrongPrivateKey, messageHash);
        bytes memory invalidSignature = abi.encodePacked(r, s, v);

        // 调用验证函数，应该会因为签名验证失败而revert
        vm.prank(dataRequester);
        vm.expectRevert("Signature not from dataOwner");
        verify.punishIfRangeHashProofFailed(proof, input, invalidSignature);
    }
}
