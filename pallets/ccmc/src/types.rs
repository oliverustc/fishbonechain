use codec::{Decode, Encode, MaxEncodedLen};
use frame_support::{pallet_prelude::*, BoundedVec};
use scale_info::TypeInfo;

pub type ChainId = u32;
pub type EpochId = u64;

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct ChainInfo<AccountId, Balance> {
    pub creator: AccountId,
    pub name: BoundedVec<u8, ConstU32<64>>,
    pub miner_count: u32,
    pub min_miners: u32,
    pub deposit_required: Balance,
    pub status: ChainStatus,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum ChainStatus {
    Active,
    Terminated,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct MinerInfo<Balance> {
    pub deposit: Balance,
    pub status: MinerStatus,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum MinerStatus {
    Active,
    Slashed,
}
