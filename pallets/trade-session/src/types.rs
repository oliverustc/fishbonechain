use codec::{Decode, DecodeWithMemTracking, Encode, MaxEncodedLen};
use frame_support::pallet_prelude::RuntimeDebug;
use scale_info::TypeInfo;

pub type SessionId = u32;

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
pub enum SessionStatus {
	Created,
	Funded,
	DepositLocked,
	Settled,
	Punished,
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
pub enum TradeSettlementMode {
	MainEscrow,
	FmcAssisted,
	Hybrid,
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
pub struct TradingSession<AccountId, Balance, Hash> {
	pub requester: AccountId,
	pub data_owner: AccountId,
	pub hash_chain_end: Hash,
	pub max_rounds: u32,
	pub locked_funds: Balance,
	pub deposit: Balance,
	pub status: SessionStatus,
	pub settlement_mode: TradeSettlementMode,
}
