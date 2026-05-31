// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "forge-std/Script.sol";
import "forge-std/console.sol";
import "../src/Fund.sol";
import "../src/Verify.sol";

contract DeployContracts is Script {
    function run() external {
        // 获取部署者私钥（本地模拟时使用默认值）
        uint256 deployerPrivateKey;
        try vm.envUint("PRIVATE_KEY") returns (uint256 key) {
            deployerPrivateKey = key;
        } catch {
            // 本地测试时使用的默认私钥（这是一个示例私钥，仅用于测试）
            deployerPrivateKey = 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80;
            console.log("Using default private key for local testing");
        }
        
        // 设置部署者地址
        address deployerAddress = vm.addr(deployerPrivateKey);
        console.log("Deployer address:", deployerAddress);
        
        // 在zkSync环境中模拟资金
        // 这一步在模拟模式下为部署者提供足够的ETH
        vm.deal(deployerAddress, 100 ether);
        console.log("Funded deployer with 100 ETH for simulation");
        
        // 设置数据所有者地址 (可以从环境变量获取或直接设置)
        address dataOwnerAddress;
        try vm.envAddress("DATA_OWNER") returns (address owner) {
            dataOwnerAddress = owner;
            // 为数据所有者也提供一些资金
            if (dataOwnerAddress != deployerAddress) {
                vm.deal(dataOwnerAddress, 100 ether);
                console.log("Funded data owner with 100 ETH for simulation");
            }
        } catch {
            // 默认使用部署者地址作为数据所有者地址
            dataOwnerAddress = deployerAddress;
            console.log("Using deployer as data owner for local testing");
        }
        console.log("Data owner address:", dataOwnerAddress);
        
        // 模拟Gas消耗而不实际部署（可选）
        bool simulateOnly = vm.envOr("SIMULATE_ONLY", false);
        if (simulateOnly) {
            console.log("Running in simulation mode only - no actual deployment");
        }
        
        // 开始广播交易
        vm.startBroadcast(deployerPrivateKey);
        
        // 1. 首先部署Verify合约
        console.log("Deploying Verify contract...");
        Verify verifyContract = new Verify(dataOwnerAddress);
        console.log("Verify contract deployed at:", address(verifyContract));
        
        // 2. 设置哈希链终点和最大轮数 (这些值可以根据需要调整)
        bytes32 hashChainEnd = 0x0000000000000000000000000000000000000000000000000000000000000001; // 示例值，实际使用时应替换
        uint256 maxRounds = 100; // 示例值，实际使用时应替换
        
        // 3. 部署Fund合约
        console.log("Deploying Fund contract...");
        Fund fundContract = new Fund(
            dataOwnerAddress,
            hashChainEnd,
            maxRounds,
            address(verifyContract)
        );
        console.log("Fund contract deployed at:", address(fundContract));
        
        // 4. 在Verify合约中设置Fund合约地址
        console.log("Setting Fund address in Verify contract...");
        verifyContract.setFundAddress(address(fundContract));
        console.log("Fund address set in Verify contract");
        
        // 结束广播
        vm.stopBroadcast();
        
        // 输出部署信息摘要
        console.log("\nDeployment Summary:");
        console.log("------------------");
        console.log("Verify Contract: ", address(verifyContract));
        console.log("Fund Contract:   ", address(fundContract));
        console.log("Data Owner:      ", dataOwnerAddress);
        console.log("Data Requester:  ", deployerAddress);
        console.log("------------------");
    }
}
