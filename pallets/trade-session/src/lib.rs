#![cfg_attr(not(feature = "std"), no_std)]

extern crate alloc;

pub use pallet::*;
pub use types::*;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

pub mod types;

pub trait WeightInfo {
	fn create_session() -> frame_support::weights::Weight;
	fn lock_funds() -> frame_support::weights::Weight;
	fn lock_deposit() -> frame_support::weights::Weight;
	fn claim_funds() -> frame_support::weights::Weight;
	fn punish() -> frame_support::weights::Weight;
}

impl WeightInfo for () {
	fn create_session() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(20_000, 0)
	}

	fn lock_funds() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(20_000, 0)
	}

	fn lock_deposit() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(20_000, 0)
	}

	fn claim_funds() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(50_000, 0)
	}

	fn punish() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(20_000, 0)
	}
}

#[frame_support::pallet]
pub mod pallet {
	use super::*;
	use alloc::vec::Vec;
	use codec::Encode;
	use frame_support::{
		pallet_prelude::*,
		traits::{BalanceStatus, Currency, ReservableCurrency},
	};
	use frame_system::pallet_prelude::*;
	use sp_runtime::traits::Hash;

	pub type BalanceOf<T> =
		<<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
		type Currency: ReservableCurrency<Self::AccountId>;
		type WeightInfo: WeightInfo;
	}

	#[pallet::storage]
	pub type Sessions<T: Config> = StorageMap<
		_,
		Blake2_128Concat,
		SessionId,
		TradingSession<T::AccountId, BalanceOf<T>, T::Hash>,
	>;

	#[pallet::storage]
	pub type NextSessionId<T: Config> = StorageValue<_, SessionId, ValueQuery>;

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		SessionCreated { session_id: SessionId, requester: T::AccountId, data_owner: T::AccountId },
		FundsLocked { session_id: SessionId, requester: T::AccountId, amount: BalanceOf<T> },
		DepositLocked { session_id: SessionId, data_owner: T::AccountId, amount: BalanceOf<T> },
		FundsClaimed { session_id: SessionId, data_owner: T::AccountId, rounds: u32 },
		SessionPunished { session_id: SessionId },
	}

	#[pallet::error]
	pub enum Error<T> {
		SessionIdOverflow,
		SessionNotFound,
		NotRequester,
		NotDataOwner,
		InvalidSessionStatus,
		InvalidHashPreimage,
		UnsupportedSettlementMode,
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::create_session())]
		pub fn create_session(
			origin: OriginFor<T>,
			data_owner: T::AccountId,
			hash_chain_end: T::Hash,
			max_rounds: u32,
			settlement_mode: TradeSettlementMode,
		) -> DispatchResult {
			let requester = ensure_signed(origin)?;
			let session_id = NextSessionId::<T>::get();
			let next_id = session_id.checked_add(1).ok_or(Error::<T>::SessionIdOverflow)?;

			Sessions::<T>::insert(
				session_id,
				TradingSession {
					requester: requester.clone(),
					data_owner: data_owner.clone(),
					hash_chain_end,
					max_rounds,
					locked_funds: BalanceOf::<T>::default(),
					deposit: BalanceOf::<T>::default(),
					status: SessionStatus::Created,
					settlement_mode,
				},
			);
			NextSessionId::<T>::put(next_id);
			Self::deposit_event(Event::SessionCreated { session_id, requester, data_owner });
			Ok(())
		}

		#[pallet::call_index(1)]
		#[pallet::weight(T::WeightInfo::lock_funds())]
		pub fn lock_funds(
			origin: OriginFor<T>,
			session_id: SessionId,
			amount: BalanceOf<T>,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Sessions::<T>::try_mutate(session_id, |maybe_session| -> DispatchResult {
				let session = maybe_session.as_mut().ok_or(Error::<T>::SessionNotFound)?;
				ensure!(session.requester == who, Error::<T>::NotRequester);
				ensure!(session.status == SessionStatus::Created, Error::<T>::InvalidSessionStatus);
				ensure!(
					session.settlement_mode == TradeSettlementMode::MainEscrow,
					Error::<T>::UnsupportedSettlementMode,
				);
				T::Currency::reserve(&who, amount)?;
				session.locked_funds = amount;
				session.status = SessionStatus::Funded;
				Ok(())
			})?;
			Self::deposit_event(Event::FundsLocked { session_id, requester: who, amount });
			Ok(())
		}

		#[pallet::call_index(2)]
		#[pallet::weight(T::WeightInfo::lock_deposit())]
		pub fn lock_deposit(
			origin: OriginFor<T>,
			session_id: SessionId,
			amount: BalanceOf<T>,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Sessions::<T>::try_mutate(session_id, |maybe_session| -> DispatchResult {
				let session = maybe_session.as_mut().ok_or(Error::<T>::SessionNotFound)?;
				ensure!(session.data_owner == who, Error::<T>::NotDataOwner);
				ensure!(session.status == SessionStatus::Funded, Error::<T>::InvalidSessionStatus);
				ensure!(
					session.settlement_mode == TradeSettlementMode::MainEscrow,
					Error::<T>::UnsupportedSettlementMode,
				);
				T::Currency::reserve(&who, amount)?;
				session.deposit = amount;
				session.status = SessionStatus::DepositLocked;
				Ok(())
			})?;
			Self::deposit_event(Event::DepositLocked { session_id, data_owner: who, amount });
			Ok(())
		}

		#[pallet::call_index(3)]
		#[pallet::weight(T::WeightInfo::claim_funds())]
		pub fn claim_funds(
			origin: OriginFor<T>,
			session_id: SessionId,
			pre_image: Vec<u8>,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let rounds = Sessions::<T>::try_mutate(
				session_id,
				|maybe_session| -> Result<u32, DispatchError> {
					let session = maybe_session.as_mut().ok_or(Error::<T>::SessionNotFound)?;
					ensure!(session.data_owner == who, Error::<T>::NotDataOwner);
					ensure!(
						session.status == SessionStatus::DepositLocked,
						Error::<T>::InvalidSessionStatus,
					);
					let rounds = Self::verify_hash_chain(
						&pre_image,
						session.hash_chain_end,
						session.max_rounds,
					)
					.ok_or(Error::<T>::InvalidHashPreimage)?;

					T::Currency::repatriate_reserved(
						&session.requester,
						&session.data_owner,
						session.locked_funds,
						BalanceStatus::Free,
					)?;
					T::Currency::unreserve(&session.data_owner, session.deposit);
					session.status = SessionStatus::Settled;
					Ok(rounds)
				},
			)?;
			Self::deposit_event(Event::FundsClaimed { session_id, data_owner: who, rounds });
			Ok(())
		}

		#[pallet::call_index(4)]
		#[pallet::weight(T::WeightInfo::punish())]
		pub fn punish(origin: OriginFor<T>, session_id: SessionId) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Sessions::<T>::try_mutate(session_id, |maybe_session| -> DispatchResult {
				let session = maybe_session.as_mut().ok_or(Error::<T>::SessionNotFound)?;
				ensure!(session.requester == who, Error::<T>::NotRequester);
				ensure!(
					session.status == SessionStatus::DepositLocked,
					Error::<T>::InvalidSessionStatus,
				);
				let _ = T::Currency::slash_reserved(&session.data_owner, session.deposit);
				T::Currency::unreserve(&session.requester, session.locked_funds);
				session.status = SessionStatus::Punished;
				Ok(())
			})?;
			Self::deposit_event(Event::SessionPunished { session_id });
			Ok(())
		}
	}

	impl<T: Config> Pallet<T> {
		pub fn verify_hash_chain(
			pre_image: &[u8],
			target: T::Hash,
			max_rounds: u32,
		) -> Option<u32> {
			let mut current = pre_image.to_vec();
			for round in 1..=max_rounds {
				let hashed = T::Hashing::hash(&current);
				if hashed == target {
					return Some(round);
				}
				current = hashed.encode();
			}
			None
		}
	}
}
