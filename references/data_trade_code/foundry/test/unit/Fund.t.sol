// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "forge-std/Test.sol";
import "../../src/Fund.sol";
import "../../src/Verify.sol";
import "../../src/HashVerifier.sol";

contract FundTest is Test {
    Fund public fund;
    Verify public verify;
    HashVerifier public hashVerifier;
    address public dataRequester;
    address public dataOwner;
    bytes32 public hashChainEnd;
    uint256 public maxRounds;

    bytes public constant PREIMAGE =
        "A Customizable and Verifiable Masking Data Trading Scheme based on zk-SNARK and Smart Contracts";

    uint256 public constant DATA_REQUESTER_BALANCE = 30 ether;
    uint256 public constant DEPOSIT_AMOUNT = 30 ether;

    // Events to test
    event FundsLocked(address indexed requester, uint256 amount);
    event DepositLocked(address indexed owner, uint256 amount);

    function hashNTimes(
        uint256 n,
        bytes memory preImage
    ) public pure returns (bytes32 hash) {
        hash = keccak256(abi.encodePacked(preImage));
        for (uint256 i = 0; i < n; i++) {
            hash = keccak256(abi.encodePacked(hash));
        }
    }

    function setUp() public {
        dataRequester = address(1);
        dataOwner = address(2);
        maxRounds = 10;
        vm.deal(dataRequester, DATA_REQUESTER_BALANCE);
        vm.deal(dataOwner, DEPOSIT_AMOUNT);

        // Create a simple hash chain for testing
        hashChainEnd = hashNTimes(maxRounds, PREIMAGE);

        vm.startPrank(dataRequester);
        verify = new Verify(dataOwner);
        fund = new Fund(dataOwner, hashChainEnd, maxRounds, address(verify));
        verify.setFundAddress(address(fund));
        hashVerifier = new HashVerifier();
        vm.stopPrank();
    }

    function testConstructor() public view {
        assertEq(fund.dataRequester(), dataRequester);
        assertEq(fund.dataOwner(), dataOwner);
        assertEq(fund.hashChainEnd(), hashChainEnd);
        assertEq(fund.maxRounds(), maxRounds);
        assertEq(fund.verifyContract(), address(verify));
    }

    function testLockFunds() public {
        uint256 amount = 1 ether;

        vm.startPrank(dataRequester);
        fund.lockFunds{value: amount}();
        vm.stopPrank();

        assertEq(address(fund).balance, amount);
        assertEq(fund.lockedFunds(), amount);
    }

    function testLockDeposit() public {
        // First lock funds as dataRequester
        uint256 lockedFundsAmount = 1 ether;
        vm.prank(dataRequester);
        fund.lockFunds{value: lockedFundsAmount}();
        vm.stopPrank();

        // Then lock deposit as dataOwner
        uint256 depositAmount = 0.5 ether;
        vm.prank(dataOwner);
        fund.lockDeposit{value: depositAmount}();
        vm.stopPrank();

        assertEq(fund.deposit(), depositAmount);
        assertEq(address(fund).balance, depositAmount + lockedFundsAmount);
    }

    function test_RevertWhen_LockDepositWithoutFunds() public {
        uint256 depositAmount = 0.5 ether;
        vm.prank(dataOwner);
        
        bool hasError;
        try fund.lockDeposit{value: depositAmount}() {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
    }

    function testClaimFunds() public {
        // Setup initial state
        uint256 lockedFundsAmount = 1 ether;
        uint256 depositAmount = 0.5 ether;

        vm.prank(dataRequester);
        fund.lockFunds{value: lockedFundsAmount}();

        vm.prank(dataOwner);
        fund.lockDeposit{value: depositAmount}();

        // Record balances before claim
        uint256 ownerBalanceBefore = dataOwner.balance;
        uint256 requesterBalanceBefore = dataRequester.balance;

        // Create valid preImage for testing
        // Hash 3 times, 即DO与DR经过了7次交易
        bytes32 hashBytes = hashNTimes(3, PREIMAGE);
        // Convert bytes32 to bytes memory
        bytes memory preImage = abi.encodePacked(hashBytes);
        // times = 7
        uint256 times = hashVerifier.verify(preImage, hashChainEnd);
        // 计算应得的锁定资金量
        uint256 deservedLockedFundsAmount = (fund.lockedFunds() * times) /
            maxRounds;
        // Claim funds
        vm.prank(dataOwner);
        fund.claimFunds(preImage);

        // Verify balances after claim
        assertTrue(dataOwner.balance > ownerBalanceBefore);
        assertEq(
            dataOwner.balance,
            ownerBalanceBefore + deservedLockedFundsAmount + depositAmount
        );
        assertTrue(dataRequester.balance > requesterBalanceBefore);
    }

    function testPunish() public {
        uint256 lockedFundsAmount = 1 ether;
        uint256 depositAmount = 0.5 ether;

        vm.prank(dataRequester);
        fund.lockFunds{value: lockedFundsAmount}();

        vm.prank(dataOwner);
        fund.lockDeposit{value: depositAmount}();

        uint256 requesterBalanceBefore = dataRequester.balance;

        vm.prank(address(verify));
        fund.punish();

        assertEq(
            dataRequester.balance,
            requesterBalanceBefore + lockedFundsAmount + depositAmount
        );
    }

    function test_RevertWhen_PunishNotVerifyContract() public {
        uint256 amount = 1 ether;

        vm.prank(dataRequester);
        fund.lockFunds{value: amount}();

        vm.prank(dataOwner);
        bool hasError;
        try fund.punish() {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
    }

    function testClaimLastPayment() public {
        uint256 lockedFundsAmount = 1 ether;
        uint256 depositAmount = 0.5 ether;

        vm.prank(dataRequester);
        fund.lockFunds{value: lockedFundsAmount}();

        vm.prank(dataOwner);
        fund.lockDeposit{value: depositAmount}();

        uint256 dataOwnerBalanceBefore = dataOwner.balance;

        vm.prank(address(verify));
        fund.claimLastPayment();

        uint256 deserverLockedFundsAmount = fund.lockedFunds() / maxRounds;

        assertEq(
            dataOwner.balance,
            dataOwnerBalanceBefore + deserverLockedFundsAmount
        );
    }

    function test_RevertWhen_ClaimLastPaymentNotVerifyContract() public {
        uint256 amount = 1 ether;

        vm.prank(dataRequester);
        fund.lockFunds{value: amount}();

        vm.prank(dataOwner);
        bool hasError;
        try fund.claimLastPayment() {
            hasError = false;
        } catch {
            hasError = true;
        }
        assertTrue(hasError, "Expected function to revert");
    }
}
