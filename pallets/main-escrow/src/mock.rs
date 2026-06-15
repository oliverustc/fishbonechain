use codec::Encode;
use frame_support::{assert_ok, construct_runtime, derive_impl};
use sp_runtime::BuildStorage;

type Block = frame_system::mocking::MockBlock<Test>;

construct_runtime!(
	pub enum Test {
		System: frame_system,
		Balances: pallet_balances,
		MainEscrow: crate,
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
		balances: vec![(1, 1_000_000), (2, 1_000_000), (3, 1_000_000), (4, 1_000_000)],
		dev_accounts: None,
	}
	.assimilate_storage(&mut storage)
	.unwrap();
	let mut ext = sp_io::TestExternalities::new(storage);
	ext.execute_with(|| frame_system::Pallet::<Test>::set_block_number(1));
	ext
}

/// Hash a value once using Blake2-256.
pub fn hash_once(data: &[u8]) -> sp_core::H256 {
	sp_core::blake2_256(data).into()
}

/// Hash `data` exactly `n` times.
pub fn hash_n_times(data: &[u8], n: u32) -> sp_core::H256 {
	let mut current = hash_once(data);
	for _ in 1..n {
		current = hash_once(&current.encode());
	}
	current
}

/// Hash `data` `n` times, return the raw bytes of the result.
pub fn hash_n_times_bytes(data: &[u8], n: u32) -> Vec<u8> {
	hash_n_times(data, n).encode()
}

/// Open an escrow, lock funds, and lock deposit (status Ready).
pub fn open_ready_escrow(
	anchor: sp_core::H256,
	max_rounds: u32,
	price_per_round: u64,
	deposit: u64,
) {
	assert_ok!(crate::Pallet::<Test>::open_escrow(
		frame_system::RawOrigin::Signed(1).into(),
		2,
		max_rounds,
		price_per_round,
		deposit,
		anchor,
	));
	assert_ok!(crate::Pallet::<Test>::lock_funds(frame_system::RawOrigin::Signed(1).into(), 0,));
	assert_ok!(crate::Pallet::<Test>::lock_deposit(frame_system::RawOrigin::Signed(2).into(), 0,));
}
