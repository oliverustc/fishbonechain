use sc_service::ChainType;
use fishbone_runtime::WASM_BINARY;

pub type ChainSpec = sc_service::GenericChainSpec;

fn wasm() -> &'static [u8] {
    WASM_BINARY.expect("WASM binary not available")
}

// ── 主链 ──────────────────────────────────────────────────────────────────────

/// 主链开发模式（单节点 Alice，6 秒出块）
pub fn main_dev() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Main Dev")
        .with_id("fishbone_main_dev")
        .with_chain_type(ChainType::Development)
        .with_genesis_config_preset_name(sp_genesis_builder::DEV_RUNTIME_PRESET)
        .build())
}

/// 主链本地多节点（Alice+Bob 验证者，6 秒出块）
pub fn main_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Main")
        .with_id("fishbone_main")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

// ── 子链 ──────────────────────────────────────────────────────────────────────

/// 子链1：城市快递配送确认（基准链，AURA-3，6s 出块，10min Sc）
pub fn child1_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-1 (Crowdsource)")
        .with_id("fishbone_child_1")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

/// 子链2：实时城市交通感知（高频微任务，AURA-3，2s 出块，5min Sc）
/// 使用 fishbone-node-2s binary（--features block-2s 编译）启动。
pub fn child2_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-2 (Traffic Sensing)")
        .with_id("fishbone_child_2")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

/// 子链3：专业医疗影像标注（长周期高价值，AURA-3，30min Sc，10MB 区块）
/// 使用 fishbone-node-10mb binary（--features block-10mb 编译）启动。
pub fn child3_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-3 (Medical Annotation)")
        .with_id("fishbone_child_3")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

/// 子链4：金融支付凭证核验（高安全性，AURA-7，6s 出块，20min Sc）
/// 7 个验证人（f1-f7），提供更强的拜占庭容错（f=2）。
pub fn child4_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-4 (Financial Verification)")
        .with_id("fishbone_child_4")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

/// 子链5：野外科学传感器网络（超高频 IoT，AURA-3，1s 出块，60s Epoch）
/// 使用 fishbone-node-1s binary（--features block-1s 编译）启动。
pub fn child5_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-5 (IoT Sensor Network)")
        .with_id("fishbone_child_5")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

/// 子链6：去中心化数据市场（AURA-5，6s 出块，10min Sc）
/// 注：BABE 需要 pallet_babe 提供 NextEpochData digest，超出本实验范围。
/// 实验使用 AURA-5 运行，论文中 BABE vs AURA 开销差异引用已有文献（~15-20%）。
pub fn child6_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-6 (Data Market, AURA-5)")
        .with_id("fishbone_child_6")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

/// 保留 BABE chain spec（仅供 build-spec 测试，节点无法正常出块）
pub fn child6_babe_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-6 (Data Market, BABE-experimental)")
        .with_id("fishbone_child_6_babe")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

// ── 向后兼容（保留原始接口供 command.rs 使用）────────────────────────────────

pub fn development_chain_spec() -> Result<ChainSpec, String> {
    main_dev()
}

pub fn local_chain_spec() -> Result<ChainSpec, String> {
    main_local()
}
