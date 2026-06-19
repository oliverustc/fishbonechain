use frame_support::{
	construct_runtime, derive_impl, parameter_types,
	traits::{ConstBool, OnInitialize},
};
use sp_runtime::BuildStorage;

use crate::pallet::AlwaysValidate;

type Block = frame_system::mocking::MockBlock<Test>;

construct_runtime!(
	pub enum Test {
		System:      frame_system,
		Balances:    pallet_balances,
		Crowdsource: crate,
	}
);

#[derive_impl(frame_system::config_preludes::TestDefaultConfig)]
impl frame_system::Config for Test {
	type Block = Block;
	type AccountData = pallet_balances::AccountData<u64>;
}

#[derive_impl(pallet_balances::config_preludes::TestDefaultConfig)]
impl pallet_balances::Config for Test {
	type AccountStore = System;
}

parameter_types! {
	pub const TestCollectingSlot: u32 = 100;
	pub const TestSyncingSlot: u32    = 20;
}

pub struct MockChainProfile;

impl pallet_chain_profile::ChainIdentityProvider for MockChainProfile {
	fn chain_id() -> pallet_ccmc::types::ChainId {
		5
	}

	fn scene_kind() -> pallet_chain_profile::SceneKind {
		pallet_chain_profile::SceneKind::Crowdsource
	}

	fn settlement_mode() -> pallet_chain_profile::SettlementMode {
		pallet_chain_profile::SettlementMode::FmcTaskBill
	}
}

impl crate::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type ChainProfile = MockChainProfile;
	type CollectingSlotBlocks = TestCollectingSlot;
	type SyncingSlotBlocks = TestSyncingSlot;
	type MaxSubmissionsPerEpoch = frame_support::traits::ConstU32<100>;
	type DataValidator = AlwaysValidate;
	type FullSubmissionEvents = ConstBool<true>;
	type IndexedSubmissionStorage = ConstBool<false>;
	type MaxBatchSize = frame_support::traits::ConstU32<1>;
	type WeightInfo = ();
}

pub fn new_test_ext() -> sp_io::TestExternalities {
	let mut storage = frame_system::GenesisConfig::<Test>::default().build_storage().unwrap();
	pallet_balances::GenesisConfig::<Test> {
		balances: vec![(1, 1_000_000), (2, 1_000_000), (3, 1_000_000)],
		dev_accounts: None,
	}
	.assimilate_storage(&mut storage)
	.unwrap();
	let mut ext = sp_io::TestExternalities::new(storage);
	ext.execute_with(|| frame_system::Pallet::<Test>::set_block_number(1));
	ext
}

/// 把区块号推进到 n，触发 on_initialize
pub fn run_to_block(n: u64) {
	while System::block_number() < n {
		let next = System::block_number() + 1;
		System::set_block_number(next);
		Crowdsource::on_initialize(next);
	}
}

pub use indexed::{
	new_test_ext as new_indexed_test_ext, Crowdsource as IndexedCrowdsource,
	RuntimeOrigin as IndexedRuntimeOrigin, Test as IndexedTest,
};

pub mod indexed {
	use super::*;

	type Block = frame_system::mocking::MockBlock<Test>;

	construct_runtime!(
		pub enum Test {
			System:      frame_system,
			Balances:    pallet_balances,
			Crowdsource: crate,
		}
	);

	#[derive_impl(frame_system::config_preludes::TestDefaultConfig)]
	impl frame_system::Config for Test {
		type Block = Block;
		type AccountData = pallet_balances::AccountData<u64>;
	}

	#[derive_impl(pallet_balances::config_preludes::TestDefaultConfig)]
	impl pallet_balances::Config for Test {
		type AccountStore = System;
	}

	impl crate::Config for Test {
		type RuntimeEvent = RuntimeEvent;
		type Currency = Balances;
		type ChainProfile = MockChainProfile;
		type CollectingSlotBlocks = TestCollectingSlot;
		type SyncingSlotBlocks = TestSyncingSlot;
		type MaxSubmissionsPerEpoch = frame_support::traits::ConstU32<100>;
		type DataValidator = AlwaysValidate;
		type FullSubmissionEvents = ConstBool<false>;
		type IndexedSubmissionStorage = ConstBool<true>;
		type MaxBatchSize = frame_support::traits::ConstU32<4>;
		type WeightInfo = ();
	}

	pub fn new_test_ext() -> sp_io::TestExternalities {
		let mut storage = frame_system::GenesisConfig::<Test>::default().build_storage().unwrap();
		pallet_balances::GenesisConfig::<Test> {
			balances: vec![(1, 1_000_000), (2, 1_000_000), (3, 1_000_000)],
			dev_accounts: None,
		}
		.assimilate_storage(&mut storage)
		.unwrap();
		let mut ext = sp_io::TestExternalities::new(storage);
		ext.execute_with(|| frame_system::Pallet::<Test>::set_block_number(1));
		ext
	}
}
