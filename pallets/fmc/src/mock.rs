use frame_support::{construct_runtime, derive_impl};
use sp_runtime::BuildStorage;

use crate::pallet::CcmcInterface;

type Block = frame_system::mocking::MockBlock<Test>;

// Mock CCMC：测试用，所有账户都是矿工，矿工数固定为 1
pub struct MockCcmc;
impl CcmcInterface<u64> for MockCcmc {
	fn is_miner(_chain_id: u32, _who: &u64) -> bool {
		true
	}
	fn miner_count(_chain_id: u32) -> u32 {
		1
	}
}

construct_runtime!(
	pub enum Test {
		System:   frame_system,
		Balances: pallet_balances,
		Fmc:      crate,
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
	type CcmcPallet = MockCcmc;
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
