use codec::{Decode, DecodeWithMemTracking, Encode, MaxEncodedLen};
use frame_support::pallet_prelude::RuntimeDebug;
use scale_info::TypeInfo;
use sp_core::H256;

pub type ChainId = u32;

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
	serde::Serialize,
	serde::Deserialize,
)]
pub enum SceneKind {
	PlatformOnly,
	Crowdsource,
	DataTrade,
	VerifiableTraining,
	ZkVmAnalytics,
	Custom,
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
	serde::Serialize,
	serde::Deserialize,
)]
pub enum SettlementMode {
	FmcTaskBill,
	MainEscrow,
	Hybrid,
	None,
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
	serde::Serialize,
	serde::Deserialize,
)]
pub struct ChainProfileInfo {
	pub chain_id: ChainId,
	pub scene: SceneKind,
	pub settlement: SettlementMode,
	pub params_hash: H256,
}

impl Default for ChainProfileInfo {
	fn default() -> Self {
		Self {
			chain_id: 0,
			scene: SceneKind::PlatformOnly,
			settlement: SettlementMode::None,
			params_hash: H256::zero(),
		}
	}
}
