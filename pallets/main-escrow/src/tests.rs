use crate::{mock::*, types::EscrowStatus, Error, Escrows};
use frame_support::{assert_noop, assert_ok};

#[test]
fn dr_locks_funds_and_do_locks_deposit() {
	new_test_ext().execute_with(|| {
		let anchor = sp_core::H256::repeat_byte(9);

		assert_ok!(MainEscrow::open_escrow(RuntimeOrigin::signed(1), 2, 5, 100, 300, anchor,));
		assert_ok!(MainEscrow::lock_funds(RuntimeOrigin::signed(1), 0));
		assert_ok!(MainEscrow::lock_deposit(RuntimeOrigin::signed(2), 0));

		let escrow = Escrows::<Test>::get(0).expect("escrow exists");
		assert_eq!(escrow.requester, 1);
		assert_eq!(escrow.data_owner, 2);
		assert_eq!(escrow.total_funds, 500);
		assert_eq!(escrow.deposit, 300);
		assert_eq!(escrow.status, EscrowStatus::Ready);
		assert_eq!(Balances::reserved_balance(1), 500);
		assert_eq!(Balances::reserved_balance(2), 300);
	});
}

#[test]
fn do_claims_partial_payment_and_requester_gets_refund() {
	new_test_ext().execute_with(|| {
		let secret = b"secret".to_vec();
		let anchor = hash_n_times(&secret, 5);
		open_ready_escrow(anchor, 5, 100, 300);

		let preimage_after_two_rounds = hash_n_times_bytes(&secret, 3);
		assert_ok!(MainEscrow::settle_by_preimage(
			RuntimeOrigin::signed(2),
			0,
			preimage_after_two_rounds,
			3,
		));

		assert_eq!(Escrows::<Test>::get(0).unwrap().status, EscrowStatus::Settled);
		assert_eq!(Balances::reserved_balance(1), 0);
		assert_eq!(Balances::reserved_balance(2), 0);
		// DO earned 2 rounds * 100 = 200, original balance 1_000_000
		assert_eq!(Balances::free_balance(2), 1_000_000 + 200);
		System::assert_last_event(RuntimeEvent::MainEscrow(crate::Event::EscrowSettled {
			escrow_id: 0,
			paid_rounds: 2,
			refunded: 300,
		}));
	});
}

#[test]
fn open_escrow_rejects_zero_params() {
	new_test_ext().execute_with(|| {
		let anchor = sp_core::H256::repeat_byte(9);
		assert_noop!(
			MainEscrow::open_escrow(RuntimeOrigin::signed(1), 2, 0, 100, 300, anchor),
			Error::<Test>::InvalidTradeTerms,
		);
		assert_noop!(
			MainEscrow::open_escrow(RuntimeOrigin::signed(1), 2, 5, 0, 300, anchor),
			Error::<Test>::InvalidTradeTerms,
		);
		assert_noop!(
			MainEscrow::open_escrow(RuntimeOrigin::signed(1), 2, 5, 100, 0, anchor),
			Error::<Test>::InvalidTradeTerms,
		);
	});
}

#[test]
fn dr_can_punish_data_owner() {
	new_test_ext().execute_with(|| {
		let anchor = sp_core::H256::repeat_byte(9);
		open_ready_escrow(anchor, 5, 100, 300);

		assert_ok!(MainEscrow::punish_data_owner(RuntimeOrigin::signed(1), 0));

		let escrow = Escrows::<Test>::get(0).unwrap();
		assert_eq!(escrow.status, EscrowStatus::Punished);
		// DR gets all funds refunded plus the punished DO deposit.
		assert_eq!(Balances::free_balance(1), 1_000_000 + 300);
		assert_eq!(Balances::reserved_balance(1), 0);
		// DO loses the reserved deposit.
		assert_eq!(Balances::free_balance(2), 1_000_000 - 300);
		assert_eq!(Balances::reserved_balance(2), 0);
	});
}

#[test]
fn do_can_claim_last_payment() {
	new_test_ext().execute_with(|| {
		let anchor = sp_core::H256::repeat_byte(9);
		open_ready_escrow(anchor, 5, 100, 300);

		assert_ok!(MainEscrow::claim_last_payment(RuntimeOrigin::signed(2), 0, 0));

		let escrow = Escrows::<Test>::get(0).unwrap();
		assert_eq!(escrow.status, EscrowStatus::Settled);
		assert_eq!(escrow.paid_rounds, 1);
		// DO gets one round payment (100)
		assert_eq!(Balances::free_balance(2), 1_000_000 + 100);
		// DR reserved: 400 refunded, 0 left
		assert_eq!(Balances::reserved_balance(1), 0);
		assert_eq!(Balances::reserved_balance(2), 0);
		System::assert_last_event(RuntimeEvent::MainEscrow(crate::Event::EscrowSettled {
			escrow_id: 0,
			paid_rounds: 1,
			refunded: 400,
		}));
	});
}

#[test]
fn wrong_actor_cannot_call() {
	new_test_ext().execute_with(|| {
		let anchor = sp_core::H256::repeat_byte(9);
		assert_ok!(MainEscrow::open_escrow(RuntimeOrigin::signed(1), 2, 5, 100, 300, anchor,));

		// Non-requester cannot lock funds
		assert_noop!(
			MainEscrow::lock_funds(RuntimeOrigin::signed(2), 0),
			Error::<Test>::NotRequester,
		);

		// Non-data-owner cannot lock deposit (escrow not yet Funded anyway)
		assert_ok!(MainEscrow::lock_funds(RuntimeOrigin::signed(1), 0));
		assert_noop!(
			MainEscrow::lock_deposit(RuntimeOrigin::signed(1), 0),
			Error::<Test>::NotDataOwner,
		);
	});
}

#[test]
fn cannot_settle_with_invalid_preimage() {
	new_test_ext().execute_with(|| {
		let secret = b"secret".to_vec();
		let anchor = hash_n_times(&secret, 5);
		open_ready_escrow(anchor, 5, 100, 300);

		assert_noop!(
			MainEscrow::settle_by_preimage(RuntimeOrigin::signed(2), 0, b"wrong".to_vec(), 3,),
			Error::<Test>::InvalidHashPreimage,
		);
	});
}
