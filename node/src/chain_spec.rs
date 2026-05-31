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

/// 子链1：数据众包场景（Bob+Charlie 验证者）
pub fn child1_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-1 (Crowdsource)")
        .with_id("fishbone_child_1")
        .with_chain_type(ChainType::Local)
        .with_genesis_config_preset_name(sp_genesis_builder::LOCAL_TESTNET_RUNTIME_PRESET)
        .build())
}

/// 子链2：数据交易场景（Dave+Eve 验证者）
pub fn child2_local() -> Result<ChainSpec, String> {
    Ok(ChainSpec::builder(wasm(), None)
        .with_name("Fishbone Child-2 (Data Trading)")
        .with_id("fishbone_child_2")
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
