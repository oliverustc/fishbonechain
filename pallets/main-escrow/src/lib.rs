#![cfg_attr(not(feature = "std"), no_std)]

extern crate alloc;

pub use pallet::*;
pub use types::*;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

pub mod types;

#[frame_support::pallet]
pub mod pallet {
	use alloc::vec::Vec;
	use codec::Encode;
	use frame_support::{
		pallet_prelude::*,
		traits::{Currency, ReservableCurrency},
	};
	use frame_system::pallet_prelude::*;
	use sp_runtime::traits::Hash;

	use crate::types::*;

	pub type BalanceOf<T> =
		<<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

	pub trait WeightInfo {
		fn open_escrow() -> frame_support::weights::Weight;
		fn lock_funds() -> frame_support::weights::Weight;
		fn lock_deposit() -> frame_support::weights::Weight;
		fn settle_by_preimage() -> frame_support::weights::Weight;
		fn punish_data_owner() -> frame_support::weights::Weight;
		fn claim_last_payment() -> frame_support::weights::Weight;
	}

	impl WeightInfo for () {
		fn open_escrow() -> Weight {
			Weight::from_parts(20_000, 0)
		}
		fn lock_funds() -> Weight {
			Weight::from_parts(20_000, 0)
		}
		fn lock_deposit() -> Weight {
			Weight::from_parts(20_000, 0)
		}
		fn settle_by_preimage() -> Weight {
			Weight::from_parts(50_000, 0)
		}
		fn punish_data_owner() -> Weight {
			Weight::from_parts(20_000, 0)
		}
		fn claim_last_payment() -> Weight {
			Weight::from_parts(30_000, 0)
		}
	}

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
		type Currency: ReservableCurrency<Self::AccountId>;
		type WeightInfo: WeightInfo;
	}

	#[pallet::storage]
	pub type Escrows<T: Config> =
		StorageMap<_, Blake2_128Concat, EscrowId, Escrow<T::AccountId, BalanceOf<T>, T::Hash>>;

	#[pallet::storage]
	pub type NextEscrowId<T: Config> = StorageValue<_, EscrowId, ValueQuery>;

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		EscrowOpened { escrow_id: EscrowId, requester: T::AccountId, data_owner: T::AccountId },
		FundsLocked { escrow_id: EscrowId, amount: BalanceOf<T> },
		DepositLocked { escrow_id: EscrowId, amount: BalanceOf<T> },
		EscrowSettled { escrow_id: EscrowId, paid_rounds: u32, refunded: BalanceOf<T> },
		EscrowPunished { escrow_id: EscrowId, slashed_deposit: BalanceOf<T> },
	}

	#[pallet::error]
	pub enum Error<T> {
		EscrowIdOverflow,
		EscrowNotFound,
		NotRequester,
		NotDataOwner,
		InvalidEscrowStatus,
		InvalidHashPreimage,
		InvalidRemainingRounds,
		InvalidTradeTerms,
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::open_escrow())]
		pub fn open_escrow(
			origin: OriginFor<T>,
			data_owner: T::AccountId,
			max_rounds: u32,
			price_per_round: BalanceOf<T>,
			deposit: BalanceOf<T>,
			hash_chain_anchor: T::Hash,
		) -> DispatchResult {
			let requester = ensure_signed(origin)?;
			ensure!(max_rounds > 0, Error::<T>::InvalidTradeTerms);
			ensure!(!price_per_round.is_zero(), Error::<T>::InvalidTradeTerms);
			ensure!(!deposit.is_zero(), Error::<T>::InvalidTradeTerms);

			let escrow_id = NextEscrowId::<T>::get();
			let next_id = escrow_id.checked_add(1).ok_or(Error::<T>::EscrowIdOverflow)?;

			let total_funds = price_per_round
				.checked_mul(&(max_rounds as u32).into())
				.ok_or(Error::<T>::InvalidTradeTerms)?;

			let data_owner_for_event = data_owner.clone();
			Escrows::<T>::insert(
				escrow_id,
				Escrow {
					requester: requester.clone(),
					data_owner,
					max_rounds,
					price_per_round,
					total_funds,
					deposit,
					hash_chain_anchor,
					paid_rounds: 0,
					status: EscrowStatus::Opened,
				},
			);
			NextEscrowId::<T>::put(next_id);
			Self::deposit_event(Event::EscrowOpened {
				escrow_id,
				requester,
				data_owner: data_owner_for_event,
			});
			Ok(())
		}

		#[pallet::call_index(1)]
		#[pallet::weight(T::WeightInfo::lock_funds())]
		pub fn lock_funds(origin: OriginFor<T>, escrow_id: EscrowId) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Escrows::<T>::try_mutate(escrow_id, |maybe_escrow| -> DispatchResult {
				let escrow = maybe_escrow.as_mut().ok_or(Error::<T>::EscrowNotFound)?;
				ensure!(escrow.requester == who, Error::<T>::NotRequester);
				ensure!(escrow.status == EscrowStatus::Opened, Error::<T>::InvalidEscrowStatus);
				let amount = escrow.total_funds;
				T::Currency::reserve(&who, amount)?;
				escrow.status = EscrowStatus::Funded;
				Ok(())
			})?;
			Self::deposit_event(Event::FundsLocked {
				escrow_id,
				amount: Escrows::<T>::get(escrow_id).unwrap().total_funds,
			});
			Ok(())
		}

		#[pallet::call_index(2)]
		#[pallet::weight(T::WeightInfo::lock_deposit())]
		pub fn lock_deposit(origin: OriginFor<T>, escrow_id: EscrowId) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Escrows::<T>::try_mutate(escrow_id, |maybe_escrow| -> DispatchResult {
				let escrow = maybe_escrow.as_mut().ok_or(Error::<T>::EscrowNotFound)?;
				ensure!(escrow.data_owner == who, Error::<T>::NotDataOwner);
				ensure!(escrow.status == EscrowStatus::Funded, Error::<T>::InvalidEscrowStatus);
				let deposit = escrow.deposit;
				T::Currency::reserve(&who, deposit)?;
				escrow.status = EscrowStatus::Ready;
				Ok(())
			})?;
			Self::deposit_event(Event::DepositLocked {
				escrow_id,
				amount: Escrows::<T>::get(escrow_id).unwrap().deposit,
			});
			Ok(())
		}

		#[pallet::call_index(3)]
		#[pallet::weight(T::WeightInfo::settle_by_preimage())]
		pub fn settle_by_preimage(
			origin: OriginFor<T>,
			escrow_id: EscrowId,
			pre_image: Vec<u8>,
			remaining_rounds: u32,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Escrows::<T>::try_mutate(escrow_id, |maybe_escrow| -> DispatchResult {
				let escrow = maybe_escrow.as_mut().ok_or(Error::<T>::EscrowNotFound)?;
				ensure!(escrow.data_owner == who, Error::<T>::NotDataOwner);
				ensure!(escrow.status == EscrowStatus::Ready, Error::<T>::InvalidEscrowStatus);
				ensure!(remaining_rounds < escrow.max_rounds, Error::<T>::InvalidRemainingRounds,);

				let paid_rounds = escrow.max_rounds - remaining_rounds;
				let _claimed_hash =
					Self::verify_hash_chain(&pre_image, escrow.hash_chain_anchor, paid_rounds)
						.ok_or(Error::<T>::InvalidHashPreimage)?;

				let payment = escrow
					.price_per_round
					.checked_mul(&(paid_rounds as u32).into())
					.ok_or(Error::<T>::InvalidTradeTerms)?;
				let refund_amount = escrow
					.total_funds
					.checked_sub(&payment)
					.ok_or(Error::<T>::InvalidTradeTerms)?;

				T::Currency::repatriate_reserved(
					&escrow.requester,
					&escrow.data_owner,
					payment,
					frame_support::traits::BalanceStatus::Free,
				)?;

				if !refund_amount.is_zero() {
					T::Currency::unreserve(&escrow.requester, refund_amount);
				}

				T::Currency::unreserve(&escrow.data_owner, escrow.deposit);

				escrow.paid_rounds = paid_rounds;
				escrow.status = EscrowStatus::Settled;
				Ok(())
			})?;
			let escrow = Escrows::<T>::get(escrow_id).unwrap();
			let refunded = escrow
				.price_per_round
				.checked_mul(&(escrow.paid_rounds as u32).into())
				.and_then(|payment| escrow.total_funds.checked_sub(&payment))
				.unwrap_or_default();
			Self::deposit_event(Event::EscrowSettled {
				escrow_id,
				paid_rounds: escrow.paid_rounds,
				refunded,
			});
			Ok(())
		}

		#[pallet::call_index(4)]
		#[pallet::weight(T::WeightInfo::punish_data_owner())]
		pub fn punish_data_owner(origin: OriginFor<T>, escrow_id: EscrowId) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Escrows::<T>::try_mutate(escrow_id, |maybe_escrow| -> DispatchResult {
				let escrow = maybe_escrow.as_mut().ok_or(Error::<T>::EscrowNotFound)?;
				ensure!(escrow.requester == who, Error::<T>::NotRequester);
				ensure!(escrow.status == EscrowStatus::Ready, Error::<T>::InvalidEscrowStatus);

				let deposit = escrow.deposit;
				T::Currency::repatriate_reserved(
					&escrow.data_owner,
					&escrow.requester,
					deposit,
					frame_support::traits::BalanceStatus::Free,
				)?;
				T::Currency::unreserve(&escrow.requester, escrow.total_funds);
				escrow.status = EscrowStatus::Punished;
				Ok(())
			})?;
			Self::deposit_event(Event::EscrowPunished {
				escrow_id,
				slashed_deposit: Escrows::<T>::get(escrow_id).unwrap().deposit,
			});
			Ok(())
		}

		#[pallet::call_index(5)]
		#[pallet::weight(T::WeightInfo::claim_last_payment())]
		pub fn claim_last_payment(
			origin: OriginFor<T>,
			escrow_id: EscrowId,
			_round_index: u32,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Escrows::<T>::try_mutate(escrow_id, |maybe_escrow| -> DispatchResult {
				let escrow = maybe_escrow.as_mut().ok_or(Error::<T>::EscrowNotFound)?;
				ensure!(escrow.data_owner == who, Error::<T>::NotDataOwner);
				ensure!(escrow.status == EscrowStatus::Ready, Error::<T>::InvalidEscrowStatus,);

				let payment = escrow.price_per_round;
				let refund = escrow
					.total_funds
					.checked_sub(&payment)
					.ok_or(Error::<T>::InvalidTradeTerms)?;

				T::Currency::repatriate_reserved(
					&escrow.requester,
					&escrow.data_owner,
					payment,
					frame_support::traits::BalanceStatus::Free,
				)?;
				if !refund.is_zero() {
					T::Currency::unreserve(&escrow.requester, refund);
				}
				T::Currency::unreserve(&escrow.data_owner, escrow.deposit);
				escrow.paid_rounds += 1;
				escrow.status = EscrowStatus::Settled;
				Ok(())
			})?;
			let escrow = Escrows::<T>::get(escrow_id).unwrap();
			let refunded = escrow
				.price_per_round
				.checked_mul(&(escrow.paid_rounds as u32).into())
				.and_then(|payment| escrow.total_funds.checked_sub(&payment))
				.unwrap_or_default();
			Self::deposit_event(Event::EscrowSettled {
				escrow_id,
				paid_rounds: escrow.paid_rounds,
				refunded,
			});
			Ok(())
		}
	}

	impl<T: Config> Pallet<T> {
		pub fn verify_hash_chain(pre_image: &[u8], target: T::Hash, max: u32) -> Option<u32> {
			let mut current = pre_image.to_vec();
			for round in 1..=max {
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
