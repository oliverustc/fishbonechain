use crate::{mock::*, Error, Event};
use crate::types::{EpochPhase, TaskStatus};
use frame_support::{assert_noop, assert_ok, BoundedVec};

// ── Epoch 生命周期 ────────────────────────────────────────────────────────────

#[test]
fn collecting_slot_transitions_to_syncing_at_block_100() {
    new_test_ext().execute_with(|| {
        // 初始：Collecting，start_block=0（创世块）
        assert_eq!(Crowdsource::current_epoch().phase, EpochPhase::Collecting);

        // block 100 时触发 on_initialize：elapsed=100 >= CollectingSlot=100
        run_to_block(100);
        assert_eq!(Crowdsource::current_epoch().phase, EpochPhase::Syncing);
        System::assert_has_event(
            Event::SyncingSlotStarted { epoch: 0, block: 100 }.into(),
        );
    });
}

#[test]
fn epoch_finalizes_at_block_120() {
    new_test_ext().execute_with(|| {
        run_to_block(120);
        // elapsed=120 >= 100+20 → EpochFinalized 并重置
        let epoch = Crowdsource::current_epoch();
        assert_eq!(epoch.epoch_id, 1);
        assert_eq!(epoch.phase, EpochPhase::Collecting);

        // 空 epoch 也应该发出 EpochFinalized 事件
        let events = System::events();
        let has_finalized = events.iter().any(|e| {
            matches!(e.event, RuntimeEvent::Crowdsource(Event::EpochFinalized { epoch: 0, .. }))
        });
        assert!(has_finalized, "期望 EpochFinalized 事件");
    });
}

#[test]
fn syncing_slot_rejects_data_submission() {
    new_test_ext().execute_with(|| {
        // 同步一个任务
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 10_000, BoundedVec::default()
        ));
        // 推进到 S_s
        run_to_block(100);
        assert_eq!(Crowdsource::current_epoch().phase, EpochPhase::Syncing);

        // S_s 阶段提交数据应失败
        assert_noop!(
            Crowdsource::submit_data(
                RuntimeOrigin::signed(2), 0,
                b"data".to_vec().try_into().unwrap(), 100,
            ),
            Error::<Test>::NotInCollectingSlot
        );
    });
}

// ── 任务管理 ─────────────────────────────────────────────────────────────────

#[test]
fn sync_task_works() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1),
            42,
            2,
            50_000,
            b"deliver package".to_vec().try_into().unwrap(),
        ));
        let task = crate::ActiveTasks::<Test>::get(42).unwrap();
        assert_eq!(task.requester, 2);
        assert_eq!(task.budget_per_epoch, 50_000);
        assert_eq!(task.status, TaskStatus::Active);
        System::assert_has_event(Event::TaskSynced { task_id: 42 }.into());
    });
}

#[test]
fn submit_data_on_unknown_task_fails() {
    new_test_ext().execute_with(|| {
        assert_noop!(
            Crowdsource::submit_data(
                RuntimeOrigin::signed(2), 99,
                b"data".to_vec().try_into().unwrap(), 100,
            ),
            Error::<Test>::TaskNotFound
        );
    });
}

// ── 数据提交 ─────────────────────────────────────────────────────────────────

#[test]
fn submit_data_accumulates_spent_budget() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 10_000, BoundedVec::default()
        ));
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(2), 0, b"d1".to_vec().try_into().unwrap(), 3_000
        ));
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(3), 0, b"d2".to_vec().try_into().unwrap(), 4_000
        ));
        assert_eq!(crate::SpentBudget::<Test>::get(0), 7_000);
    });
}

#[test]
fn submit_data_exceeds_budget_fails() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 5_000, BoundedVec::default()
        ));
        assert_noop!(
            Crowdsource::submit_data(
                RuntimeOrigin::signed(2), 0, b"x".to_vec().try_into().unwrap(), 6_000
            ),
            Error::<Test>::ExceedsBudget
        );
    });
}

#[test]
fn budget_exhaustion_marks_task_and_rejects_further_submissions() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 1_000, BoundedVec::default()
        ));
        // 消耗全部预算
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(2), 0, b"x".to_vec().try_into().unwrap(), 1_000
        ));
        assert_eq!(
            crate::ActiveTasks::<Test>::get(0).unwrap().status,
            TaskStatus::Exhausted
        );
        // 后续提交应失败
        assert_noop!(
            Crowdsource::submit_data(
                RuntimeOrigin::signed(3), 0, b"y".to_vec().try_into().unwrap(), 1
            ),
            Error::<Test>::BudgetExhausted
        );
    });
}

// ── Epoch 结算 ────────────────────────────────────────────────────────────────

#[test]
fn manual_finalize_epoch_succeeds_in_syncing_slot() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 10_000, BoundedVec::default()
        ));
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(2), 0, b"hello".to_vec().try_into().unwrap(), 500
        ));

        run_to_block(100); // 进入 S_s
        assert_ok!(Crowdsource::finalize_epoch(RuntimeOrigin::signed(1)));

        // epoch_id 递增
        assert_eq!(Crowdsource::current_epoch().epoch_id, 1);
        // 提交记录已清空
        assert_eq!(crate::EpochSubmissions::<Test>::get().len(), 0);
    });
}

#[test]
fn finalize_epoch_in_collecting_slot_fails() {
    new_test_ext().execute_with(|| {
        assert_noop!(
            Crowdsource::finalize_epoch(RuntimeOrigin::signed(1)),
            Error::<Test>::NotInSyncingSlot
        );
    });
}

#[test]
fn epoch_merkle_root_stored_after_finalization() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 10_000, BoundedVec::default()
        ));
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(2), 0, b"data".to_vec().try_into().unwrap(), 100
        ));
        run_to_block(120); // 自动结算
        // EpochRoots[0] 应已写入（非零值，有提交）
        let root = crate::EpochRoots::<Test>::get(0);
        assert!(root.is_some(), "Merkle root 应被存储");
    });
}

#[test]
fn bill_amounts_aggregated_per_worker() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 10_000, BoundedVec::default()
        ));
        // worker 2 提交 2 次
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(2), 0, b"a".to_vec().try_into().unwrap(), 300
        ));
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(2), 0, b"b".to_vec().try_into().unwrap(), 400
        ));
        // worker 3 提交 1 次
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(3), 0, b"c".to_vec().try_into().unwrap(), 200
        ));

        run_to_block(120); // 自动结算

        let events = System::events();
        let finalized = events.iter().find_map(|e| {
            if let RuntimeEvent::Crowdsource(Event::EpochFinalized { bill_amounts, .. }) = &e.event {
                Some(bill_amounts.clone())
            } else {
                None
            }
        }).expect("应有 EpochFinalized 事件");

        // worker 2 合计 700，worker 3 合计 200
        let bill_map: std::collections::HashMap<u64, u64> =
            finalized.iter().cloned().collect();
        assert_eq!(bill_map.get(&2), Some(&700));
        assert_eq!(bill_map.get(&3), Some(&200));
    });
}

#[test]
fn epoch_resets_exhausted_tasks_after_finalization() {
    new_test_ext().execute_with(|| {
        assert_ok!(Crowdsource::sync_task(
            RuntimeOrigin::signed(1), 0, 1, 500, BoundedVec::default()
        ));
        assert_ok!(Crowdsource::submit_data(
            RuntimeOrigin::signed(2), 0, b"x".to_vec().try_into().unwrap(), 500
        ));
        assert_eq!(
            crate::ActiveTasks::<Test>::get(0).unwrap().status,
            TaskStatus::Exhausted
        );
        run_to_block(120);
        // 下一个 epoch 任务状态重置为 Active
        assert_eq!(
            crate::ActiveTasks::<Test>::get(0).unwrap().status,
            TaskStatus::Active
        );
    });
}

#[test]
fn empty_epoch_produces_zero_merkle_root() {
    new_test_ext().execute_with(|| {
        run_to_block(120);
        // 无提交时 root 为零值
        let root = crate::EpochRoots::<Test>::get(0).unwrap_or_default();
        assert_eq!(root, Default::default());
    });
}
