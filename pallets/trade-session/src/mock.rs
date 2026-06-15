use codec::Encode;
use frame_support::{assert_ok, construct_runtime, derive_impl, parameter_types};
use sp_runtime::BuildStorage;

use crate::proof::{AlwaysPassVerifier, ListingProvider};
use crate::types::ListingId;

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

/// Test listing provider — uses a simple static store for tests.
/// For real runtime, data-registry implements this trait.
pub struct TestListingProvider;
impl ListingProvider<u64, u64, sp_core::H256> for TestListingProvider {
	fn listing_exists(listing_id: ListingId) -> bool {
		// In tests, listing_id=0 always exists and is owned by account 2
		listing_id == 0
	}
	fn listing_owner(listing_id: ListingId) -> Option<u64> {
		if listing_id == 0 { Some(2) } else { None }
	}
	fn listing_active(listing_id: ListingId) -> bool {
		listing_id == 0
	}
	fn listing_terms(listing_id: ListingId) -> Option<(u64, u32, u64, sp_core::H256)> {
		if listing_id == 0 {
			Some((100, 5, 300, sp_core::H256::default()))
		} else {
			None
		}
	}
}

parameter_types! {
	pub const VerifierAccount: u64 = 3;
}

impl crate::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type ListingProvider = TestListingProvider;
	type ProofVerifier = AlwaysPassVerifier;
	type VerifierAuthority = VerifierAccount;
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

/// Hash once using Blake2-256 (same as runtime's Hashing).
pub fn hash_once(bytes: &[u8]) -> sp_core::H256 {
	sp_core::blake2_256(bytes).into()
}

/// Hash `data` exactly `n` times.
pub fn hash_n_times(data: &[u8], n: u32) -> sp_core::H256 {
	let mut current = hash_once(data);
	for _ in 1..n {
		current = hash_once(&current.encode());
	}
	current
}

/// Helper: create a session (DR=1, DO=2, listing_id=0, escrow_id=42).
pub fn create_session_helper() {
	assert_ok!(crate::Pallet::<Test>::create_session(
		frame_system::RawOrigin::Signed(1).into(),
		0,    // listing_id
		42,   // escrow_id
		2,    // data_owner
		sp_core::H256::repeat_byte(4),
		100,  // price_per_round
		5,    // max_rounds
		hash_n_times(b"secret", 5),
		crate::types::TradeSettlementMode::MainEscrow,
	));
}

/// Helper: accept session (DO=2).
pub fn accept_session_helper(session_id: u32) {
	assert_ok!(crate::Pallet::<Test>::accept_session(
		frame_system::RawOrigin::Signed(2).into(),
		session_id,
	));
}

/// Helper: complete one round (DR=1, DO=2).
pub fn complete_round(session_id: u32, round_index: u32) {
	let ch = sp_core::H256::repeat_byte(round_index as u8 + 1);

	assert_ok!(crate::Pallet::<Test>::open_round(
		frame_system::RawOrigin::Signed(1).into(),
		session_id,
		round_index,
		ch,
	));
	assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
		frame_system::RawOrigin::Signed(1).into(),
		session_id,
		round_index,
		ch,
	));
	assert_ok!(crate::Pallet::<Test>::submit_data_proof(
		frame_system::RawOrigin::Signed(2).into(),
		session_id,
		round_index,
		ch,
	));
	assert_ok!(crate::Pallet::<Test>::attest_data_proof(
		frame_system::RawOrigin::Signed(3).into(),
		session_id,
		round_index,
		ch,
		true,
	));
	assert_ok!(crate::Pallet::<Test>::submit_proof_signature(
		frame_system::RawOrigin::Signed(1).into(),
		session_id,
		round_index,
		ch,
	));
	assert_ok!(crate::Pallet::<Test>::submit_data_delivery_hash(
		frame_system::RawOrigin::Signed(2).into(),
		session_id,
		round_index,
		ch,
	));
	assert_ok!(crate::Pallet::<Test>::submit_payment_preimage(
		frame_system::RawOrigin::Signed(1).into(),
		session_id,
		round_index,
		ch,
	));
}
