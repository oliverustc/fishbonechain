#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;
pub use types::*;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

pub mod types;

pub trait WeightInfo {
	fn publish_data() -> frame_support::weights::Weight;
	fn update_imt_root() -> frame_support::weights::Weight;
	fn set_listing_status() -> frame_support::weights::Weight;
}

impl WeightInfo for () {
	fn publish_data() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(20_000, 0)
	}

	fn update_imt_root() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(15_000, 0)
	}

	fn set_listing_status() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(10_000, 0)
	}
}

#[frame_support::pallet]
pub mod pallet {
	use super::*;
	use frame_support::{
		pallet_prelude::*,
		traits::Currency,
	};
	use frame_system::pallet_prelude::*;

	pub type BalanceOf<T> =
		<<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
		type Currency: Currency<Self::AccountId>;
		type WeightInfo: WeightInfo;
	}

	#[pallet::storage]
	pub type Listings<T: Config> =
		StorageMap<_, Blake2_128Concat, ListingId, DataListing<T::AccountId, BalanceOf<T>, T::Hash>>;

	#[pallet::storage]
	pub type NextListingId<T: Config> = StorageValue<_, ListingId, ValueQuery>;

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		DataPublished { listing_id: ListingId, owner: T::AccountId, price_per_round: BalanceOf<T>, max_rounds: u32 },
		ImtRootUpdated { listing_id: ListingId, new_root: T::Hash },
		ListingStatusChanged { listing_id: ListingId, status: ListingStatus },
	}

	#[pallet::error]
	pub enum Error<T> {
		ListingIdOverflow,
		ListingNotFound,
		NotListingOwner,
		InvalidTradeTerms,
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::publish_data())]
		pub fn publish_data(
			origin: OriginFor<T>,
			imt_root: T::Hash,
			description: DataDescription,
			price_per_round: BalanceOf<T>,
			max_rounds: u32,
			deposit_hint: BalanceOf<T>,
			request_schema_hash: T::Hash,
			proof_params_hash: T::Hash,
		) -> DispatchResult {
			let owner = ensure_signed(origin)?;
			ensure!(!price_per_round.is_zero(), Error::<T>::InvalidTradeTerms);
			ensure!(max_rounds > 0, Error::<T>::InvalidTradeTerms);
			ensure!(!deposit_hint.is_zero(), Error::<T>::InvalidTradeTerms);

			let listing_id = NextListingId::<T>::get();
			let next_id = listing_id.checked_add(1).ok_or(Error::<T>::ListingIdOverflow)?;

			Listings::<T>::insert(
				listing_id,
				DataListing {
					owner: owner.clone(),
					imt_root,
					description,
					price_per_round,
					max_rounds,
					deposit_hint,
					request_schema_hash,
					proof_params_hash,
					status: ListingStatus::Active,
				},
			);
			NextListingId::<T>::put(next_id);
			Self::deposit_event(Event::DataPublished { listing_id, owner, price_per_round, max_rounds });
			Ok(())
		}

		#[pallet::call_index(1)]
		#[pallet::weight(T::WeightInfo::update_imt_root())]
		pub fn update_imt_root(
			origin: OriginFor<T>,
			listing_id: ListingId,
			new_root: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Listings::<T>::try_mutate(listing_id, |maybe_listing| -> DispatchResult {
				let listing = maybe_listing.as_mut().ok_or(Error::<T>::ListingNotFound)?;
				ensure!(listing.owner == who, Error::<T>::NotListingOwner);
				listing.imt_root = new_root;
				Ok(())
			})?;
			Self::deposit_event(Event::ImtRootUpdated { listing_id, new_root });
			Ok(())
		}

		#[pallet::call_index(2)]
		#[pallet::weight(T::WeightInfo::set_listing_status())]
		pub fn set_listing_status(
			origin: OriginFor<T>,
			listing_id: ListingId,
			status: ListingStatus,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let status_clone = status.clone();
			Listings::<T>::try_mutate(listing_id, |maybe_listing| -> DispatchResult {
				let listing = maybe_listing.as_mut().ok_or(Error::<T>::ListingNotFound)?;
				ensure!(listing.owner == who, Error::<T>::NotListingOwner);
				listing.status = status;
				Ok(())
			})?;
			Self::deposit_event(Event::ListingStatusChanged { listing_id, status: status_clone });
			Ok(())
		}
	}
}

/// Let pallet-data-registry serve as a listing provider for pallet-trade-session.
impl<T: Config> pallet_trade_session::ListingProvider<T::AccountId, BalanceOf<T>, T::Hash>
	for Pallet<T>
{
	fn listing_exists(listing_id: pallet_trade_session::ListingId) -> bool {
		Listings::<T>::contains_key(listing_id)
	}

	fn listing_owner(
		listing_id: pallet_trade_session::ListingId,
	) -> Option<T::AccountId> {
		Listings::<T>::get(listing_id).map(|listing| listing.owner)
	}

	fn listing_active(listing_id: pallet_trade_session::ListingId) -> bool {
		matches!(
			Listings::<T>::get(listing_id).map(|listing| listing.status),
			Some(ListingStatus::Active)
		)
	}

	fn listing_terms(
		listing_id: pallet_trade_session::ListingId,
	) -> Option<(BalanceOf<T>, u32, BalanceOf<T>, T::Hash)> {
		Listings::<T>::get(listing_id).map(|listing| {
			(
				listing.price_per_round,
				listing.max_rounds,
				listing.deposit_hint,
				listing.proof_params_hash,
			)
		})
	}
}
