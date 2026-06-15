use codec::{Decode, DecodeWithMemTracking, Encode, MaxEncodedLen};
use frame_support::pallet_prelude::RuntimeDebug;
use scale_info::TypeInfo;

pub type EscrowId = u32;

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
pub enum EscrowStatus {
	Opened,
	Funded,
	Ready,
	Settled,
	Punished,
	Cancelled,
}

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
pub struct Escrow<AccountId, Balance, Hash> {
	pub requester: AccountId,
	pub data_owner: AccountId,
	pub max_rounds: u32,
	pub price_per_round: Balance,
	pub total_funds: Balance,
	pub deposit: Balance,
	pub hash_chain_anchor: Hash,
	pub paid_rounds: u32,
	pub status: EscrowStatus,
}
