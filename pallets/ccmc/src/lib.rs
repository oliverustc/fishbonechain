//! pallet-ccmc: Child Chain Management Contract
//! 管理子链的注册、矿工、Epoch 摘要提交与验证。
//! Phase 1 实现骨架，业务逻辑在 Step 2 填充。

#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;

#[cfg(test)]
mod mock;
#[cfg(test)]
mod tests;

pub mod types;

#[frame_support::pallet]
pub mod pallet {
    use frame_support::{
        pallet_prelude::*,
        traits::{Currency, ReservableCurrency},
    };
    use frame_system::pallet_prelude::*;

    use crate::types::*;

    pub type BalanceOf<T> =
        <<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

    #[pallet::config]
    pub trait Config: frame_system::Config {
        type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

        /// 用于押金管理的货币
        type Currency: ReservableCurrency<Self::AccountId>;

        type WeightInfo: WeightInfo;
    }

    pub trait WeightInfo {
        fn register_child_chain() -> Weight;
        fn join_child_chain() -> Weight;
        fn leave_child_chain() -> Weight;
        fn submit_epoch_digest() -> Weight;
        fn vote_slash_miner() -> Weight;
        fn terminate_child_chain() -> Weight;
    }

    impl WeightInfo for () {
        fn register_child_chain() -> Weight { Weight::from_parts(10_000, 0) }
        fn join_child_chain()     -> Weight { Weight::from_parts(10_000, 0) }
        fn leave_child_chain()    -> Weight { Weight::from_parts(10_000, 0) }
        fn submit_epoch_digest()  -> Weight { Weight::from_parts(10_000, 0) }
        fn vote_slash_miner()     -> Weight { Weight::from_parts(10_000, 0) }
        fn terminate_child_chain()-> Weight { Weight::from_parts(10_000, 0) }
    }

    // ── 存储 ─────────────────────────────────────────────────────────────────

    /// 子链信息
    #[pallet::storage]
    pub type ChildChains<T: Config> =
        StorageMap<_, Blake2_128Concat, ChainId, ChainInfo<T::AccountId, BalanceOf<T>>>;

    /// 子链矿工信息（含押金）
    #[pallet::storage]
    pub type Miners<T: Config> = StorageDoubleMap<
        _,
        Blake2_128Concat, ChainId,
        Blake2_128Concat, T::AccountId,
        MinerInfo<BalanceOf<T>>,
    >;

    /// Epoch 确认摘要（达到阈值后写入）
    #[pallet::storage]
    pub type EpochDigests<T: Config> = StorageDoubleMap<
        _,
        Blake2_128Concat, ChainId,
        Blake2_128Concat, EpochId,
        T::Hash,
    >;

    /// Epoch 摘要投票（矿工 → 候选 Root → 投票集合）
    #[pallet::storage]
    pub type DigestVotes<T: Config> = StorageNMap<
        _,
        (
            NMapKey<Blake2_128Concat, ChainId>,
            NMapKey<Blake2_128Concat, EpochId>,
            NMapKey<Blake2_128Concat, T::Hash>,
        ),
        BoundedBTreeSet<T::AccountId, ConstU32<100>>,
    >;

    /// 恶意矿工驱逐投票
    #[pallet::storage]
    pub type SlashVotes<T: Config> = StorageDoubleMap<
        _,
        Blake2_128Concat, ChainId,
        Blake2_128Concat, T::AccountId, // 被投矿工
        BoundedBTreeSet<T::AccountId, ConstU32<100>>,
    >;

    /// 子链 ID 计数器
    #[pallet::storage]
    #[pallet::getter(fn next_chain_id)]
    pub type NextChainId<T: Config> = StorageValue<_, ChainId, ValueQuery>;

    // ── 事件 ─────────────────────────────────────────────────────────────────

    #[pallet::event]
    #[pallet::generate_deposit(pub(super) fn deposit_event)]
    pub enum Event<T: Config> {
        /// 新子链注册
        ChainRegistered { chain_id: ChainId, creator: T::AccountId },
        /// 矿工加入子链
        MinerJoined { chain_id: ChainId, miner: T::AccountId },
        /// 矿工退出子链
        MinerLeft { chain_id: ChainId, miner: T::AccountId },
        /// Epoch 摘要确认（达到阈值）
        EpochDigestConfirmed { chain_id: ChainId, epoch: EpochId, root: T::Hash },
        /// 矿工被驱逐
        MinerSlashed { chain_id: ChainId, miner: T::AccountId },
        /// 子链终止
        ChainTerminated { chain_id: ChainId },
    }

    // ── 错误 ─────────────────────────────────────────────────────────────────

    #[pallet::error]
    pub enum Error<T> {
        ChainNotFound,
        ChainTerminated,
        NotAMiner,
        AlreadyAMiner,
        AlreadyVoted,
        InsufficientDeposit,
        /// 子链已存在（内部错误）
        ChainIdOverflow,
    }

    // ── Pallet ───────────────────────────────────────────────────────────────

    #[pallet::pallet]
    pub struct Pallet<T>(_);

    // ── Dispatchable ─────────────────────────────────────────────────────────

    #[pallet::call]
    impl<T: Config> Pallet<T> {
        /// 注册新子链（调用者成为 creator，并自动成为第一个矿工）
        #[pallet::call_index(0)]
        #[pallet::weight(T::WeightInfo::register_child_chain())]
        pub fn register_child_chain(
            origin: OriginFor<T>,
            name: BoundedVec<u8, ConstU32<64>>,
            min_miners: u32,
            deposit_required: BalanceOf<T>,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            let chain_id = Self::next_chain_id();
            let next = chain_id.checked_add(1).ok_or(Error::<T>::ChainIdOverflow)?;

            // 锁定创建者押金
            T::Currency::reserve(&who, deposit_required)?;

            let info = ChainInfo {
                creator: who.clone(),
                name,
                miner_count: 1,
                min_miners: min_miners.max(1),
                deposit_required,
                status: ChainStatus::Active,
            };
            ChildChains::<T>::insert(chain_id, info);

            let miner_info = MinerInfo {
                deposit: deposit_required,
                status: MinerStatus::Active,
            };
            Miners::<T>::insert(chain_id, &who, miner_info);
            NextChainId::<T>::put(next);

            Self::deposit_event(Event::ChainRegistered { chain_id, creator: who.clone() });
            Self::deposit_event(Event::MinerJoined { chain_id, miner: who });
            Ok(())
        }

        /// 矿工加入已有子链（缴纳押金）
        #[pallet::call_index(1)]
        #[pallet::weight(T::WeightInfo::join_child_chain())]
        pub fn join_child_chain(
            origin: OriginFor<T>,
            chain_id: ChainId,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            let mut chain = ChildChains::<T>::get(chain_id).ok_or(Error::<T>::ChainNotFound)?;
            ensure!(chain.status == ChainStatus::Active, Error::<T>::ChainTerminated);
            ensure!(!Miners::<T>::contains_key(chain_id, &who), Error::<T>::AlreadyAMiner);

            T::Currency::reserve(&who, chain.deposit_required)?;

            let miner_info = MinerInfo {
                deposit: chain.deposit_required,
                status: MinerStatus::Active,
            };
            Miners::<T>::insert(chain_id, &who, miner_info);
            chain.miner_count = chain.miner_count.saturating_add(1);
            ChildChains::<T>::insert(chain_id, chain);

            Self::deposit_event(Event::MinerJoined { chain_id, miner: who });
            Ok(())
        }

        /// 矿工退出子链（归还押金）
        #[pallet::call_index(2)]
        #[pallet::weight(T::WeightInfo::leave_child_chain())]
        pub fn leave_child_chain(
            origin: OriginFor<T>,
            chain_id: ChainId,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            let miner = Miners::<T>::get(chain_id, &who).ok_or(Error::<T>::NotAMiner)?;
            T::Currency::unreserve(&who, miner.deposit);

            Miners::<T>::remove(chain_id, &who);
            ChildChains::<T>::mutate(chain_id, |c| {
                if let Some(chain) = c {
                    chain.miner_count = chain.miner_count.saturating_sub(1);
                }
            });

            Self::deposit_event(Event::MinerLeft { chain_id, miner: who });
            Ok(())
        }

        /// 提交 Epoch 摘要投票（达到 ≥2/3 阈值后自动确认）
        #[pallet::call_index(3)]
        #[pallet::weight(T::WeightInfo::submit_epoch_digest())]
        pub fn submit_epoch_digest(
            origin: OriginFor<T>,
            chain_id: ChainId,
            epoch: EpochId,
            root: T::Hash,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            let chain = ChildChains::<T>::get(chain_id).ok_or(Error::<T>::ChainNotFound)?;
            ensure!(chain.status == ChainStatus::Active, Error::<T>::ChainTerminated);
            ensure!(Miners::<T>::contains_key(chain_id, &who), Error::<T>::NotAMiner);
            // 已确认的 epoch 不再接受投票
            ensure!(!EpochDigests::<T>::contains_key(chain_id, epoch), Error::<T>::AlreadyVoted);

            DigestVotes::<T>::try_mutate((chain_id, epoch, root), |votes| -> DispatchResult {
                let votes = votes.get_or_insert_with(BoundedBTreeSet::new);
                ensure!(!votes.contains(&who), Error::<T>::AlreadyVoted);
                votes.try_insert(who).map_err(|_| Error::<T>::ChainIdOverflow)?;

                // 检查是否达到 2/3 阈值
                let threshold = (chain.miner_count * 2).div_ceil(3);
                if votes.len() as u32 >= threshold {
                    EpochDigests::<T>::insert(chain_id, epoch, root);
                    Self::deposit_event(Event::EpochDigestConfirmed { chain_id, epoch, root });
                }
                Ok(())
            })
        }

        /// 投票驱逐恶意矿工（超过 2/3 矿工投票后执行 slash）
        #[pallet::call_index(4)]
        #[pallet::weight(T::WeightInfo::vote_slash_miner())]
        pub fn vote_slash_miner(
            origin: OriginFor<T>,
            chain_id: ChainId,
            target: T::AccountId,
        ) -> DispatchResult {
            let who = ensure_signed(origin)?;

            let chain = ChildChains::<T>::get(chain_id).ok_or(Error::<T>::ChainNotFound)?;
            ensure!(Miners::<T>::contains_key(chain_id, &who), Error::<T>::NotAMiner);
            ensure!(Miners::<T>::contains_key(chain_id, &target), Error::<T>::NotAMiner);

            let target_key = target.clone();
            SlashVotes::<T>::try_mutate(chain_id, target_key, |votes| -> DispatchResult {
                let votes = votes.get_or_insert_with(BoundedBTreeSet::new);
                ensure!(!votes.contains(&who), Error::<T>::AlreadyVoted);
                votes.try_insert(who).map_err(|_| Error::<T>::ChainIdOverflow)?;

                let threshold = (chain.miner_count * 2).div_ceil(3);
                if votes.len() as u32 >= threshold {
                    if let Some(miner) = Miners::<T>::get(chain_id, &target) {
                        T::Currency::unreserve(&target, miner.deposit);
                        let _ = T::Currency::slash_reserved(&target, miner.deposit);
                    }
                    Miners::<T>::remove(chain_id, &target);
                    ChildChains::<T>::mutate(chain_id, |c| {
                        if let Some(chain) = c {
                            chain.miner_count = chain.miner_count.saturating_sub(1);
                        }
                    });
                    SlashVotes::<T>::remove(chain_id, &target);
                    Self::deposit_event(Event::MinerSlashed { chain_id, miner: target });
                }
                Ok(())
            })
        }

        /// 终止子链（creator 或 sudo 权限）
        #[pallet::call_index(5)]
        #[pallet::weight(T::WeightInfo::terminate_child_chain())]
        pub fn terminate_child_chain(
            origin: OriginFor<T>,
            chain_id: ChainId,
        ) -> DispatchResult {
            let mut chain = ChildChains::<T>::get(chain_id).ok_or(Error::<T>::ChainNotFound)?;

            // Root 可直接终止；普通账户只有 creator 可以终止
            match ensure_signed(origin.clone()) {
                Ok(caller) => ensure!(caller == chain.creator, Error::<T>::NotAMiner),
                Err(_) => ensure_root(origin)?,
            }

            chain.status = ChainStatus::Terminated;
            ChildChains::<T>::insert(chain_id, chain);

            Self::deposit_event(Event::ChainTerminated { chain_id });
            Ok(())
        }
    }

    // ── 公开查询接口 ─────────────────────────────────────────────────────────

    impl<T: Config> Pallet<T> {
        /// 查询某矿工是否在指定子链中（供 pallet-fmc 调用）
        pub fn is_miner(chain_id: ChainId, who: &T::AccountId) -> bool {
            Miners::<T>::contains_key(chain_id, who)
        }

        /// 获取子链 miner_count（供阈值计算）
        pub fn miner_count(chain_id: ChainId) -> u32 {
            ChildChains::<T>::get(chain_id)
                .map(|c| c.miner_count)
                .unwrap_or(0)
        }

        /// 验证 block_hash 是否在指定 epoch 的 Merkle Root 下
        /// 简化版：只检查 root 是否已确认，完整 Merkle 验证在 BPiano 阶段引入
        pub fn epoch_root_confirmed(chain_id: ChainId, epoch: EpochId) -> Option<T::Hash> {
            EpochDigests::<T>::get(chain_id, epoch)
        }
    }
}
