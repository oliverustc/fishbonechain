use crate::{mock::*, Error, Event};
use frame_support::{assert_noop, assert_ok};

// ── 充值与提款 ────────────────────────────────────────────────────────────────

#[test]
fn deposit_and_withdraw_works() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 50_000));
		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.free, 50_000);
		assert_eq!(pool.locked, 0);

		assert_ok!(Fmc::withdraw(RuntimeOrigin::signed(1), 20_000));
		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.free, 30_000);
	});
}

#[test]
fn withdraw_more_than_free_fails() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 10_000));
		assert_noop!(
			Fmc::withdraw(RuntimeOrigin::signed(1), 20_000),
			Error::<Test>::InsufficientFreeBalance
		);
	});
}

#[test]
fn withdraw_without_deposit_fails() {
	new_test_ext().execute_with(|| {
		assert_noop!(Fmc::withdraw(RuntimeOrigin::signed(1), 1), Error::<Test>::NoFundPool);
	});
}

// ── 任务创建与激活 ────────────────────────────────────────────────────────────

#[test]
fn create_and_activate_task() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 100_000));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,      // target_chain
			10_000, // budget_per_epoch
			b"test task".to_vec().try_into().unwrap(),
		));

		// 激活前：FB = 100_000，LB = 0
		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.free, 100_000);
		assert_eq!(pool.locked, 0);

		assert_ok!(Fmc::activate_task(RuntimeOrigin::signed(1), 0));

		// 激活后：FB = 90_000，LB = 10_000
		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.free, 90_000);
		assert_eq!(pool.locked, 10_000);

		let task = crate::Tasks::<Test>::get(1, 0).unwrap();
		assert_eq!(task.status, crate::types::TaskStatus::Activated);

		System::assert_has_event(Event::TaskActivated { requester: 1, task_id: 0 }.into());
	});
}

#[test]
fn activate_fails_when_insufficient_free_balance() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 5_000));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			10_000,
			b"task".to_vec().try_into().unwrap(),
		));
		assert_noop!(
			Fmc::activate_task(RuntimeOrigin::signed(1), 0),
			Error::<Test>::InsufficientFreeBalance
		);
	});
}

// ── 双花防护 ─────────────────────────────────────────────────────────────────

#[test]
fn double_spend_protection() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 15_000));

		// 创建两个任务，各需 10_000
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			10_000,
			b"task-a".to_vec().try_into().unwrap(),
		));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			10_000,
			b"task-b".to_vec().try_into().unwrap(),
		));

		// 激活第一个任务成功（FB: 15000 → 5000，LB: 10000）
		assert_ok!(Fmc::activate_task(RuntimeOrigin::signed(1), 0));

		// 激活第二个任务失败（FB=5000 < 10000）
		assert_noop!(
			Fmc::activate_task(RuntimeOrigin::signed(1), 1),
			Error::<Test>::InsufficientFreeBalance
		);
	});
}

// ── 任务终止 ─────────────────────────────────────────────────────────────────

#[test]
fn terminate_task_returns_locked_to_free() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 100_000));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			30_000,
			b"task".to_vec().try_into().unwrap(),
		));
		assert_ok!(Fmc::activate_task(RuntimeOrigin::signed(1), 0));

		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.free, 70_000);
		assert_eq!(pool.locked, 30_000);

		assert_ok!(Fmc::terminate_task(RuntimeOrigin::signed(1), 0));

		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.free, 100_000);
		assert_eq!(pool.locked, 0);
	});
}

// ── 账单结算 ─────────────────────────────────────────────────────────────────

#[test]
fn submit_bill_settles_and_pays_recipients() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 100_000));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			10_000,
			b"task".to_vec().try_into().unwrap(),
		));
		assert_ok!(Fmc::activate_task(RuntimeOrigin::signed(1), 0));

		let initial_balance_2 = Balances::free_balance(2);
		let initial_balance_3 = Balances::free_balance(3);

		let bill: frame_support::BoundedVec<_, _> =
			vec![(2u64, 6_000u64), (3u64, 3_000u64)].try_into().unwrap();

		// 矿工2提交账单（MockCcmc 中任何账户都是矿工，单矿工即达阈值）
		assert_ok!(Fmc::submit_bill(RuntimeOrigin::signed(2), 1, 0, 0, bill));

		// 验证接收方收到款项
		assert_eq!(Balances::free_balance(2), initial_balance_2 + 6_000);
		assert_eq!(Balances::free_balance(3), initial_balance_3 + 3_000);

		// 验证 FMC 资金池：LB 扣除预算，剩余 1000 归还 FB
		// 支付后 FB=90000+1000=91000，若 FB≥B 任务自动续期，LB=10000
		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.locked, 10_000); // 自动续期，重新锁定
		assert_eq!(pool.free, 81_000); // 91000 - 10000
	});
}

#[test]
fn bill_exceeding_budget_fails() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 100_000));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			10_000,
			b"task".to_vec().try_into().unwrap(),
		));
		assert_ok!(Fmc::activate_task(RuntimeOrigin::signed(1), 0));

		let bill: frame_support::BoundedVec<_, _> = vec![(2u64, 15_000u64)].try_into().unwrap(); // 超预算
		assert_noop!(
			Fmc::submit_bill(RuntimeOrigin::signed(2), 1, 0, 0, bill),
			Error::<Test>::ExceedsBudget
		);
	});
}

#[test]
fn task_terminates_when_insufficient_fb_after_settlement() {
	new_test_ext().execute_with(|| {
		// 充值恰好一个 epoch 的预算（账单仅消耗预算的大部分，剩余 1 单位归还 FB）
		// 结算后 FB=1 < B=10_000，任务应自动 Terminated
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 10_000));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			10_000,
			b"task".to_vec().try_into().unwrap(),
		));
		assert_ok!(Fmc::activate_task(RuntimeOrigin::signed(1), 0));

		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.free, 0);
		assert_eq!(pool.locked, 10_000);

		// 账单消耗 9_999，剩余 1 归还 FB；FB=1 < budget=10_000，不续期
		let bill: frame_support::BoundedVec<_, _> = vec![(2u64, 9_999u64)].try_into().unwrap();
		assert_ok!(Fmc::submit_bill(RuntimeOrigin::signed(2), 1, 0, 0, bill));

		let task = crate::Tasks::<Test>::get(1, 0).unwrap();
		assert_eq!(task.status, crate::types::TaskStatus::Terminated);

		let pool = crate::FundPools::<Test>::get(1).unwrap();
		assert_eq!(pool.locked, 0);
		assert_eq!(pool.free, 1); // 剩余 1 单位归还 FB
	});
}

#[test]
fn bill_on_terminated_task_fails() {
	new_test_ext().execute_with(|| {
		assert_ok!(Fmc::deposit(RuntimeOrigin::signed(1), 100_000));
		assert_ok!(Fmc::create_task(
			RuntimeOrigin::signed(1),
			0,
			10_000,
			b"task".to_vec().try_into().unwrap(),
		));
		// 未激活的任务
		let bill: frame_support::BoundedVec<_, _> = vec![(2u64, 5_000u64)].try_into().unwrap();
		assert_noop!(
			Fmc::submit_bill(RuntimeOrigin::signed(2), 1, 0, 0, bill),
			Error::<Test>::TaskNotActive
		);
	});
}
