use crate::{
	mock::*,
	types::{RoundStatus, SessionStatus, TradeSettlementMode},
	Error, Rounds, Sessions,
};
use frame_support::{assert_noop, assert_ok};

#[test]
fn create_session_binds_listing_and_escrow() {
	new_test_ext().execute_with(|| {
		create_session_helper();

		let session = Sessions::<Test>::get(0).expect("session exists");
		assert_eq!(session.listing_id, 0);
		assert_eq!(session.escrow_id, 42);
		assert_eq!(session.requester, 1);
		assert_eq!(session.data_owner, 2);
		assert_eq!(session.status, SessionStatus::Requested);
		assert_eq!(session.settlement_mode, TradeSettlementMode::MainEscrow);
	});
}

#[test]
fn create_session_rejects_missing_listing() {
	new_test_ext().execute_with(|| {
		assert_noop!(
			crate::Pallet::<Test>::create_session(
				frame_system::RawOrigin::Signed(1).into(),
				99,  // non-existent listing
				42,
				2,
				sp_core::H256::repeat_byte(4),
				100,
				5,
				sp_core::H256::repeat_byte(9),
				TradeSettlementMode::MainEscrow,
			),
			Error::<Test>::ListingNotFound,
		);
	});
}

#[test]
fn dob_can_accept_session() {
	new_test_ext().execute_with(|| {
		create_session_helper();

		assert_ok!(crate::Pallet::<Test>::accept_session(
			frame_system::RawOrigin::Signed(2).into(),
			0,
		));

		assert_eq!(Sessions::<Test>::get(0).unwrap().status, SessionStatus::Accepted);
	});
}

#[test]
fn wrong_actor_cannot_advance_round() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		// Non-requester cannot open round
		assert_noop!(
			crate::Pallet::<Test>::open_round(
				frame_system::RawOrigin::Signed(2).into(),
				0,
				0,
				sp_core::H256::repeat_byte(1),
			),
			Error::<Test>::NotRequester,
		);
	});
}

#[test]
fn round_steps_must_be_in_order() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);

		// Submit payment proof before opening round should fail (session not InDelivery yet)
		assert_noop!(
			crate::Pallet::<Test>::submit_payment_proof(
				frame_system::RawOrigin::Signed(1).into(),
				0,
				0,
				ch,
			),
			Error::<Test>::InvalidSessionStatus,
		);
	});
}

#[test]
fn happy_path_one_round() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		complete_round(0, 0);

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.completed_rounds, 1);
		assert_eq!(session.status, SessionStatus::InDelivery);

		let round = Rounds::<Test>::get(0, 0).unwrap();
		assert_eq!(round.status, RoundStatus::Completed);
	});
}

#[test]
fn do_can_claim_settlement_after_rounds() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		complete_round(0, 0);

		// DO claims settlement with 4 remaining rounds (1 completed)
		assert_ok!(crate::Pallet::<Test>::claim_settlement(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			sp_core::H256::repeat_byte(42),
			4,
		));

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.status, SessionStatus::SettlementClaimed);
	});
}

#[test]
fn dr_can_dispute_invalid_proof_and_mark_session_punished() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		// Open round and submit payment proof
		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));

		// DO submits data proof (passes mock verifier)
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));

		// DR disputes (receives bad data off-chain)
		assert_ok!(crate::Pallet::<Test>::dispute_invalid_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			sp_core::H256::repeat_byte(99),
		));

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.status, SessionStatus::Punished);
	});
}

#[test]
fn do_can_claim_last_payment_after_signature_and_delivery() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		// Complete up to DataDelivered (but NOT PaymentPreimage)
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			ch,
			true,
		));
		assert_ok!(crate::Pallet::<Test>::submit_proof_signature(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_delivery_hash(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));

		// DR refuses to pay — DO claims last payment
		assert_ok!(crate::Pallet::<Test>::claim_last_payment(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
		));

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.status, SessionStatus::SettlementClaimed);
	});
}

#[test]
fn dr_can_dispute_plaintext_hash_mismatch() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			ch,
			true,
		));
		assert_ok!(crate::Pallet::<Test>::submit_proof_signature(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		// DO delivers data with hash that doesn't match expected
		assert_ok!(crate::Pallet::<Test>::submit_data_delivery_hash(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));

		// DR disputes: actual hash != expected
		assert_ok!(crate::Pallet::<Test>::dispute_invalid_plaintext(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			sp_core::H256::repeat_byte(99), // actual
			sp_core::H256::repeat_byte(100), // expected (different)
		));

		let session = Sessions::<Test>::get(0).unwrap();
		assert_eq!(session.status, SessionStatus::Punished);
	});
}

#[test]
fn claim_settlement_rejects_more_paid_rounds_than_completed() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		complete_round(0, 0);

		// max_rounds=5, completed=1, remaining=1 => claimed=4 > 1 => reject
		assert_noop!(
			crate::Pallet::<Test>::claim_settlement(
				frame_system::RawOrigin::Signed(2).into(),
				0,
				sp_core::H256::repeat_byte(42),
				1,
			),
			Error::<Test>::SettlementRoundsExceedCompleted,
		);
	});
}

#[test]
fn only_authorized_verifier_can_attest_data_proof() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(), 0, 0, ch,
		));

		assert_noop!(
			crate::Pallet::<Test>::attest_data_proof(
				frame_system::RawOrigin::Signed(4).into(), 0, 0, ch, true,
			),
			Error::<Test>::NotVerifier,
		);
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(), 0, 0, ch, true,
		));
	});
}

#[test]
fn rejected_attestation_cannot_be_signed_by_requester() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(), 0, 0, ch, false,
		));

		assert_noop!(
			crate::Pallet::<Test>::submit_proof_signature(
				frame_system::RawOrigin::Signed(1).into(), 0, 0, ch,
			),
			Error::<Test>::RoundStepsOutOfOrder,
		);
	});
}

#[test]
fn requester_can_dispute_after_verifier_accepts_proof() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(), 0, 0, ch,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(), 0, 0, ch, true,
		));

		assert_ok!(crate::Pallet::<Test>::dispute_invalid_proof(
			frame_system::RawOrigin::Signed(1).into(), 0, 0,
			sp_core::H256::repeat_byte(99),
		));
		assert_eq!(
			Sessions::<Test>::get(0).unwrap().status,
			SessionStatus::Punished
		);
	});
}
