//! pallet-fmc: Fund Management Contract
//! 管理请求者资金池（FB/LB 双余额）和众包任务生命周期。

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[cfg(test)]
mod mock;
#[cfg(test)]
mod tests;

pub mod types;

/// Blanket impl: any `pallet_ccmc::Pallet<T>` automatically satisfies `CcmcInterface`.
/// This lets the runtime set `type CcmcPallet = Ccmc;` without extra boilerplate.
impl<T> pallet::CcmcInterface<T::AccountId> for pallet_ccmc::Pallet<T>
where
    T: frame_system::Config + pallet_ccmc::Config,
{
    fn is_miner(chain_id: pallet_ccmc::types::ChainId, who: &T::AccountId) -> bool {
        pallet_ccmc::Pallet::<T>::is_miner(chain_id, who)
    }
    fn miner_count(chain_id: pallet_ccmc::types::ChainId) -> u32 {
        pallet_ccmc::Pallet::<T>::miner_count(chain_id)
    }
}

#[frame_support::pallet]
pub mod pallet {
    use codec::Encode;
    use frame_support::{
        pallet_prelude::*,
        traits::{Currency, ExistenceRequirement, ReservableCurrency},
        PalletId,
    };
    use frame_system::pallet_prelude::*;
    use sp_runtime::traits::{AccountIdConversion, Hash, Saturating};

    use crate::types::*;

    pub type BalanceOf<T> =
        <<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

    // ── Config ───────────────────────────────────────────────────────────────

    /// FMC 对 CCMC 的查询接口（解耦，避免类型歧义）
    pub trait CcmcInterface<AccountId> {
        fn is_miner(chain_id: pallet_ccmc::types::ChainId, who: &AccountId) -> bool;
        fn miner_count(chain_id: pallet_ccmc::types::ChainId) -> u32;
    }

    #[pallet::config]
    pub trait Config: frame_system::Config {
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

        /// 用于资金托管的货币（不继承 CCMC 的 Currency，独立声明）
        type Currency: ReservableCurrency<Self::AccountId>;

        /// CCMC 查询接口（由外部实现，避免直接依赖 pallet_ccmc::Config）
        type CcmcPallet: CcmcInterface<Self::AccountId>;

        type WeightInfo: WeightInfo;
    }

    pub trait WeightInfo {
        fn deposit() -> Weight;
        fn withdraw() -> Weight;
        fn create_task() -> Weight;
        fn activate_task() -> Weight;
        fn terminate_task() -> Weight;
        fn submit_bill() -> Weight;
    }

    impl WeightInfo for () {
        fn deposit()        -> Weight { Weight::from_parts(10_000, 0) }
        fn withdraw()       -> Weight { Weight::from_parts(10_000, 0) }
        fn create_task()    -> Weight { Weight::from_parts(10_000, 0) }
        fn activate_task()  -> Weight { Weight::from_parts(10_000, 0) }
        fn terminate_task() -> Weight { Weight::from_parts(10_000, 0) }
        fn submit_bill()    -> Weight { Weight::from_parts(50_000, 0) }
    }

    // ── 存储 ─────────────────────────────────────────────────────────────────

    #[pallet::storage]
    pub type FundPools<T: Config> =
        StorageMap<_, Blake2_128Concat, T::AccountId, FundPool<BalanceOf<T>>>;

    #[pallet::storage]
    pub type Tasks<T: Config> = StorageDoubleMap<
        _,
        Blake2_128Concat, T::AccountId,
        Blake2_128Concat, TaskId,
        TaskInfo<BalanceOf<T>>,
    >;

    #[pallet::storage]
    pub type NextTaskId<T: Config> =
        StorageMap<_, Blake2_128Concat, T::AccountId, TaskId, ValueQuery>;

    /// Epoch 账单投票
    #[pallet::storage]
    pub type BillVotes<T: Config> = StorageNMap<
        _,
        (
            NMapKey<Blake2_128Concat, T::AccountId>,
            NMapKey<Blake2_128Concat, TaskId>,
            NMapKey<Blake2_128Concat, pallet_ccmc::types::EpochId>,
            NMapKey<Blake2_128Concat, T::Hash>,
        ),
        BoundedBTreeSet<T::AccountId, ConstU32<100>>,
    >;

    // ── 事件 ─────────────────────────────────────────────────────────────────

    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        Deposited   { who: T::AccountId, amount: BalanceOf<T> },
        Withdrawn   { who: T::AccountId, amount: BalanceOf<T> },
        TaskCreated { requester: T::AccountId, task_id: TaskId },
        TaskActivated  { requester: T::AccountId, task_id: TaskId },
        TaskTerminated { requester: T::AccountId, task_id: TaskId },
        BillSettled {
            requester: T::AccountId,
            task_id: TaskId,
            epoch: pallet_ccmc::types::EpochId,
            total_paid: BalanceOf<T>,
        },
    }

    // ── 错误 ─────────────────────────────────────────────────────────────────

    #[pallet::error]
    pub enum Error<T> {
        NoFundPool,
        InsufficientFreeBalance,
        TaskNotFound,
        TaskNotActive,
        ExceedsBudget,
        NotAMiner,
        AlreadyVoted,
        Overflow,
    }

    // ── Pallet ───────────────────────────────────────────────────────────────

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    // ── Dispatchable ─────────────────────────────────────────────────────────

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        /// 向 FMC 充值（资金进入 FB）
        #[pallet::call_index(0)]
        #[pallet::weight(<T as Config>::WeightInfo::deposit())]
        pub fn deposit(origin: OriginFor<T>, amount: BalanceOf<T>) -> DispatchResult {
            let who = ensure_signed(origin)?;
            T::Currency::transfer(
                &who,
                &Self::account_id(),
                amount,
                ExistenceRequirement::KeepAlive,
            )?;
            FundPools::<T>::mutate(&who, |pool| {
                let p = pool.get_or_insert(FundPool {
                    free:   BalanceOf::<T>::default(),
                    locked: BalanceOf::<T>::default(),
                });
                p.free = p.free.saturating_add(amount);
            });
            Self::deposit_event(Event::Deposited { who, amount });
            Ok(())
        }

        /// 从 FMC 提款（只能提取 FB）
        #[pallet::call_index(1)]
        #[pallet::weight(<T as Config>::WeightInfo::withdraw())]
        pub fn withdraw(origin: OriginFor<T>, amount: BalanceOf<T>) -> DispatchResult {
            let who = ensure_signed(origin)?;
            FundPools::<T>::try_mutate(&who, |pool| -> DispatchResult {
                let p = pool.as_mut().ok_or(Error::<T>::NoFundPool)?;
                ensure!(p.free >= amount, Error::<T>::InsufficientFreeBalance);
                p.free = p.free.saturating_sub(amount);
                Ok(())
            })?;
            T::Currency::transfer(
                &Self::account_id(),
                &who,
                amount,
                ExistenceRequirement::AllowDeath,
            )?;
            Self::deposit_event(Event::Withdrawn { who, amount });
            Ok(())
        }

        /// 创建任务（初始 Terminated）
        #[pallet::call_index(2)]
        #[pallet::weight(<T as Config>::WeightInfo::create_task())]
        pub fn create_task(
            origin: OriginFor<T>,
            target_chain: pallet_ccmc::types::ChainId,
            budget_per_epoch: BalanceOf<T>,
            description: BoundedVec<u8, ConstU32<256>>,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;
            let task_id = NextTaskId::<T>::get(&who);
            let next = task_id.checked_add(1).ok_or(Error::<T>::Overflow)?;
            Tasks::<T>::insert(&who, task_id, TaskInfo {
                target_chain,
                budget_per_epoch,
                status: TaskStatus::Terminated,
                current_epoch: 0,
                description,
            });
            NextTaskId::<T>::insert(&who, next);
            Self::deposit_event(Event::TaskCreated { requester: who, task_id });
            Ok(())
        }

        /// 激活任务（FB ≥ B → B 从 FB 转入 LB）
        #[pallet::call_index(3)]
        #[pallet::weight(<T as Config>::WeightInfo::activate_task())]
        pub fn activate_task(origin: OriginFor<T>, task_id: TaskId) -> DispatchResult {
            let who = ensure_signed(origin)?;
            let task = Tasks::<T>::get(&who, task_id).ok_or(Error::<T>::TaskNotFound)?;
            let budget = task.budget_per_epoch;
            FundPools::<T>::try_mutate(&who, |pool| -> DispatchResult {
                let p = pool.as_mut().ok_or(Error::<T>::NoFundPool)?;
                ensure!(p.free >= budget, Error::<T>::InsufficientFreeBalance);
                p.free   = p.free.saturating_sub(budget);
                p.locked = p.locked.saturating_add(budget);
                Ok(())
            })?;
            Tasks::<T>::mutate(&who, task_id, |t| {
                if let Some(t) = t { t.status = TaskStatus::Activated; }
            });
            Self::deposit_event(Event::TaskActivated { requester: who, task_id });
            Ok(())
        }

        /// 终止任务（LB 预算归还 FB）
        #[pallet::call_index(4)]
        #[pallet::weight(<T as Config>::WeightInfo::terminate_task())]
        pub fn terminate_task(origin: OriginFor<T>, task_id: TaskId) -> DispatchResult {
            let who = ensure_signed(origin)?;
            let task = Tasks::<T>::get(&who, task_id).ok_or(Error::<T>::TaskNotFound)?;
            if task.status == TaskStatus::Activated {
                let budget = task.budget_per_epoch;
                FundPools::<T>::mutate(&who, |pool| {
                    if let Some(p) = pool {
                        p.locked = p.locked.saturating_sub(budget);
                        p.free   = p.free.saturating_add(budget);
                    }
                });
            }
            Tasks::<T>::mutate(&who, task_id, |t| {
                if let Some(t) = t { t.status = TaskStatus::Terminated; }
            });
            Self::deposit_event(Event::TaskTerminated { requester: who, task_id });
            Ok(())
        }

        /// 提交 Epoch 账单（矿工投票，达到 2/3 阈值后自动结算）
        #[pallet::call_index(5)]
        #[pallet::weight(<T as Config>::WeightInfo::submit_bill())]
        pub fn submit_bill(
            origin: OriginFor<T>,
            requester: T::AccountId,
            task_id: TaskId,
            epoch: pallet_ccmc::types::EpochId,
            bill_amounts: BoundedVec<(T::AccountId, BalanceOf<T>), ConstU32<1024>>,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            let task = Tasks::<T>::get(&requester, task_id).ok_or(Error::<T>::TaskNotFound)?;
            ensure!(task.status == TaskStatus::Activated, Error::<T>::TaskNotActive);
            ensure!(
                T::CcmcPallet::is_miner(task.target_chain, &who),
                Error::<T>::NotAMiner
            );

            // 验证账单总额不超预算
            let total: BalanceOf<T> = bill_amounts
                .iter()
                .try_fold(BalanceOf::<T>::default(), |acc, (_, amount)| {
                    acc.checked_add(amount).ok_or(Error::<T>::Overflow)
                })?;
            ensure!(total <= task.budget_per_epoch, Error::<T>::ExceedsBudget);

            // 账单哈希：对账单内容编码后哈希
            let bill_hash = T::Hashing::hash(&bill_amounts.encode());

            let miner_count = T::CcmcPallet::miner_count(task.target_chain);
            let threshold   = (miner_count * 2).div_ceil(3).max(1);

            let settled = BillVotes::<T>::try_mutate(
                (&requester, task_id, epoch, bill_hash),
                |votes| -> Result<bool, DispatchError> {
                    let votes = votes.get_or_insert_with(BoundedBTreeSet::new);
                    ensure!(!votes.contains(&who), Error::<T>::AlreadyVoted);
                    votes.try_insert(who).map_err(|_| Error::<T>::Overflow)?;
                    Ok(votes.len() as u32 >= threshold)
                },
            )?;

            if settled {
                Self::settle_bill(&requester, task_id, epoch, &bill_amounts, total)?;
            }
            Ok(())
        }
    }

    // ── 内部方法 ─────────────────────────────────────────────────────────────

    impl<T: Config> Pallet<T> {
        pub fn account_id() -> T::AccountId {
            const PALLET_ID: PalletId = PalletId(*b"fishb/fm");
            PALLET_ID.into_account_truncating()
        }

        fn settle_bill(
            requester: &T::AccountId,
            task_id: TaskId,
            epoch: pallet_ccmc::types::EpochId,
            bill_amounts: &[(T::AccountId, BalanceOf<T>)],
            total: BalanceOf<T>,
        ) -> DispatchResult {
            let task = Tasks::<T>::get(requester, task_id).ok_or(Error::<T>::TaskNotFound)?;
            let budget = task.budget_per_epoch;

            for (recipient, amount) in bill_amounts {
                T::Currency::transfer(
                    &Self::account_id(),
                    recipient,
                    *amount,
                    ExistenceRequirement::KeepAlive,
                )?;
            }

            let remainder = budget.saturating_sub(total);

            // 判断是否可以自动续期
            let can_renew = FundPools::<T>::get(requester)
                .map(|p| p.free.saturating_add(remainder) >= budget)
                .unwrap_or(false);

            FundPools::<T>::mutate(requester, |pool| {
                if let Some(p) = pool {
                    p.locked = p.locked.saturating_sub(budget);
                    p.free   = p.free.saturating_add(remainder);
                    if can_renew {
                        p.free   = p.free.saturating_sub(budget);
                        p.locked = p.locked.saturating_add(budget);
                    }
                }
            });

            let new_status = if can_renew { TaskStatus::Activated } else { TaskStatus::Terminated };
            Tasks::<T>::mutate(requester, task_id, |t| {
                if let Some(t) = t {
                    t.status = new_status;
                    t.current_epoch = t.current_epoch.saturating_add(1);
                }
            });

            Self::deposit_event(Event::BillSettled {
                requester: requester.clone(),
                task_id,
                epoch,
                total_paid: total,
            });
            Ok(())
        }
    }
}
