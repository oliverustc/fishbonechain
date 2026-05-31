// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "forge-std/Test.sol";
import "../../src/HashVerifier.sol";

contract HashVerifierTest is Test {
    HashVerifier public verifier;
    bytes constant  PREIMAGE =
        "A Customizable and Verifiable Masking Data Trading Scheme based on zk-SNARK and Smart Contracts";

    function setUp() public {
        verifier = new HashVerifier();
    }

    function testBasicVerification() public view {
        bytes32 hash1 = keccak256(PREIMAGE);
        bytes32 hash2 = keccak256(abi.encodePacked(hash1));
        bytes32 hash3 = keccak256(abi.encodePacked(hash2));

        uint256 count = verifier.verify(PREIMAGE, hash3);
        assertEq(count, 3, "Hash chain length should be 3");
    }

    function testSingleHashVerification() public view {
        bytes32 hash = keccak256(PREIMAGE);

        uint256 count = verifier.verify(PREIMAGE, hash);
        assertEq(count, 1, "Hash chain length should be 1");
    }

    function testLongHashChain() public view {
        bytes32 currentHash = keccak256(PREIMAGE);

        // Create a chain of 20 hashes
        for (uint i = 0; i < 19; i++) {
            currentHash = keccak256(abi.encodePacked(currentHash));
        }

        uint256 count = verifier.verify(PREIMAGE, currentHash);
        assertEq(count, 20, "Hash chain length should be 20");
    }

    function test_RevertWhen_InvalidHash() public view {
        bytes32 wrongHash = bytes32(uint256(1)); // Some random hash

        bool hasError;
        try verifier.verify(PREIMAGE, wrongHash) {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
    }

    function test_RevertWhen_TooLongChain() public view {
        bytes32 currentHash = keccak256(PREIMAGE);

        // Create a chain longer than 1000 hashes
        for (uint i = 0; i < 1001; i++) {
            currentHash = keccak256(abi.encodePacked(currentHash));
        }

        bool hasError;
        try verifier.verify(PREIMAGE, currentHash) {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
    }
}
