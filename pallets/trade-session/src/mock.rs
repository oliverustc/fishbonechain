use frame_support::{construct_runtime, derive_impl};
use sp_runtime::BuildStorage;

type Block = frame_system::mocking::MockBlock<Test>;

construct_runtime!(
	pub enum Test {
		System: frame_system,
		Balances: pallet_balances,
		TradeSession: crate,
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
