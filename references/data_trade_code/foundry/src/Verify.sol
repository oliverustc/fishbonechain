// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;


import "./gnark/groth16RangeHashProofVerifier.sol";
import "./gnark/groth16SubsetHashProofVerifier.sol";
import "./gnark/groth16SubstrHashProofVerifier.sol";
import "./gnark/groth16RootObfuscationProofVerifier20.sol";

interface IFund {
    function punish() external;
    function claimLastPayment() external;
}

contract Verify {

    address public dataRequester;
    address public dataOwner;
    address public fund;
    groth16RangeHashProofVerifier public rangeHashVerifier;
    groth16SubsetHashProofVerifier public subsetHashVerifier;
    groth16SubstrHashProofVerifier public substrHashVerifier;
    groth16RootObfuscationProofVerifier20 public rootObfuscationVerifier;

    constructor(address _dataOwner) {
        dataOwner = _dataOwner;
        dataRequester = msg.sender;
        rangeHashVerifier = new groth16RangeHashProofVerifier();
        subsetHashVerifier = new groth16SubsetHashProofVerifier();
        substrHashVerifier = new groth16SubstrHashProofVerifier();
        rootObfuscationVerifier = new groth16RootObfuscationProofVerifier20();
    }

    modifier onlyDataOwner {
        require(msg.sender == dataOwner, "Only data owner can call this function");
        _;
    }

    modifier onlyDataRequester {
        require(msg.sender == dataRequester, "Only data requester can call this function");
        _;
    }
    
    /**
     * @dev 验证签名是否来自dataOwner
     * @param signature dataOwner的签名
     * @param messageHash 被签名的消息哈希
     * @return 签名验证结果
     */
    function verifySignature(bytes memory signature, bytes32 messageHash) internal view returns (bool) {
        if (signature.length != 65) {
            return false;
        }
        
        bytes32 r;
        bytes32 s;
        uint8 v;
        
        assembly {
            r := mload(add(signature, 32))
            s := mload(add(signature, 64))
            v := byte(0, mload(add(signature, 96)))
        }
        
        // 如果签名使用的是旧版本的以太坊客户端，v可能是27或28
        if (v < 27) {
            v += 27;
        }
        
        // 验证签名
        address signer = ecrecover(messageHash, v, r, s);
        return signer == dataOwner;
    }

    // 设置Fund合约地址
    function setFundAddress(address _fund) external onlyDataRequester {
        require(_fund != address(0), "Invalid fund address");
        require(fund == address(0), "Fund address already set");
        fund = _fund;
    }

    /**
     * @dev 创建用于签名验证的消息哈希
     * @param encodedProof 编码后的proof数据
     * @param encodedInput 编码后的input数据
     * @return 消息哈希
     */
    function _createMessageHash(bytes memory encodedProof, bytes memory encodedInput) internal pure returns (bytes32) {
        bytes32 proofHash = keccak256(encodedProof);
        bytes32 inputHash = keccak256(encodedInput);
        return keccak256(abi.encodePacked(
            "\x19Ethereum Signed Message:\n32",
            keccak256(abi.encodePacked(proofHash, inputHash))
        ));
    }
    
    /**
     * @dev 验证dataOwner对proof和input的签名，并验证proof。如果proof验证失败，则调用fund合约的punish函数
     * @param proof 需要验证的proof
     * @param input 需要验证的input
     * @param signature dataOwner对proof和input的签名
     */
    function punishIfRangeHashProofFailed(
        uint256[8] calldata proof, 
        uint256[3] calldata input, 
        bytes calldata signature
    ) external onlyDataRequester {
        require(fund != address(0), "Fund address not set");
        
        // 创建消息哈希
        bytes32 messageHash = _createMessageHash(abi.encode(proof), abi.encode(input));
        
        // 验证签名
        bytes memory signatureBytes = signature;
        require(verifySignature(signatureBytes, messageHash), "Signature not from dataOwner");
        
        // 使用低级调用来捕获验证失败的情况
        (bool success, ) = address(rangeHashVerifier).staticcall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[3])", proof, input)
        );
        
        // 如果验证失败（调用被revert），则调用fund合约的punish函数
        if (!success) {
            IFund(fund).punish();
        }
    }
    
    /**
     * @dev 验证dataOwner对proof和input的签名，并验证proof。如果proof验证失败，则调用fund合约的punish函数
     * @param proof 需要验证的proof
     * @param input 需要验证的input
     * @param signature dataOwner对proof和input的签名
     */
    function punishIfSubsetHashProofFailed(
        uint256[8] calldata proof, 
        uint256[4] calldata input, 
        bytes calldata signature
    ) external onlyDataRequester {
        require(fund != address(0), "Fund address not set");
        
        // 创建消息哈希
        bytes32 messageHash = _createMessageHash(abi.encode(proof), abi.encode(input));
        
        // 验证签名
        bytes memory signatureBytes = signature;
        require(verifySignature(signatureBytes, messageHash), "Signature not from dataOwner");
        
        // 使用低级调用来捕获验证失败的情况
        (bool success, ) = address(subsetHashVerifier).staticcall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[2])", proof, input)
        );
        
        // 如果验证失败（调用被revert），则调用fund合约的punish函数
        if (!success) {
            IFund(fund).punish();
        }
    }
    
    /**
     * @dev 验证dataOwner对proof和input的签名，并验证proof。如果proof验证失败，则调用fund合约的punish函数
     * @param proof 需要验证的proof
     * @param input 需要验证的input
     * @param signature dataOwner对proof和input的签名
     */
    function punishIfSubstrHashProofFailed(
        uint256[8] calldata proof, 
        uint256[2] calldata input, 
        bytes calldata signature
    ) external onlyDataRequester {
        require(fund != address(0), "Fund address not set");
        
        // 创建消息哈希
        bytes32 messageHash = _createMessageHash(abi.encode(proof), abi.encode(input));
        
        // 验证签名
        bytes memory signatureBytes = signature;
        require(verifySignature(signatureBytes, messageHash), "Signature not from dataOwner");
        
        // 使用低级调用来捕获验证失败的情况
        (bool success, ) = address(substrHashVerifier).staticcall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[2])", proof, input)
        );
        
        // 如果验证失败（调用被revert），则调用fund合约的punish函数
        if (!success) {
            IFund(fund).punish();
        }
    }

    //首先验证signature是不是DataOwner对message||givenHash的签名
    //随后在合约中计算hash，如果givenHash与expectedHash不相等，则执行punish
    function punishIfHashDismatch(bytes calldata message, bytes32 expectedHash, bytes32 givenHash, bytes calldata signature) external onlyDataRequester {
        require(fund != address(0), "Fund address not set");
        
        // 验证签名
        bytes memory signatureBytes = signature;
        bytes32 messageHash = keccak256(abi.encodePacked(message, givenHash));
        require(verifySignature(signatureBytes, messageHash), "Signature not from dataOwner");
        
        // 计算hash
        bytes32 hash = keccak256(message);
        require(hash == expectedHash, "Hash mismatch");
        
        // 如果hash不匹配，则调用fund合约的punish函数
        IFund(fund).punish();
    }

    function claimLastPayment(uint256[8] calldata substrProof, uint256[2] calldata substrInput,uint256[8] calldata rootObfuscationProof, uint256[5] calldata rootObfuscationInput) external onlyDataOwner {
        require(fund != address(0), "Fund address not set");
        // loop 5 time for verify substrProof
        for (uint256 i = 0; i < 3; i++) {
            substrHashVerifier.verifyProof(substrProof, substrInput);
        }
        //loop 10 time for verify rootObfuscationproof
        for (uint256 i = 0; i < 5; i++) {
            rootObfuscationVerifier.verifyProof(rootObfuscationProof, rootObfuscationInput);
        }
        // if all proofs are valid, call fund contract's claimLastPayment function
        IFund(fund).claimLastPayment();
    }

}
