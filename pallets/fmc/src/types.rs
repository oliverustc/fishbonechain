use codec::{Decode, Encode, MaxEncodedLen};
use frame_support::{pallet_prelude::*, BoundedVec};
use scale_info::TypeInfo;

pub type TaskId = u32;

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct FundPool<Balance> {
	pub free: Balance,
	pub locked: Balance,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct TaskInfo<Balance> {
	pub target_chain: pallet_ccmc::types::ChainId,
	pub budget_per_epoch: Balance,
	pub status: TaskStatus,
	pub current_epoch: pallet_ccmc::types::EpochId,
	pub description: BoundedVec<u8, ConstU32<256>>,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum TaskStatus {
	Terminated,
	Activated,
	Waiting,
}
