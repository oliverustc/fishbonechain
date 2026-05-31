// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract HashVerifier {
    function verify(
        bytes memory preImage,
        bytes32 h
    ) public pure returns (uint256 counter) {
        bytes32 currentHash = keccak256(preImage);
        counter = 1;
        
        while (currentHash != h && counter < 1000) { // 设置一个最大迭代次数以防止无限循环
            currentHash = keccak256(abi.encodePacked(currentHash));
            counter++;
        }
        
        require(currentHash == h, "Hash chain does not match target hash");
        return counter;
    }
}
