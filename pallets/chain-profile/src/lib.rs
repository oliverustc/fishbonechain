#![cfg_attr(not(feature = "std"), no_std)]

pub use pallet::*;
pub use types::*;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

pub mod types;

pub trait ChainIdentityProvider {
	fn chain_id() -> pallet_ccmc::types::ChainId;
	fn scene_kind() -> SceneKind;
	fn settlement_mode() -> SettlementMode;
}

pub trait WeightInfo {
	fn set_profile() -> frame_support::weights::Weight;
}

impl WeightInfo for () {
	fn set_profile() -> frame_support::weights::Weight {
		frame_support::weights::Weight::from_parts(10_000, 0)
	}
}

#[frame_support::pallet]
pub mod pallet {
	use super::*;
	use core::marker::PhantomData;
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
	pub type Profile<T: Config> = StorageValue<_, ChainProfileInfo, ValueQuery>;

	#[pallet::genesis_config]
	pub struct GenesisConfig<T: Config> {
		pub profile: ChainProfileInfo,
		#[serde(skip)]
		pub _phantom: PhantomData<T>,
	}

	impl<T: Config> Default for GenesisConfig<T> {
		fn default() -> Self {
			Self { profile: ChainProfileInfo::default(), _phantom: Default::default() }
		}
	}

	#[pallet::genesis_build]
	impl<T: Config> BuildGenesisConfig for GenesisConfig<T> {
		fn build(&self) {
			Profile::<T>::put(self.profile.clone());
		}
	}

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		ProfileUpdated { profile: ChainProfileInfo },
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::set_profile())]
		pub fn set_profile(origin: OriginFor<T>, profile: ChainProfileInfo) -> DispatchResult {
			frame_system::ensure_root(origin)?;
			Profile::<T>::put(profile.clone());
			Self::deposit_event(Event::ProfileUpdated { profile });
			Ok(())
		}
	}
}

impl<T: pallet::Config> ChainIdentityProvider for pallet::Pallet<T> {
	fn chain_id() -> pallet_ccmc::types::ChainId {
		pallet::Profile::<T>::get().chain_id
	}

	fn scene_kind() -> SceneKind {
		pallet::Profile::<T>::get().scene
	}

	fn settlement_mode() -> SettlementMode {
		pallet::Profile::<T>::get().settlement
	}
}
