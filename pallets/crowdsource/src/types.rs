use codec::{Decode, Encode, MaxEncodedLen};
use frame_support::{pallet_prelude::*, BoundedVec};
use scale_info::TypeInfo;

// 类型别名：与主链 pallet 对齐，直接引用其 types 模块
pub use pallet_ccmc::types::{ChainId, EpochId};
pub use pallet_fmc::types::TaskId;

/// 从主链同步来的任务信息（子链本地副本）
#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct TaskDetail<AccountId, Balance> {
	pub requester: AccountId,
	pub budget_per_epoch: Balance,
	pub description: BoundedVec<u8, ConstU32<256>>,
	pub status: TaskStatus,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum TaskStatus {
	Active,    // 有预算，接受提交
	Exhausted, // 本 epoch 预算已耗尽
}

/// 工作者的数据提交记录（SCALE 编码后作为 Merkle 叶节点）
#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct Submission<AccountId, Balance> {
	pub task_id: TaskId,
	pub worker: AccountId,
	pub reward: Balance,
	pub data: BoundedVec<u8, ConstU32<1024>>,
}

/// 当前 Epoch 状态
#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen, Default)]
pub struct EpochInfo {
	pub epoch_id: EpochId,
	pub phase: EpochPhase,
	pub start_block: u32,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen, Default)]
pub enum EpochPhase {
	#[default]
	Collecting, // S_c：接受数据提交
	Syncing, // S_s：结算中，不接受新提交
}
