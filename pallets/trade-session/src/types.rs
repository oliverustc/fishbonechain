use codec::{Decode, DecodeWithMemTracking, Encode, MaxEncodedLen};
use frame_support::pallet_prelude::RuntimeDebug;
use scale_info::TypeInfo;

pub type SessionId = u32;
pub type ListingId = u32;
pub type EscrowId = u32;
pub type RoundIndex = u32;

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
	Requested,
	Accepted,
	Ready,
	InDelivery,
	SettlementClaimed,
	Settled,
	Disputed,
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
pub enum RoundStatus {
	Opened,
	PaymentProofSubmitted,
	DataProofSubmitted,
	DataProofVerified,
	DataProofRejected,
	ProofSigned,
	DataDelivered,
	PaymentPreimageSubmitted,
	Disputed,
	Completed,
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
	pub listing_id: ListingId,
	pub escrow_id: EscrowId,
	pub requester: AccountId,
	pub data_owner: AccountId,
	pub request_hash: Hash,
	pub price_per_round: Balance,
	pub max_rounds: u32,
	pub hash_chain_anchor: Hash,
	pub latest_payment_preimage: Option<Hash>,
	pub completed_rounds: u32,
	pub status: SessionStatus,
	pub settlement_mode: TradeSettlementMode,
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
pub struct RoundState<AccountId, Hash> {
	pub session_id: SessionId,
	pub round_index: RoundIndex,
	pub payment_commitment_hash: Hash,
	pub proof_hash: Option<Hash>,
	pub proof_signature_hash: Option<Hash>,
	pub delivered_data_hash: Option<Hash>,
	pub payment_preimage_hash: Option<Hash>,
	pub status: RoundStatus,
	pub last_actor: Option<AccountId>,
}
