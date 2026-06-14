use crate::{mock::*, Error, Event, NextSessionId, SessionStatus, Sessions, TradeSettlementMode};
use frame_support::{assert_noop, assert_ok};
use sp_runtime::traits::Hash;

fn hash_once(bytes: &[u8]) -> sp_core::H256 {
	<Test as frame_system::Config>::Hashing::hash(bytes)
}

fn create_main_escrow_session() {
	assert_ok!(TradeSession::create_session(
		RuntimeOrigin::signed(1),
		2,
		hash_once(b"secret"),
		1,
		TradeSettlementMode::MainEscrow,
	));
}

#[test]
fn requester_creates_session() {
	new_test_ext().execute_with(|| {
		let end = hash_once(b"secret");

		assert_ok!(TradeSession::create_session(
			RuntimeOrigin::signed(1),
			2,
			end,
			1,
			TradeSettlementMode::MainEscrow,
		));

		let session = Sessions::<Test>::get(0).expect("session exists");
		assert_eq!(session.requester, 1);
		assert_eq!(session.data_owner, 2);
		assert_eq!(session.hash_chain_end, end);
		assert_eq!(session.max_rounds, 1);
		assert_eq!(session.status, SessionStatus::Created);
		assert_eq!(session.settlement_mode, TradeSettlementMode::MainEscrow);
		assert_eq!(NextSessionId::<Test>::get(), 1);
		System::assert_has_event(
			Event::SessionCreated { session_id: 0, requester: 1, data_owner: 2 }.into(),
		);
	});
}

#[test]
fn requester_locks_funds() {
	new_test_ext().execute_with(|| {
		create_main_escrow_session();

		assert_ok!(TradeSession::lock_funds(RuntimeOrigin::signed(1), 0, 100));

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.locked_funds, 100);
		assert_eq!(session.status, SessionStatus::Funded);
		assert_eq!(Balances::reserved_balance(1), 100);
	});
}

#[test]
fn data_owner_locks_deposit_after_funded() {
	new_test_ext().execute_with(|| {
		create_main_escrow_session();
		assert_ok!(TradeSession::lock_funds(RuntimeOrigin::signed(1), 0, 100));

		assert_ok!(TradeSession::lock_deposit(RuntimeOrigin::signed(2), 0, 50));

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.deposit, 50);
		assert_eq!(session.status, SessionStatus::DepositLocked);
		assert_eq!(Balances::reserved_balance(2), 50);
	});
}

#[test]
fn non_data_owner_cannot_lock_deposit() {
	new_test_ext().execute_with(|| {
		create_main_escrow_session();
		assert_ok!(TradeSession::lock_funds(RuntimeOrigin::signed(1), 0, 100));

		assert_noop!(
			TradeSession::lock_deposit(RuntimeOrigin::signed(3), 0, 50),
			Error::<Test>::NotDataOwner
		);
	});
}

#[test]
fn data_owner_claims_with_valid_hash_chain_preimage() {
	new_test_ext().execute_with(|| {
		create_main_escrow_session();
		assert_ok!(TradeSession::lock_funds(RuntimeOrigin::signed(1), 0, 100));
		assert_ok!(TradeSession::lock_deposit(RuntimeOrigin::signed(2), 0, 50));

		assert_ok!(TradeSession::claim_funds(RuntimeOrigin::signed(2), 0, b"secret".to_vec()));

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.status, SessionStatus::Settled);
		assert_eq!(Balances::reserved_balance(1), 0);
		assert_eq!(Balances::reserved_balance(2), 0);
		assert_eq!(Balances::free_balance(2), 1_000_100);
		System::assert_has_event(
			Event::FundsClaimed { session_id: 0, data_owner: 2, rounds: 1 }.into(),
		);
	});
}

#[test]
fn fmc_assisted_lock_funds_is_unsupported_in_first_version() {
	new_test_ext().execute_with(|| {
		assert_ok!(TradeSession::create_session(
			RuntimeOrigin::signed(1),
			2,
			hash_once(b"secret"),
			1,
			TradeSettlementMode::FmcAssisted,
		));

		assert_noop!(
			TradeSession::lock_funds(RuntimeOrigin::signed(1), 0, 100),
			Error::<Test>::UnsupportedSettlementMode
		);
	});
}
