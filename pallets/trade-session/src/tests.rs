use crate::{
	mock::*,
	types::{ConstraintKind, ProofSystem, RoundStatus, SessionStatus, TradeSettlementMode},
	Error, Rounds, Sessions,
};
use frame_support::{assert_noop, assert_ok};

// ── Helpers ─────────────────────────────────────────────────────────────────

fn compute_dev_digest(
	session_id: u32,
	round: u32,
	ch_proof_hash: sp_core::H256,
	ro_proof_hash: sp_core::H256,
	public_input_hash: sp_core::H256,
	vk_hash: sp_core::H256,
) -> sp_core::H256 {
	let business_input_hash = sp_core::H256::repeat_byte(99);
	crate::proof::compute_zk_proof_digest::<<Test as frame_system::Config>::Hashing>(
		ProofSystem::GnarkGroth16Bn254,
		ConstraintKind::Range,
		10,
		sp_core::H256::repeat_byte(4), // request_hash from create_session_helper
		session_id,
		round,
		vk_hash,
		ch_proof_hash,
		ro_proof_hash,
		public_input_hash,
		business_input_hash,
	)
}

fn compute_dev_attestation(
	session_id: u32,
	round: u32,
	proof_digest: sp_core::H256,
) -> sp_core::H256 {
	// SCALE encoding of u64 account 3: [3, 0, 0, 0, 0, 0, 0, 0]
	let verifier_encoded: [u8; 8] = 3u64.to_le_bytes();
	crate::proof::compute_zk_attestation_digest::<<Test as frame_system::Config>::Hashing>(
		session_id,
		round,
		proof_digest,
		true,
		&verifier_encoded,
	)
}

// ── Tests ───────────────────────────────────────────────────────────────────

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
	});
}

#[test]
fn create_session_rejects_missing_listing() {
	new_test_ext().execute_with(|| {
		assert_noop!(
			crate::Pallet::<Test>::create_session(
				frame_system::RawOrigin::Signed(1).into(),
				99,
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
		assert_noop!(
			crate::Pallet::<Test>::submit_payment_proof(
				frame_system::RawOrigin::Signed(1).into(),
				0,
				0,
				sp_core::H256::repeat_byte(1),
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
		assert_ok!(crate::Pallet::<Test>::claim_settlement(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			sp_core::H256::repeat_byte(42),
			4,
		));
		assert_eq!(Sessions::<Test>::get(0).unwrap().status, SessionStatus::SettlementClaimed);
	});
}

#[test]
fn dr_can_dispute_invalid_proof_and_mark_session_punished() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);
		let ch = sp_core::H256::repeat_byte(1);

		let digest = compute_dev_digest(0, 0, ch, ch, ch, ch);
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
			ProofSystem::GnarkGroth16Bn254,
			ConstraintKind::Range,
			10,
			ch,
			ch,
			ch,
			ch,
			sp_core::H256::repeat_byte(99),
			digest,
		));
		assert_ok!(crate::Pallet::<Test>::dispute_invalid_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			sp_core::H256::repeat_byte(99),
		));
		assert_eq!(Sessions::<Test>::get(0).unwrap().status, SessionStatus::Punished);
	});
}

#[test]
fn do_can_claim_last_payment_after_signature_and_delivery() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);
		let ch = sp_core::H256::repeat_byte(1);

		let digest = compute_dev_digest(0, 0, ch, ch, ch, ch);
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
			ProofSystem::GnarkGroth16Bn254,
			ConstraintKind::Range,
			10,
			ch,
			ch,
			ch,
			ch,
			sp_core::H256::repeat_byte(99),
			digest,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			digest,
			true,
			compute_dev_attestation(0, 0, digest),
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
		assert_ok!(crate::Pallet::<Test>::claim_last_payment(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
		));
		assert_eq!(Sessions::<Test>::get(0).unwrap().status, SessionStatus::SettlementClaimed);
	});
}

#[test]
fn dr_can_dispute_plaintext_hash_mismatch() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);
		let ch = sp_core::H256::repeat_byte(1);

		let digest = compute_dev_digest(0, 0, ch, ch, ch, ch);
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
			ProofSystem::GnarkGroth16Bn254,
			ConstraintKind::Range,
			10,
			ch,
			ch,
			ch,
			ch,
			sp_core::H256::repeat_byte(99),
			digest,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			digest,
			true,
			compute_dev_attestation(0, 0, digest),
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
		assert_ok!(crate::Pallet::<Test>::dispute_invalid_plaintext(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			sp_core::H256::repeat_byte(99),
			sp_core::H256::repeat_byte(100),
		));
		assert_eq!(Sessions::<Test>::get(0).unwrap().status, SessionStatus::Punished);
	});
}

#[test]
fn claim_settlement_rejects_more_paid_rounds_than_completed() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);
		complete_round(0, 0);
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

		let digest = compute_dev_digest(0, 0, ch, ch, ch, ch);
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
			ProofSystem::GnarkGroth16Bn254,
			ConstraintKind::Range,
			10,
			ch,
			ch,
			ch,
			ch,
			sp_core::H256::repeat_byte(99),
			digest,
		));
		assert_noop!(
			crate::Pallet::<Test>::attest_data_proof(
				frame_system::RawOrigin::Signed(4).into(),
				0,
				0,
				digest,
				true,
				compute_dev_attestation(0, 0, digest),
			),
			Error::<Test>::NotVerifier,
		);
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			digest,
			true,
			compute_dev_attestation(0, 0, digest),
		));
	});
}

#[test]
fn rejected_attestation_cannot_be_signed_by_requester() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);
		let ch = sp_core::H256::repeat_byte(1);

		let digest = compute_dev_digest(0, 0, ch, ch, ch, ch);
		let rejected_att = crate::proof::compute_zk_attestation_digest::<
			<Test as frame_system::Config>::Hashing,
		>(0, 0, digest, false, &3u64.to_le_bytes());
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
			ProofSystem::GnarkGroth16Bn254,
			ConstraintKind::Range,
			10,
			ch,
			ch,
			ch,
			ch,
			sp_core::H256::repeat_byte(99),
			digest,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			digest,
			false,
			rejected_att,
		));
		assert_noop!(
			crate::Pallet::<Test>::submit_proof_signature(
				frame_system::RawOrigin::Signed(1).into(),
				0,
				0,
				ch,
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

		let digest = compute_dev_digest(0, 0, ch, ch, ch, ch);
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
			ProofSystem::GnarkGroth16Bn254,
			ConstraintKind::Range,
			10,
			ch,
			ch,
			ch,
			ch,
			sp_core::H256::repeat_byte(99),
			digest,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			digest,
			true,
			compute_dev_attestation(0, 0, digest),
		));
		assert_ok!(crate::Pallet::<Test>::dispute_invalid_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			sp_core::H256::repeat_byte(99),
		));
		assert_eq!(Sessions::<Test>::get(0).unwrap().status, SessionStatus::Punished);
	});
}

// ── Task 4: ZK proof binding tests ──────────────────────────────────────────

#[test]
fn submit_data_proof_records_bound_metadata() {
	new_test_ext().execute_with(|| {
		let session_id = setup_accepted_session();
		let round = 0;
		let payment_hash = sp_core::H256::repeat_byte(10);
		let ch_proof_hash = sp_core::H256::repeat_byte(11);
		let ro_proof_hash = sp_core::H256::repeat_byte(12);
		let public_input_hash = sp_core::H256::repeat_byte(13);
		let vk_hash = sp_core::H256::repeat_byte(14);
		let proof_digest = compute_dev_digest(
			session_id,
			round,
			ch_proof_hash,
			ro_proof_hash,
			public_input_hash,
			vk_hash,
		);

		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			session_id,
			round,
			payment_hash,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			session_id,
			round,
			payment_hash,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(),
			session_id,
			round,
			ProofSystem::GnarkGroth16Bn254,
			ConstraintKind::Range,
			10,
			ch_proof_hash,
			ro_proof_hash,
			public_input_hash,
			vk_hash,
			sp_core::H256::repeat_byte(99),
			proof_digest,
		));

		let state = Rounds::<Test>::get(session_id, round).unwrap();
		assert_eq!(state.proof_hash, Some(proof_digest));
		assert_eq!(state.ch_proof_hash, Some(ch_proof_hash));
		assert_eq!(state.ro_proof_hash, Some(ro_proof_hash));
		assert_eq!(state.public_input_hash, Some(public_input_hash));
		assert_eq!(state.vk_hash, Some(vk_hash));
	});
}

#[test]
fn submit_data_proof_rejects_digest_not_bound_to_session() {
	new_test_ext().execute_with(|| {
		let session_id = setup_accepted_session();
		let round = 0;
		let payment_hash = sp_core::H256::repeat_byte(10);

		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			session_id,
			round,
			payment_hash,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			session_id,
			round,
			payment_hash,
		));

		// Wrong digest — computed with different session_id
		let wrong_digest =
			compute_dev_digest(99, round, payment_hash, payment_hash, payment_hash, payment_hash);
		assert_noop!(
			crate::Pallet::<Test>::submit_data_proof(
				frame_system::RawOrigin::Signed(2).into(),
				session_id,
				round,
				ProofSystem::GnarkGroth16Bn254,
				ConstraintKind::Range,
				10,
				payment_hash,
				payment_hash,
				payment_hash,
				payment_hash,
				sp_core::H256::repeat_byte(99),
				wrong_digest,
			),
			Error::<Test>::InvalidProof,
		);
	});
}

#[test]
fn verifier_attestation_must_match_payload() {
	new_test_ext().execute_with(|| {
		let session_id = setup_session_with_submitted_data_proof();
		let round = 0;
		let proof_digest = Rounds::<Test>::get(session_id, round).unwrap().proof_hash.unwrap();
		let wrong_attestation = sp_core::H256::repeat_byte(88);
		assert_noop!(
			crate::Pallet::<Test>::attest_data_proof(
				frame_system::RawOrigin::Signed(3).into(),
				session_id,
				round,
				proof_digest,
				true,
				wrong_attestation,
			),
			Error::<Test>::InvalidAttestation,
		);
	});
}
