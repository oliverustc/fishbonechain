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
}

impl WeightInfo for () {
	fn publish_data() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(20_000, 0)
	}

	fn update_imt_root() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(15_000, 0)
	}
}

#[frame_support::pallet]
pub mod pallet {
	use super::*;
	use frame_support::pallet_prelude::*;
	use frame_system::pallet_prelude::*;

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
		type WeightInfo: WeightInfo;
	}

	#[pallet::storage]
	pub type Listings<T: Config> =
		StorageMap<_, Blake2_128Concat, ListingId, DataListing<T::AccountId, T::Hash>>;

	#[pallet::storage]
	pub type NextListingId<T: Config> = StorageValue<_, ListingId, ValueQuery>;

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		DataPublished { listing_id: ListingId, owner: T::AccountId },
		ImtRootUpdated { listing_id: ListingId, new_root: T::Hash },
	}

	#[pallet::error]
	pub enum Error<T> {
		ListingIdOverflow,
		ListingNotFound,
		NotListingOwner,
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::publish_data())]
		pub fn publish_data(
			origin: OriginFor<T>,
			imt_root: T::Hash,
			description: DataDescription,
		) -> DispatchResult {
			let owner = ensure_signed(origin)?;
			let listing_id = NextListingId::<T>::get();
			let next_id = listing_id.checked_add(1).ok_or(Error::<T>::ListingIdOverflow)?;

			Listings::<T>::insert(
				listing_id,
				DataListing { owner: owner.clone(), imt_root, description },
			);
			NextListingId::<T>::put(next_id);
			Self::deposit_event(Event::DataPublished { listing_id, owner });
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
	}
}
