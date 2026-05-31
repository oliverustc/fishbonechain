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
    uint256 private dataOwnerPrivateKey;

    function setUp() public {
        dataOwnerPrivateKey = 0xA11CE; // 测试私钥
        dataRequester = address(1);
        dataOwner = vm.addr(dataOwnerPrivateKey); // 从私钥生成地址
        maxRounds = 10;

        // Create a simple hash chain for testing
        bytes memory preImage = "test data";
        hashChainEnd = keccak256(abi.encodePacked(preImage));

        vm.startPrank(dataRequester);
        verify = new Verify(dataOwner);
        fund = new Fund(dataOwner, hashChainEnd, maxRounds, address(verify));
        vm.stopPrank();
    }

    function testConstructor() public view {
        assertEq(verify.dataRequester(), dataRequester);
        assertEq(verify.dataOwner(), dataOwner);
        assertEq(verify.fund(), address(0)); // Fund address not set yet
    }

    function testSetFundAddress() public {
        vm.prank(dataRequester);
        verify.setFundAddress(address(fund));
        assertEq(verify.fund(), address(fund));
    }

    // 创建dataOwner对proof和input的签名
    function _signProofAndInput(
        bytes memory encodedProof,
        bytes memory encodedInput
    ) internal view returns (bytes memory) {
        bytes32 proofHash = keccak256(encodedProof);
        bytes32 inputHash = keccak256(encodedInput);
        bytes32 messageHash = keccak256(
            abi.encodePacked(
                "\x19Ethereum Signed Message:\n32",
                keccak256(abi.encodePacked(proofHash, inputHash))
            )
        );

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(
            dataOwnerPrivateKey,
            messageHash
        );
        return abi.encodePacked(r, s, v);
    }

    // 测试RangeHash验证失败时的惩罚功能
    function testPunishIfRangeHashProofFailed() public {
        // 设置fund地址
        vm.prank(dataRequester);
        verify.setFundAddress(address(fund));

        // 锁定一些资金用于测试
        uint256 amount = 1 ether;
        vm.deal(dataRequester, amount);
        vm.prank(dataRequester);
        fund.lockFunds{value: amount}();

        // 准备一个无效的proof（会导致验证失败）
        uint256[8] memory invalidProof = [17303988958211003921576352319910417885293512740320415088209876206006005386059,11921422916342589820797598555304922328195233557053400435638357088492211376721,14099979157727358389952825178227396115396464229872991548440417952142311781254,12243782710655077262934785058425925917334943076288567375732214210135398564402,8451336284388570249651508983101400662448435027820762180462639808887102354286,17894476320570208052339224819477647520482513764802590287682783155257011428038,7681428772969040583374937724652797774925340396804034948044221604579005405672,4516517951814133696682391694472537632197345159275171943982163200750250046597];
        uint256[3] memory input =[1985387465309357632163097509124894878492351398003011757474506935035132907134,0,21888242871839275222246405745257275088548364400416034343698204186575808495616];

        // 创建dataOwner的签名
        bytes memory signature = _signProofAndInput(
            abi.encode(invalidProof),
            abi.encode(input)
        );

        // 记录调用前的余额
        uint256 requesterBalanceBefore = dataRequester.balance;
        uint256 fundBalanceBefore = address(fund).balance;

        // 调用验证函数
        vm.prank(dataRequester);
        verify.punishIfRangeHashProofFailed(invalidProof, input, signature);

        // 验证是否执行了惩罚（资金转移）
        assertEq(dataRequester.balance, requesterBalanceBefore + amount);
        assertEq(address(fund).balance, fundBalanceBefore - amount);
    }

    // 测试SubsetHash验证失败时的惩罚功能
    function testPunishIfSubsetHashProofFailed() public {
        // 设置fund地址
        vm.prank(dataRequester);
        verify.setFundAddress(address(fund));

        // 锁定一些资金用于测试
        uint256 amount = 1 ether;
        vm.deal(dataRequester, amount);
        vm.prank(dataRequester);
        fund.lockFunds{value: amount}();

        // 准备一个无效的proof（会导致验证失败）
        uint256[8] memory invalidProof = [12642032552173898555259825610957488182974679171248101837351600940580117877207,15214088366884050357280564378265731951691358216597734261495724667326993828949,2464063733160686903752212814352732912031745494762070396005298244516331159105,20106506609132800702880314104145091908893777749441597323020435200215315854352,3786458671254773770146654248988328380607453781506812710649312245022242559539,6451459886760631606446830433881222706802594535958474201944093349635984109207,7123670869933541678922970266057378251948705717007186265192645839928768068344,11129314914848787018657867562542490824737136572739223528604499473263073322718];
        uint256[4] memory input = [29143352915433715317876415032364880146004171956268037183589559265813656434116,2170125137595863186744376756629067392557026423466148081946082119973435201294,14861083672216646279787743994426912506610339397370315227145794140406059573145,7334724107407190911650359253875604464530406538427658378041400879840862328083];

        // 创建dataOwner的签名
        bytes memory signature = _signProofAndInput(
            abi.encode(invalidProof),
            abi.encode(input)
        );

        // 记录调用前的余额
        uint256 requesterBalanceBefore = dataRequester.balance;
        uint256 fundBalanceBefore = address(fund).balance;

        // 调用验证函数
        vm.prank(dataRequester);
        verify.punishIfSubsetHashProofFailed(invalidProof, input, signature);

        // 验证是否执行了惩罚（资金转移）
        assertEq(dataRequester.balance, requesterBalanceBefore + amount);
        assertEq(address(fund).balance, fundBalanceBefore - amount);
    }

    // 测试SubstrHash验证失败时的惩罚功能
    function testPunishIfSubstrHashProofFailed() public {
        // 设置fund地址
        vm.prank(dataRequester);
        verify.setFundAddress(address(fund));

        // 锁定一些资金用于测试
        uint256 amount = 1 ether;
        vm.deal(dataRequester, amount);
        vm.prank(dataRequester);
        fund.lockFunds{value: amount}();

        // 准备一个无效的proof（会导致验证失败）
        uint256[8] memory invalidProof = [
            2739962893857298382981906542727031917086738077594524007352819533990548017766,
            2802075598470692757342945305177402426743254434294517699954934786670118105435,
            1639400192422477276875610321111113073481512284566023081338159062138888975773,
            5103825508042408059395908715358446425911217289621265126831824895071628395481,
            13603195513854685598070842278189053670112214557751106108424922239173389441937,
            7125571172636538660311624731643913091427013885314799025372896188977024648575,
            4596228899325014060180586597087861293375810969661095350413355236676961297498,
            13041581398630039776450600112653874923355426335469909250205182013922620985242
        ];
        uint256[2] memory input = [
            19607449305271047082156535951129267078566136275054100637781266286544672664190,
            521678708696602054185546
        ];

        // 创建dataOwner的签名
        bytes memory signature = _signProofAndInput(
            abi.encode(invalidProof),
            abi.encode(input)
        );

        // 记录调用前的余额
        uint256 requesterBalanceBefore = dataRequester.balance;
        uint256 fundBalanceBefore = address(fund).balance;

        // 使用try-catch来处理可能的gas耗尽问题
        vm.prank(dataRequester);
        
        // 我们在foundry.toml中设置gas限制，而不是在代码中设置
        verify.punishIfSubstrHashProofFailed(invalidProof, input, signature);

        // 验证是否执行了惩罚（资金转移）
        assertEq(dataRequester.balance, requesterBalanceBefore + amount);
        assertEq(address(fund).balance, fundBalanceBefore - amount);
    }

    // 创建dataOwner对message和givenHash的签名
    function _signMessageAndHash(
        bytes memory message,
        bytes32 givenHash
    ) internal view returns (bytes memory) {
        bytes32 messageHash = keccak256(abi.encodePacked(message, givenHash));

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(
            dataOwnerPrivateKey,
            messageHash
        );
        return abi.encodePacked(r, s, v);
    }

    // 测试Hash不匹配时的惩罚功能
    function testPunishIfHashDismatch() public {
        // 设置fund地址
        vm.prank(dataRequester);
        verify.setFundAddress(address(fund));

        // 锁定一些资金用于测试
        uint256 amount = 1 ether;
        vm.deal(dataRequester, amount);
        vm.prank(dataRequester);
        fund.lockFunds{value: amount}();

        // 准备测试数据
        bytes memory message = bytes("test data");
        bytes32 expectedHash = keccak256(message);
        bytes32 givenHash = bytes32(uint256(expectedHash) + 1); // 故意制造不匹配的哈希

        // 创建dataOwner的签名
        bytes memory signature = _signMessageAndHash(
            message,
            givenHash
        );

        // 记录调用前的余额
        uint256 requesterBalanceBefore = dataRequester.balance;
        uint256 fundBalanceBefore = address(fund).balance;

        // 调用验证函数
        vm.prank(dataRequester);
        verify.punishIfHashDismatch(message, expectedHash, givenHash, signature);

        // 验证是否执行了惩罚（资金转移）
        assertEq(dataRequester.balance, requesterBalanceBefore + amount);
        assertEq(address(fund).balance, fundBalanceBefore - amount);
    }

    // 测试claimLastPayment
    function testClaimLastPayment() public {
        // 设置fund地址
        vm.prank(dataRequester);
        verify.setFundAddress(address(fund));

        // 锁定一些资金用于测试
        uint256 amount = 1 ether;
        vm.deal(dataRequester, amount);
        vm.prank(dataRequester);
        fund.lockFunds{value: amount}();

        uint256[8] memory substrHashProof = [
            2739962893857298382981906542727031917086738077594524007352819533990548017766,
            2802075598470692757342945305177402426743254434294517699954934786670118105435,
            1639400192422477276875610321111113073481512284566023081338159062138888975773,
            5103825508042408059395908715358446425911217289621265126831824895071628395481,
            13603195513854685598070842278189053670112214557751106108424922239173389441937,
            7125571172636538660311624731643913091427013885314799025372896188977024648575,
            4596228899325014060180586597087861293375810969661095350413355236676961297498,
            13041581398630039776450600112653874923355426335469909250205182013922620985242
        ];
        uint256[2] memory substrHashPublicInput = [
            19607449305271047082156535951129267078566136275054100637780266286544672664190,
            521678708696602054185546
        ];
        uint256[8] memory rootObfuscationProof = [3886728562343285252465333780050909467254459889200163746255389902323552326009,7634092753286395378452268316191908565358093356188361243062588514988152342899,9074734417342102122552689323625275184977329671505275513284301838938856416055,3898529735917692260090583858731549523254737027518400883145591583279770145527,9101454723280324976325006965896560887062947834200512050948072027545839458063,14492598771163985959207978872489342675275311231016167713304231139868333070897,6442118708086483649900079357114428451099257831752913830033456214352830668220,4196893750378155435260729382331990531828434365074768323945410088456056803910];
        uint256[5] memory rootObfuscationPublicInput = [360847722892838854937367416453687685997692722372668365474491598081274712073,14337002091934406297219805755378561614020327001252386447837825842627067378103,4321155375412260467747938008195780221265858042192771120371386415150764257765,16980001009556294294591913623020758164992469373538598005346398664715163902167,10911610962791271491305441214243110103759884275615817801045221228438971341556];
        // 调用claimLastPayment
        vm.prank(dataOwner);


        verify.claimLastPayment(substrHashProof,substrHashPublicInput,rootObfuscationProof,rootObfuscationPublicInput);

    }
}
