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
        maxRounds = 11;
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

    function tryClaimFunds(uint256 time) public returns (uint256 gasUsed) {
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
        bytes32 hashBytes = hashNTimes(time, PREIMAGE);
        bytes memory preImage = abi.encodePacked(hashBytes);
        uint256 verifiedtimes = hashVerifier.verify(preImage, hashChainEnd);
        uint256 deservedLockedFundsAmount = (fund.lockedFunds() * verifiedtimes) /
            maxRounds;

        // Measure gas consumption
        uint256 gasStart = gasleft();
        vm.prank(dataOwner);
        fund.claimFunds(preImage);
        gasUsed = gasStart - gasleft();

        // Verify balances after claim
        assertTrue(dataOwner.balance > ownerBalanceBefore);
        assertEq(
            dataOwner.balance,
            ownerBalanceBefore + deservedLockedFundsAmount + depositAmount
        );
        assertTrue(dataRequester.balance > requesterBalanceBefore);
        console.log("dataOwner balance get ", dataOwner.balance);
    }

    function testClaimFunds() public {
        console.log("Testing gas consumption for different times:");
        console.log("Time\tGas Used");
        // time from 1 to maxRounds
        for (uint256 i = 1; i < maxRounds; i++) {
            uint256 gasUsed = tryClaimFunds(i);
            console.log("%d\t%d", i, gasUsed);
        }
    }
}
