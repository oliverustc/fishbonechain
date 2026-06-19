//! pallet-crowdsource: 子链众包数据收集
//!
//! 实现论文 FishboneChain 中子链侧的 Epoch 生命周期：
//!   S_c（收集时隙）→ S_s（同步时隙）→ 自动结算 → 新 Epoch
//!
//! 结算产物（EpochFinalized 事件）由链下桥接脚本转发至主链 CCMC + FMC。

#![cfg_attr(not(feature = "std"), no_std)]

extern crate alloc;

pub use pallet::*;

#[cfg(test)]
mod mock;
#[cfg(test)]
mod tests;

pub mod types;

#[frame_support::pallet]
pub mod pallet {
	use alloc::{collections::BTreeMap, vec::Vec};
	use binary_merkle_tree::merkle_root;
	use codec::Encode;
	use frame_support::{
		pallet_prelude::*,
		traits::{Currency, Get},
	};
	use frame_system::pallet_prelude::*;
	use pallet_chain_profile::ChainIdentityProvider;
	use sp_runtime::traits::{Hash as HashT, SaturatedConversion, Saturating};

	use crate::types::*;

	pub type BalanceOf<T> =
		<<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

	// ── 数据验证器 ────────────────────────────────────────────────────────────

	/// 可插拔的数据验证器接口（不同子链可实现不同验证逻辑）
	pub trait ValidateSubmission<AccountId> {
		fn validate(task_id: TaskId, worker: &AccountId, data: &[u8]) -> bool;
	}

	/// 接受所有提交（测试/演示用）
	pub struct AlwaysValidate;
	impl<AccountId> ValidateSubmission<AccountId> for AlwaysValidate {
		fn validate(_: TaskId, _: &AccountId, _: &[u8]) -> bool {
			true
		}
	}

	// ── WeightInfo ───────────────────────────────────────────────────────────

	pub trait WeightInfo {
		fn sync_task() -> Weight;
		fn submit_data() -> Weight;
		fn submit_data_batch(items: u32) -> Weight;
		fn finalize_epoch() -> Weight;
	}

	impl WeightInfo for () {
		fn sync_task() -> Weight {
			Weight::from_parts(10_000, 0)
		}
		fn submit_data() -> Weight {
			Weight::from_parts(15_000, 0)
		}
		fn submit_data_batch(items: u32) -> Weight {
			Weight::from_parts(15_000, 0).saturating_mul(items.into())
		}
		fn finalize_epoch() -> Weight {
			Weight::from_parts(50_000, 0)
		}
	}

	// ── Config ───────────────────────────────────────────────────────────────

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

		/// 货币（不继承 ccmc/fmc Config，独立声明以简化 bound）
		type Currency: Currency<Self::AccountId>;

		/// 本链的 profile provider，提供 chain id、场景和结算模式。
		type ChainProfile: pallet_chain_profile::ChainIdentityProvider;

		/// 收集时隙长度（块数），epoch 前 N 块为 S_c
		#[pallet::constant]
		type CollectingSlotBlocks: Get<u32>;

		/// 同步时隙长度（块数），epoch 后 M 块为 S_s
		#[pallet::constant]
		type SyncingSlotBlocks: Get<u32>;

		/// 每个 epoch 最大提交数（防止 on_initialize 中 finalize 的 weight 失控）
		#[pallet::constant]
		type MaxSubmissionsPerEpoch: Get<u32>;

		/// 数据验证器（可替换）
		type DataValidator: ValidateSubmission<Self::AccountId>;

		/// 是否在高频提交事件中携带完整 worker 字段。
		type FullSubmissionEvents: Get<bool>;

		/// 是否使用索引/聚合存储替代完整提交 Vec 的热路径 append。
		type IndexedSubmissionStorage: Get<bool>;

		/// 单个批量业务提交 extrinsic 允许携带的最大业务提交数。
		#[pallet::constant]
		type MaxBatchSize: Get<u32>;

		type WeightInfo: WeightInfo;
	}

	// ── 存储 ─────────────────────────────────────────────────────────────────

	/// 从主链同步来的激活任务
	#[pallet::storage]
	pub type ActiveTasks<T: Config> =
		StorageMap<_, Blake2_128Concat, TaskId, TaskDetail<T::AccountId, BalanceOf<T>>>;

	/// 本 epoch 的所有提交记录
	#[pallet::storage]
	pub type EpochSubmissions<T: Config> = StorageValue<
		_,
		BoundedVec<Submission<T::AccountId, BalanceOf<T>>, T::MaxSubmissionsPerEpoch>,
		ValueQuery,
	>;

	/// 当前 epoch 已接受的业务提交数量。压测监控优先读取该计数。
	#[pallet::storage]
	pub type AcceptedSubmissionCount<T: Config> = StorageValue<_, u32, ValueQuery>;

	/// v2/v3 profile: 每个 task/epoch 的业务提交数量。
	#[pallet::storage]
	pub type SubmissionCountByTaskEpoch<T: Config> =
		StorageDoubleMap<_, Blake2_128Concat, TaskId, Blake2_128Concat, EpochId, u32, ValueQuery>;

	/// v2/v3 profile: 每个 task/epoch/index 的提交摘要。
	#[pallet::storage]
	pub type SubmissionDigestByTaskEpochIndex<T: Config> = StorageDoubleMap<
		_,
		Blake2_128Concat,
		(TaskId, EpochId),
		Blake2_128Concat,
		u32,
		T::Hash,
		OptionQuery,
	>;

	/// v2/v3 profile: 当前 epoch 的 worker 账单聚合。
	#[pallet::storage]
	pub type EpochBills<T: Config> =
		StorageMap<_, Blake2_128Concat, T::AccountId, BalanceOf<T>, ValueQuery>;

	/// 各任务本 epoch 已消耗的预算
	#[pallet::storage]
	pub type SpentBudget<T: Config> =
		StorageMap<_, Blake2_128Concat, TaskId, BalanceOf<T>, ValueQuery>;

	/// 当前 Epoch 状态
	#[pallet::storage]
	pub type CurrentEpoch<T: Config> = StorageValue<_, EpochInfo, ValueQuery>;

	/// 历史 Epoch 的 Merkle Root（链下查询用）
	#[pallet::storage]
	pub type EpochRoots<T: Config> = StorageMap<_, Blake2_128Concat, EpochId, T::Hash>;

	// ── 事件 ─────────────────────────────────────────────────────────────────

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		/// 任务从主链同步过来
		TaskSynced { task_id: TaskId },
		/// 工作者提交数据
		DataSubmitted { task_id: TaskId, worker: T::AccountId, reward: BalanceOf<T> },
		/// 工作者提交数据的轻量事件，用于高压吞吐实验 runtime profile。
		DataSubmittedCompact { task_id: TaskId, reward: BalanceOf<T> },
		/// 批量业务提交完成。accepted 表示该 extrinsic 内被接受的业务提交条数。
		DataBatchSubmitted {
			task_id: TaskId,
			worker: T::AccountId,
			accepted: u32,
			reward_each: BalanceOf<T>,
		},
		/// 进入同步时隙（S_s）
		SyncingSlotStarted { epoch: EpochId, block: u32 },
		/// Epoch 结算完成
		/// bill_amounts 携带完整账单，供链下桥接脚本转发至主链 FMC
		EpochFinalized {
			chain_id: ChainId,
			epoch: EpochId,
			merkle_root: T::Hash,
			bill_amounts: BoundedVec<(T::AccountId, BalanceOf<T>), T::MaxSubmissionsPerEpoch>,
		},
	}

	// ── 错误 ─────────────────────────────────────────────────────────────────

	#[pallet::error]
	pub enum Error<T> {
		NotInCollectingSlot,
		NotInSyncingSlot,
		TaskNotFound,
		BudgetExhausted,
		ExceedsBudget,
		InvalidData,
		SubmissionLimitReached,
		Overflow,
		EmptyBatch,
		BatchTooLarge,
	}

	// ── Pallet ───────────────────────────────────────────────────────────────

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	// ── Hooks（Epoch 自动推进）───────────────────────────────────────────────

	#[pallet::hooks]
	impl<T: Config> Hooks<BlockNumberFor<T>> for Pallet<T> {
		fn on_initialize(now: BlockNumberFor<T>) -> Weight {
			let epoch = CurrentEpoch::<T>::get();
			let now_u32 = now.saturated_into::<u32>();
			let elapsed = now_u32.saturating_sub(epoch.start_block);
			let sc = T::CollectingSlotBlocks::get();
			let ss = T::SyncingSlotBlocks::get();

			match epoch.phase {
				EpochPhase::Collecting if elapsed >= sc => {
					// S_c 结束 → 进入 S_s
					CurrentEpoch::<T>::mutate(|e| e.phase = EpochPhase::Syncing);
					Self::deposit_event(Event::SyncingSlotStarted {
						epoch: epoch.epoch_id,
						block: now_u32,
					});
					T::WeightInfo::sync_task()
				},
				EpochPhase::Syncing if elapsed >= sc + ss => {
					// Epoch 结束 → 自动 finalize
					Self::do_finalize_epoch();
					T::WeightInfo::finalize_epoch()
				},
				_ => Weight::zero(),
			}
		}
	}

	// ── Dispatchable ─────────────────────────────────────────────────────────

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		/// 矿工代表将主链激活任务同步到子链（S_c 或 S_s 均可调用）
		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::sync_task())]
		pub fn sync_task(
			origin: OriginFor<T>,
			task_id: TaskId,
			requester: T::AccountId,
			budget_per_epoch: BalanceOf<T>,
			description: BoundedVec<u8, ConstU32<256>>,
		) -> DispatchResult {
			ensure_signed(origin)?;
			ActiveTasks::<T>::insert(
				task_id,
				TaskDetail { requester, budget_per_epoch, description, status: TaskStatus::Active },
			);
			Self::deposit_event(Event::TaskSynced { task_id });
			Ok(())
		}

		/// 工作者提交众包数据（仅 S_c 时隙有效）
		#[pallet::call_index(1)]
		#[pallet::weight(T::WeightInfo::submit_data())]
		pub fn submit_data(
			origin: OriginFor<T>,
			task_id: TaskId,
			data: BoundedVec<u8, ConstU32<1024>>,
			reward: BalanceOf<T>,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Self::accept_submission(&who, task_id, data, reward)?;

			if T::FullSubmissionEvents::get() {
				Self::deposit_event(Event::DataSubmitted { task_id, worker: who, reward });
			} else {
				Self::deposit_event(Event::DataSubmittedCompact { task_id, reward });
			}
			Ok(())
		}

		/// 工作者批量提交众包数据。TPS 图按业务提交数计数，而不是 raw extrinsic 数。
		#[pallet::call_index(3)]
		#[pallet::weight(T::WeightInfo::submit_data_batch(data.len() as u32))]
		pub fn submit_data_batch(
			origin: OriginFor<T>,
			task_id: TaskId,
			data: BoundedVec<BoundedVec<u8, ConstU32<1024>>, T::MaxBatchSize>,
			reward_each: BalanceOf<T>,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			ensure!(!data.is_empty(), Error::<T>::EmptyBatch);
			ensure!(data.len() <= T::MaxBatchSize::get() as usize, Error::<T>::BatchTooLarge);

			for item in data.iter() {
				Self::accept_submission(&who, task_id, item.clone(), reward_each)?;
			}

			Self::deposit_event(Event::DataBatchSubmitted {
				task_id,
				worker: who,
				accepted: data.len() as u32,
				reward_each,
			});
			Ok(())
		}

		/// 矿工代表手动触发 Epoch 结算（S_s 时隙内可提前调用）
		#[pallet::call_index(2)]
		#[pallet::weight(T::WeightInfo::finalize_epoch())]
		pub fn finalize_epoch(origin: OriginFor<T>) -> DispatchResult {
			ensure_signed(origin)?;
			ensure!(
				CurrentEpoch::<T>::get().phase == EpochPhase::Syncing,
				Error::<T>::NotInSyncingSlot
			);
			Self::do_finalize_epoch();
			Ok(())
		}
	}

	// ── 内部方法 ─────────────────────────────────────────────────────────────

	impl<T: Config> Pallet<T> {
		fn accept_submission(
			who: &T::AccountId,
			task_id: TaskId,
			data: BoundedVec<u8, ConstU32<1024>>,
			reward: BalanceOf<T>,
		) -> DispatchResult {
			ensure!(
				CurrentEpoch::<T>::get().phase == EpochPhase::Collecting,
				Error::<T>::NotInCollectingSlot
			);

			let task = ActiveTasks::<T>::get(task_id).ok_or(Error::<T>::TaskNotFound)?;
			ensure!(task.status == TaskStatus::Active, Error::<T>::BudgetExhausted);
			ensure!(T::DataValidator::validate(task_id, who, &data), Error::<T>::InvalidData);

			let spent = SpentBudget::<T>::get(task_id);
			let new_spent = spent.checked_add(&reward).ok_or(Error::<T>::Overflow)?;
			ensure!(new_spent <= task.budget_per_epoch, Error::<T>::ExceedsBudget);

			let epoch_id = CurrentEpoch::<T>::get().epoch_id;
			let submission = Submission { task_id, worker: who.clone(), reward, data };

			if T::IndexedSubmissionStorage::get() {
				let index = SubmissionCountByTaskEpoch::<T>::get(task_id, epoch_id);
				let next_index = index.checked_add(1).ok_or(Error::<T>::Overflow)?;
				SubmissionDigestByTaskEpochIndex::<T>::insert(
					(task_id, epoch_id),
					index,
					<T::Hashing as HashT>::hash(&submission.encode()),
				);
				SubmissionCountByTaskEpoch::<T>::insert(task_id, epoch_id, next_index);
			} else {
				EpochSubmissions::<T>::try_mutate(|subs| -> DispatchResult {
					subs.try_push(submission).map_err(|_| Error::<T>::SubmissionLimitReached)?;
					Ok(())
				})?;
			}

			AcceptedSubmissionCount::<T>::try_mutate(|count| -> DispatchResult {
				*count = count.checked_add(1).ok_or(Error::<T>::Overflow)?;
				Ok(())
			})?;
			EpochBills::<T>::mutate(who, |amount| *amount = amount.saturating_add(reward));
			SpentBudget::<T>::insert(task_id, new_spent);

			if new_spent >= task.budget_per_epoch {
				ActiveTasks::<T>::mutate(task_id, |t| {
					if let Some(t) = t {
						t.status = TaskStatus::Exhausted;
					}
				});
			}

			Ok(())
		}

		/// 执行 Epoch 结算：计算 Merkle Root、聚合账单、重置状态、发出事件
		fn do_finalize_epoch() {
			let epoch_info = CurrentEpoch::<T>::get();
			let epoch_id = epoch_info.epoch_id;
			let submissions = EpochSubmissions::<T>::get();

			// 1. Merkle Root：以每条提交记录的 SCALE 哈希作为叶节点
			let root: T::Hash = if T::IndexedSubmissionStorage::get()
				&& AcceptedSubmissionCount::<T>::get() > 0
			{
				let leaves: Vec<T::Hash> = SubmissionDigestByTaskEpochIndex::<T>::iter()
					.filter_map(|((_, stored_epoch), _, digest)| {
						(stored_epoch == epoch_id).then_some(digest)
					})
					.collect();
				let root_bytes = merkle_root::<T::Hashing, _>(leaves.iter().map(|h| h.as_ref()));
				T::Hash::decode(&mut root_bytes.as_ref()).unwrap_or_default()
			} else if submissions.is_empty() {
				T::Hash::default()
			} else {
				let leaves: Vec<T::Hash> =
					submissions.iter().map(|s| <T::Hashing as HashT>::hash(&s.encode())).collect();
				// binary_merkle_tree::merkle_root 返回 [u8; 32]，需转为 T::Hash
				let root_bytes = merkle_root::<T::Hashing, _>(leaves.iter().map(|h| h.as_ref()));
				T::Hash::decode(&mut root_bytes.as_ref()).unwrap_or_default()
			};

			// 2. 账单聚合：同一 worker 的多次提交奖励合并
			let reward_map: BTreeMap<T::AccountId, BalanceOf<T>> =
				if T::IndexedSubmissionStorage::get() {
					EpochBills::<T>::iter().collect()
				} else {
					let mut rewards = BTreeMap::new();
					for s in submissions.iter() {
						let entry =
							rewards.entry(s.worker.clone()).or_insert(BalanceOf::<T>::default());
						*entry = entry.saturating_add(s.reward);
					}
					rewards
				};
			let bill_vec: Vec<(T::AccountId, BalanceOf<T>)> = reward_map.into_iter().collect();
			let bill_amounts: BoundedVec<_, T::MaxSubmissionsPerEpoch> =
				bill_vec.try_into().unwrap_or_default();

			// 3. 存储 Merkle Root
			EpochRoots::<T>::insert(epoch_id, root);

			// 4. 发出事件（链下桥接脚本监听此事件）
			Self::deposit_event(Event::EpochFinalized {
				chain_id: T::ChainProfile::chain_id(),
				epoch: epoch_id,
				merkle_root: root,
				bill_amounts,
			});

			// 5. 清理本 epoch 数据
			EpochSubmissions::<T>::kill();
			AcceptedSubmissionCount::<T>::kill();
			let _ = EpochBills::<T>::clear(1000, None);
			// 清理各任务的 SpentBudget 并重置 Exhausted 任务
			ActiveTasks::<T>::translate(|_id, mut task: TaskDetail<T::AccountId, BalanceOf<T>>| {
				task.status = TaskStatus::Active;
				Some(task)
			});
			let _ = SpentBudget::<T>::clear(1000, None);

			// 6. 开启新 Epoch
			let now = frame_system::Pallet::<T>::block_number().saturated_into::<u32>();
			CurrentEpoch::<T>::put(EpochInfo {
				epoch_id: epoch_id.saturating_add(1),
				phase: EpochPhase::Collecting,
				start_block: now,
			});
		}

		/// 供测试使用：直接查询当前 Epoch 信息
		pub fn current_epoch() -> EpochInfo {
			CurrentEpoch::<T>::get()
		}
	}
}
