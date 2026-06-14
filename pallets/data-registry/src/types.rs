use codec::{Decode, DecodeWithMemTracking, Encode, MaxEncodedLen};
use frame_support::{pallet_prelude::RuntimeDebug, BoundedVec};
use scale_info::TypeInfo;

pub type ListingId = u32;
pub type DataDescription = BoundedVec<u8, frame_support::traits::ConstU32<512>>;

#[derive(
	Encode,
	Decode,
	DecodeWithMemTracking,
	Clone,
	PartialEq,
	Eq,
	RuntimeDebug,
	TypeInfo,
	MaxEncodedLen,
)]
pub struct DataListing<AccountId, Hash> {
	pub owner: AccountId,
	pub imt_root: Hash,
	pub description: DataDescription,
}
