// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./HashVerifier.sol";

contract Fund {
    address public immutable dataRequester;
    address public immutable dataOwner;
    bytes32 public immutable hashChainEnd;
    uint256 public lockedFunds;
    uint256 public deposit;
    uint256 public immutable maxRounds;
    HashVerifier public hashVerifier;
    address public immutable verifyContract;

    uint256 public constant LOCK_TIME = 86400 * 7;

    constructor(
        address _dataOwner, 
        bytes32 _hashChainEnd, 
        uint256 _maxRounds,
        address _verifyContract
    ) {
        require(_verifyContract != address(0), "Invalid verify contract address");
        dataRequester = msg.sender;
        dataOwner = _dataOwner;
        hashChainEnd = _hashChainEnd;
        maxRounds = _maxRounds;
        hashVerifier = new HashVerifier();
        verifyContract = _verifyContract;
    }

    modifier onlyDataRequester {
        require(msg.sender == dataRequester, "Only data requester can call this function");
        _;
    }

    modifier onlyDataOwner {
        require(msg.sender == dataOwner, "Only data owner can call this function");
        _;
    }

    modifier lockedFundsNotZero {
        require(lockedFunds >0, "Locked funds must be greater than 0");
        _;
    }

    modifier depositNotZero {
        require(deposit >0, "Deposit must be greater than 0");
        _;
    }

    modifier onlyVerifyContract {
        require(msg.sender == verifyContract, "Only Verify contract can call this function");
        _;
    }

    function lockFunds() external payable onlyDataRequester {
        require(msg.value >0, "Locked funds must be greater than 0");
        lockedFunds += msg.value;
    }
    
    function lockDeposit() external payable onlyDataOwner {
        require(msg.sender == dataOwner, "Only data owner can lock deposit");
        require(lockedFunds >0, "lock deposit must be called after data requester lock funds");
        require(msg.value >0, "Deposit must be greater than 0");
        deposit += msg.value;
    }

    function claimFunds(bytes memory preImage) external onlyDataOwner lockedFundsNotZero depositNotZero {
        uint256 cycles = hashVerifier.verify(preImage, hashChainEnd);
        require(cycles < maxRounds, "Too many cycles");
        require(cycles > 0, "Failed to verify hash");
        
        uint256 reward = lockedFunds * cycles / maxRounds;
        uint256 ownerAmount = reward + deposit;
        
        // 重置状态变量，防止重入攻击
        lockedFunds = 0;
        deposit = 0;
        
        // 先转账给data owner
        (bool successOwner, ) = payable(dataOwner).call{value: ownerAmount}("");
        require(successOwner, "Transfer to owner failed");
        
        // 将合约中剩余的所有余额转给data requester
        uint256 remainingBalance = address(this).balance;
        (bool successRequester, ) = payable(dataRequester).call{value: remainingBalance}("");
        require(successRequester, "Transfer to requester failed");
    }

    // 将所有余额转给data requester
    function punish() external onlyVerifyContract {
        uint256 remainingBalance = address(this).balance;
        (bool successRequester, ) = payable(dataRequester).call{value: remainingBalance}("");
        require(successRequester, "Transfer to requester failed");
    }

    // 为了防止dataRequester在最后一个循环中拒绝支付报酬，添加一个多支付一个循环资金的功能
    function claimLastPayment() external onlyVerifyContract {
        uint256 amount = lockedFunds / maxRounds;
        (bool successRequester, ) = payable(dataOwner).call{value: amount}("");
        require(successRequester, "Transfer to requester failed");
    }
}
