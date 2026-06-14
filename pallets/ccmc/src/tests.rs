use crate::{mock::*, Error, Event};
use frame_support::{assert_noop, assert_ok};

// ── 子链注册 ─────────────────────────────────────────────────────────────────

#[test]
fn register_child_chain_works() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"test-chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		let chain = crate::ChildChains::<Test>::get(0).unwrap();
		assert_eq!(chain.creator, 1);
		assert_eq!(chain.miner_count, 1);
		// creator 押金已被 reserve
		assert_eq!(Balances::reserved_balance(1), 1_000);
		System::assert_has_event(Event::ChainRegistered { chain_id: 0, creator: 1 }.into());
	});
}

#[test]
fn chain_id_increments() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain-a".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(2),
			b"chain-b".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		assert!(crate::ChildChains::<Test>::get(0).is_some());
		assert!(crate::ChildChains::<Test>::get(1).is_some());
	});
}

// ── 矿工管理 ─────────────────────────────────────────────────────────────────

#[test]
fn join_and_leave_child_chain() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		// 矿工2加入
		assert_ok!(Ccmc::join_child_chain(RuntimeOrigin::signed(2), 0));
		assert_eq!(Balances::reserved_balance(2), 1_000);
		assert_eq!(crate::ChildChains::<Test>::get(0).unwrap().miner_count, 2);

		// 矿工2退出
		assert_ok!(Ccmc::leave_child_chain(RuntimeOrigin::signed(2), 0));
		assert_eq!(Balances::reserved_balance(2), 0);
		assert_eq!(crate::ChildChains::<Test>::get(0).unwrap().miner_count, 1);
	});
}

#[test]
fn join_twice_fails() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		assert_noop!(
			Ccmc::join_child_chain(RuntimeOrigin::signed(1), 0),
			Error::<Test>::AlreadyAMiner
		);
	});
}

#[test]
fn non_miner_cannot_leave() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		assert_noop!(
			Ccmc::leave_child_chain(RuntimeOrigin::signed(2), 0),
			Error::<Test>::NotAMiner
		);
	});
}

// ── Epoch 摘要投票 ────────────────────────────────────────────────────────────

#[test]
fn single_miner_confirms_epoch_immediately() {
	new_test_ext().execute_with(|| {
		// min_miners=1，单矿工即可确认
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		let root = sp_core::H256::from_low_u64_be(42);
		assert_ok!(Ccmc::submit_epoch_digest(RuntimeOrigin::signed(1), 0, 0, root));
		// 应已确认
		assert_eq!(crate::EpochDigests::<Test>::get(0, 0), Some(root));
		System::assert_has_event(
			Event::EpochDigestConfirmed { chain_id: 0, epoch: 0, root }.into(),
		);
	});
}

#[test]
fn two_of_three_miners_confirm_epoch() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			2,
			1_000,
		));
		assert_ok!(Ccmc::join_child_chain(RuntimeOrigin::signed(2), 0));
		assert_ok!(Ccmc::join_child_chain(RuntimeOrigin::signed(3), 0));
		// 3个矿工，2/3阈值 = 2

		let root = sp_core::H256::from_low_u64_be(99);
		// 第1票：未确认
		assert_ok!(Ccmc::submit_epoch_digest(RuntimeOrigin::signed(1), 0, 0, root));
		assert!(crate::EpochDigests::<Test>::get(0, 0).is_none());
		// 第2票：达到阈值，确认
		assert_ok!(Ccmc::submit_epoch_digest(RuntimeOrigin::signed(2), 0, 0, root));
		assert_eq!(crate::EpochDigests::<Test>::get(0, 0), Some(root));
	});
}

#[test]
fn duplicate_vote_rejected() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			2,
			1_000,
		));
		assert_ok!(Ccmc::join_child_chain(RuntimeOrigin::signed(2), 0));
		let root = sp_core::H256::from_low_u64_be(1);
		assert_ok!(Ccmc::submit_epoch_digest(RuntimeOrigin::signed(1), 0, 0, root));
		assert_noop!(
			Ccmc::submit_epoch_digest(RuntimeOrigin::signed(1), 0, 0, root),
			Error::<Test>::AlreadyVoted
		);
	});
}

#[test]
fn non_miner_cannot_submit_digest() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		let root = sp_core::H256::from_low_u64_be(1);
		assert_noop!(
			Ccmc::submit_epoch_digest(RuntimeOrigin::signed(2), 0, 0, root),
			Error::<Test>::NotAMiner
		);
	});
}

// ── 子链终止 ─────────────────────────────────────────────────────────────────

#[test]
fn creator_can_terminate_chain() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		assert_ok!(Ccmc::terminate_child_chain(RuntimeOrigin::signed(1), 0));
		let chain = crate::ChildChains::<Test>::get(0).unwrap();
		assert_eq!(chain.status, crate::types::ChainStatus::Terminated);
	});
}

#[test]
fn terminated_chain_rejects_new_members() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		assert_ok!(Ccmc::terminate_child_chain(RuntimeOrigin::signed(1), 0));
		assert_noop!(
			Ccmc::join_child_chain(RuntimeOrigin::signed(2), 0),
			Error::<Test>::ChainTerminated
		);
	});
}

// ── 公开接口 ─────────────────────────────────────────────────────────────────

#[test]
fn is_miner_and_epoch_root_query() {
	new_test_ext().execute_with(|| {
		assert_ok!(Ccmc::register_child_chain(
			RuntimeOrigin::signed(1),
			b"chain".to_vec().try_into().unwrap(),
			1,
			1_000,
		));
		assert!(Ccmc::is_miner(0, &1));
		assert!(!Ccmc::is_miner(0, &2));
		assert!(Ccmc::epoch_root_confirmed(0, 0).is_none());

		let root = sp_core::H256::from_low_u64_be(7);
		assert_ok!(Ccmc::submit_epoch_digest(RuntimeOrigin::signed(1), 0, 0, root));
		assert_eq!(Ccmc::epoch_root_confirmed(0, 0), Some(root));
	});
}
