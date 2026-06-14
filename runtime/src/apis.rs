// This is free and unencumbered software released into the public domain.
//
// Anyone is free to copy, modify, publish, use, compile, sell, or
// distribute this software, either in source code form or as a compiled
// binary, for any purpose, commercial or non-commercial, and by any
// means.
//
// In jurisdictions that recognize copyright laws, the author or authors
// of this software dedicate any and all copyright interest in the
// software to the public domain. We make this dedication for the benefit
// of the public at large and to the detriment of our heirs and
// successors. We intend this dedication to be an overt act of
// relinquishment in perpetuity of all present and future rights to this
// software under copyright law.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
//
// For more information, please refer to <http://unlicense.org>

// External crates imports
use alloc::vec::Vec;
use frame_support::{
	genesis_builder_helper::{build_state, get_preset},
	weights::Weight,
};
use pallet_grandpa::AuthorityId as GrandpaId;
use sp_api::impl_runtime_apis;
use sp_consensus_aura::sr25519::AuthorityId as AuraId;
use sp_consensus_babe::AuthorityId as BabeId;
use sp_core::{crypto::KeyTypeId, OpaqueMetadata};
use sp_runtime::{
	traits::{Block as BlockT, NumberFor},
	transaction_validity::{TransactionSource, TransactionValidity},
	ApplyExtrinsicResult,
};
use sp_version::RuntimeVersion;

// Local module imports
use super::Aura;
#[cfg(feature = "babe")]
use super::Babe;
use super::{
	AccountId, Balance, Block, Executive, Grandpa, InherentDataExt, Nonce, Runtime, RuntimeCall,
	RuntimeGenesisConfig, SessionKeys, System, TransactionPayment, VERSION,
};
#[cfg(feature = "babe")]
use pallet_babe;

// ── BabeApi helpers ──────────────────────────────────────────────────────────
// Defined outside impl_runtime_apis! so #[cfg] is respected by the compiler.
// impl_runtime_apis! is a proc-macro that processes tokens before cfg expansion,
// so we cannot have two impl BabeApi blocks inside it under different #[cfg].

#[cfg(not(feature = "babe"))]
fn babe_api_configuration() -> sp_consensus_babe::BabeConfiguration {
	use sp_core::crypto::ByteArray;
	let authorities: Vec<(BabeId, u64)> = pallet_aura::Authorities::<Runtime>::get()
		.into_iter()
		.filter_map(|id| BabeId::from_slice(id.as_slice()).ok().map(|b| (b, 1u64)))
		.collect();
	sp_consensus_babe::BabeConfiguration {
		slot_duration: crate::SLOT_DURATION,
		epoch_length: 500_000_000,
		c: (1, 4),
		authorities,
		randomness: Default::default(),
		allowed_slots: sp_consensus_babe::AllowedSlots::PrimaryAndSecondaryVRFSlots,
	}
}
#[cfg(not(feature = "babe"))]
fn babe_api_current_epoch_start() -> sp_consensus_babe::Slot {
	sp_consensus_babe::Slot::from(0u64)
}
#[cfg(not(feature = "babe"))]
fn babe_api_current_epoch() -> sp_consensus_babe::Epoch {
	use sp_core::crypto::ByteArray;
	const EPOCH_LEN: u64 = 500_000_000;
	let authorities: Vec<(BabeId, u64)> = pallet_aura::Authorities::<Runtime>::get()
		.into_iter()
		.filter_map(|id| BabeId::from_slice(id.as_slice()).ok().map(|b| (b, 1u64)))
		.collect();
	sp_consensus_babe::Epoch {
		epoch_index: 0,
		start_slot: sp_consensus_babe::Slot::from(0u64),
		duration: EPOCH_LEN,
		authorities,
		randomness: Default::default(),
		config: sp_consensus_babe::BabeEpochConfiguration {
			c: (1, 4),
			allowed_slots: sp_consensus_babe::AllowedSlots::PrimaryAndSecondaryVRFSlots,
		},
	}
}
#[cfg(not(feature = "babe"))]
fn babe_api_next_epoch() -> sp_consensus_babe::Epoch {
	use sp_core::crypto::ByteArray;
	const EPOCH_LEN: u64 = 500_000_000;
	let authorities: Vec<(BabeId, u64)> = pallet_aura::Authorities::<Runtime>::get()
		.into_iter()
		.filter_map(|id| BabeId::from_slice(id.as_slice()).ok().map(|b| (b, 1u64)))
		.collect();
	sp_consensus_babe::Epoch {
		epoch_index: 1,
		start_slot: sp_consensus_babe::Slot::from(EPOCH_LEN),
		duration: EPOCH_LEN,
		authorities,
		randomness: Default::default(),
		config: sp_consensus_babe::BabeEpochConfiguration {
			c: (1, 4),
			allowed_slots: sp_consensus_babe::AllowedSlots::PrimaryAndSecondaryVRFSlots,
		},
	}
}

#[cfg(feature = "babe")]
fn babe_api_configuration() -> sp_consensus_babe::BabeConfiguration {
	let epoch_config = pallet_babe::EpochConfig::<Runtime>::get().unwrap_or(
		sp_consensus_babe::BabeEpochConfiguration {
			c: (1, 4),
			allowed_slots: sp_consensus_babe::AllowedSlots::PrimaryAndSecondaryVRFSlots,
		},
	);
	sp_consensus_babe::BabeConfiguration {
		slot_duration: Babe::slot_duration(),
		epoch_length: crate::configs::BabeEpochDuration::get(),
		c: epoch_config.c,
		authorities: Babe::authorities().to_vec(),
		randomness: Babe::randomness(),
		allowed_slots: epoch_config.allowed_slots,
	}
}
#[cfg(feature = "babe")]
fn babe_api_current_epoch_start() -> sp_consensus_babe::Slot {
	Babe::current_epoch_start()
}
#[cfg(feature = "babe")]
fn babe_api_current_epoch() -> sp_consensus_babe::Epoch {
	Babe::current_epoch()
}
#[cfg(feature = "babe")]
fn babe_api_next_epoch() -> sp_consensus_babe::Epoch {
	Babe::next_epoch()
}

impl_runtime_apis! {
	impl sp_api::Core<Block> for Runtime {
		fn version() -> RuntimeVersion {
			VERSION
		}

		fn execute_block(block: Block) {
			Executive::execute_block(block);
		}

		fn initialize_block(header: &<Block as BlockT>::Header) -> sp_runtime::ExtrinsicInclusionMode {
			Executive::initialize_block(header)
		}
	}

	impl sp_api::Metadata<Block> for Runtime {
		fn metadata() -> OpaqueMetadata {
			OpaqueMetadata::new(Runtime::metadata().into())
		}

		fn metadata_at_version(version: u32) -> Option<OpaqueMetadata> {
			Runtime::metadata_at_version(version)
		}

		fn metadata_versions() -> Vec<u32> {
			Runtime::metadata_versions()
		}
	}

	impl frame_support::view_functions::runtime_api::RuntimeViewFunction<Block> for Runtime {
		fn execute_view_function(id: frame_support::view_functions::ViewFunctionId, input: Vec<u8>) -> Result<Vec<u8>, frame_support::view_functions::ViewFunctionDispatchError> {
			Runtime::execute_view_function(id, input)
		}
	}

	impl sp_block_builder::BlockBuilder<Block> for Runtime {
		fn apply_extrinsic(extrinsic: <Block as BlockT>::Extrinsic) -> ApplyExtrinsicResult {
			Executive::apply_extrinsic(extrinsic)
		}

		fn finalize_block() -> <Block as BlockT>::Header {
			Executive::finalize_block()
		}

		fn inherent_extrinsics(data: sp_inherents::InherentData) -> Vec<<Block as BlockT>::Extrinsic> {
			data.create_extrinsics()
		}

		fn check_inherents(
			block: Block,
			data: sp_inherents::InherentData,
		) -> sp_inherents::CheckInherentsResult {
			data.check_extrinsics(&block)
		}
	}

	impl sp_transaction_pool::runtime_api::TaggedTransactionQueue<Block> for Runtime {
		fn validate_transaction(
			source: TransactionSource,
			tx: <Block as BlockT>::Extrinsic,
			block_hash: <Block as BlockT>::Hash,
		) -> TransactionValidity {
			Executive::validate_transaction(source, tx, block_hash)
		}
	}

	impl sp_offchain::OffchainWorkerApi<Block> for Runtime {
		fn offchain_worker(header: &<Block as BlockT>::Header) {
			Executive::offchain_worker(header)
		}
	}

	// AuraApi is always implemented: pallet_aura exists in both AURA and BABE runtimes.
	// service.rs (AURA consensus service) requires AuraApi to compile even in BABE builds.
	impl sp_consensus_aura::AuraApi<Block, AuraId> for Runtime {
		fn slot_duration() -> sp_consensus_aura::SlotDuration {
			sp_consensus_aura::SlotDuration::from_millis(Aura::slot_duration())
		}

		fn authorities() -> Vec<AuraId> {
			pallet_aura::Authorities::<Runtime>::get().into_inner()
		}
	}

	// Single BabeApi impl delegates to feature-gated helpers defined above impl_runtime_apis!.
	impl sp_consensus_babe::BabeApi<Block> for Runtime {
		fn configuration() -> sp_consensus_babe::BabeConfiguration {
			babe_api_configuration()
		}
		fn current_epoch_start() -> sp_consensus_babe::Slot {
			babe_api_current_epoch_start()
		}
		fn current_epoch() -> sp_consensus_babe::Epoch {
			babe_api_current_epoch()
		}
		fn next_epoch() -> sp_consensus_babe::Epoch {
			babe_api_next_epoch()
		}
		fn generate_key_ownership_proof(
			_slot: sp_consensus_babe::Slot,
			_authority_id: BabeId,
		) -> Option<sp_consensus_babe::OpaqueKeyOwnershipProof> {
			None
		}
		fn submit_report_equivocation_unsigned_extrinsic(
			_equivocation_proof: sp_consensus_babe::EquivocationProof<<Block as BlockT>::Header>,
			_key_owner_proof: sp_consensus_babe::OpaqueKeyOwnershipProof,
		) -> Option<()> {
			None
		}
	}


	impl sp_session::SessionKeys<Block> for Runtime {
		fn generate_session_keys(seed: Option<Vec<u8>>) -> Vec<u8> {
			SessionKeys::generate(seed)
		}

		fn decode_session_keys(
			encoded: Vec<u8>,
		) -> Option<Vec<(Vec<u8>, KeyTypeId)>> {
			SessionKeys::decode_into_raw_public_keys(&encoded)
		}
	}

	impl sp_consensus_grandpa::GrandpaApi<Block> for Runtime {
		fn grandpa_authorities() -> sp_consensus_grandpa::AuthorityList {
			Grandpa::grandpa_authorities()
		}

		fn current_set_id() -> sp_consensus_grandpa::SetId {
			Grandpa::current_set_id()
		}

		fn submit_report_equivocation_unsigned_extrinsic(
			_equivocation_proof: sp_consensus_grandpa::EquivocationProof<
				<Block as BlockT>::Hash,
				NumberFor<Block>,
			>,
			_key_owner_proof: sp_consensus_grandpa::OpaqueKeyOwnershipProof,
		) -> Option<()> {
			None
		}

		fn generate_key_ownership_proof(
			_set_id: sp_consensus_grandpa::SetId,
			_authority_id: GrandpaId,
		) -> Option<sp_consensus_grandpa::OpaqueKeyOwnershipProof> {
			// NOTE: this is the only implementation possible since we've
			// defined our key owner proof type as a bottom type (i.e. a type
			// with no values).
			None
		}
	}

	impl frame_system_rpc_runtime_api::AccountNonceApi<Block, AccountId, Nonce> for Runtime {
		fn account_nonce(account: AccountId) -> Nonce {
			System::account_nonce(account)
		}
	}

	impl pallet_transaction_payment_rpc_runtime_api::TransactionPaymentApi<Block, Balance> for Runtime {
		fn query_info(
			uxt: <Block as BlockT>::Extrinsic,
			len: u32,
		) -> pallet_transaction_payment_rpc_runtime_api::RuntimeDispatchInfo<Balance> {
			TransactionPayment::query_info(uxt, len)
		}
		fn query_fee_details(
			uxt: <Block as BlockT>::Extrinsic,
			len: u32,
		) -> pallet_transaction_payment::FeeDetails<Balance> {
			TransactionPayment::query_fee_details(uxt, len)
		}
		fn query_weight_to_fee(weight: Weight) -> Balance {
			TransactionPayment::weight_to_fee(weight)
		}
		fn query_length_to_fee(length: u32) -> Balance {
			TransactionPayment::length_to_fee(length)
		}
	}

	impl pallet_transaction_payment_rpc_runtime_api::TransactionPaymentCallApi<Block, Balance, RuntimeCall>
		for Runtime
	{
		fn query_call_info(
			call: RuntimeCall,
			len: u32,
		) -> pallet_transaction_payment::RuntimeDispatchInfo<Balance> {
			TransactionPayment::query_call_info(call, len)
		}
		fn query_call_fee_details(
			call: RuntimeCall,
			len: u32,
		) -> pallet_transaction_payment::FeeDetails<Balance> {
			TransactionPayment::query_call_fee_details(call, len)
		}
		fn query_weight_to_fee(weight: Weight) -> Balance {
			TransactionPayment::weight_to_fee(weight)
		}
		fn query_length_to_fee(length: u32) -> Balance {
			TransactionPayment::length_to_fee(length)
		}
	}

	#[cfg(feature = "runtime-benchmarks")]
	impl frame_benchmarking::Benchmark<Block> for Runtime {
		fn benchmark_metadata(extra: bool) -> (
			Vec<frame_benchmarking::BenchmarkList>,
			Vec<frame_support::traits::StorageInfo>,
		) {
			use frame_benchmarking::{baseline, BenchmarkList};
			use frame_support::traits::StorageInfoTrait;
			use frame_system_benchmarking::Pallet as SystemBench;
			use frame_system_benchmarking::extensions::Pallet as SystemExtensionsBench;
			use baseline::Pallet as BaselineBench;
			use super::*;

			let mut list = Vec::<BenchmarkList>::new();
			list_benchmarks!(list, extra);

			let storage_info = AllPalletsWithSystem::storage_info();

			(list, storage_info)
		}

		#[allow(non_local_definitions)]
		fn dispatch_benchmark(
			config: frame_benchmarking::BenchmarkConfig
		) -> Result<Vec<frame_benchmarking::BenchmarkBatch>, alloc::string::String> {
			use frame_benchmarking::{baseline, BenchmarkBatch};
			use sp_storage::TrackedStorageKey;
			use frame_system_benchmarking::Pallet as SystemBench;
			use frame_system_benchmarking::extensions::Pallet as SystemExtensionsBench;
			use baseline::Pallet as BaselineBench;
			use super::*;

			impl frame_system_benchmarking::Config for Runtime {}
			impl baseline::Config for Runtime {}

			use frame_support::traits::WhitelistedStorageKeys;
			let whitelist: Vec<TrackedStorageKey> = AllPalletsWithSystem::whitelisted_storage_keys();

			let mut batches = Vec::<BenchmarkBatch>::new();
			let params = (&config, &whitelist);
			add_benchmarks!(params, batches);

			Ok(batches)
		}
	}

	#[cfg(feature = "try-runtime")]
	impl frame_try_runtime::TryRuntime<Block> for Runtime {
		fn on_runtime_upgrade(checks: frame_try_runtime::UpgradeCheckSelect) -> (Weight, Weight) {
			// NOTE: intentional unwrap: we don't want to propagate the error backwards, and want to
			// have a backtrace here. If any of the pre/post migration checks fail, we shall stop
			// right here and right now.
			let weight = Executive::try_runtime_upgrade(checks).unwrap();
			(weight, super::configs::RuntimeBlockWeights::get().max_block)
		}

		fn execute_block(
			block: Block,
			state_root_check: bool,
			signature_check: bool,
			select: frame_try_runtime::TryStateSelect
		) -> Weight {
			// NOTE: intentional unwrap: we don't want to propagate the error backwards, and want to
			// have a backtrace here.
			Executive::try_execute_block(block, state_root_check, signature_check, select).expect("execute-block failed")
		}
	}

	impl sp_genesis_builder::GenesisBuilder<Block> for Runtime {
		fn build_state(config: Vec<u8>) -> sp_genesis_builder::Result {
			build_state::<RuntimeGenesisConfig>(config)
		}

		fn get_preset(id: &Option<sp_genesis_builder::PresetId>) -> Option<Vec<u8>> {
			get_preset::<RuntimeGenesisConfig>(id, crate::genesis_config_presets::get_preset)
		}

		fn preset_names() -> Vec<sp_genesis_builder::PresetId> {
			crate::genesis_config_presets::preset_names()
		}
	}
}
