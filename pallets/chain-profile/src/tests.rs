use crate::{mock::*, ChainIdentityProvider, ChainProfileInfo, Profile, SceneKind, SettlementMode};
use frame_support::{assert_noop, assert_ok};
use sp_core::H256;

fn profile(chain_id: u32, scene: SceneKind, settlement: SettlementMode) -> ChainProfileInfo {
	ChainProfileInfo { chain_id, scene, settlement, params_hash: H256::repeat_byte(7) }
}

#[test]
fn genesis_profile_is_readable() {
	new_test_ext().execute_with(|| {
		let stored = Profile::<Test>::get();

		assert_eq!(stored.chain_id, 0);
		assert_eq!(stored.scene, SceneKind::PlatformOnly);
		assert_eq!(stored.settlement, SettlementMode::None);
		assert_eq!(stored.params_hash, H256::zero());
		assert_eq!(ChainProfile::chain_id(), 0);
		assert_eq!(ChainProfile::scene_kind(), SceneKind::PlatformOnly);
		assert_eq!(ChainProfile::settlement_mode(), SettlementMode::None);
	});
}

#[test]
fn root_can_update_profile() {
	new_test_ext().execute_with(|| {
		let next = profile(5, SceneKind::DataTrade, SettlementMode::MainEscrow);

		assert_ok!(ChainProfile::set_profile(RuntimeOrigin::root(), next.clone()));

		assert_eq!(Profile::<Test>::get(), next);
		assert_eq!(ChainProfile::chain_id(), 5);
		assert_eq!(ChainProfile::scene_kind(), SceneKind::DataTrade);
		assert_eq!(ChainProfile::settlement_mode(), SettlementMode::MainEscrow);
	});
}

#[test]
fn signed_update_fails() {
	new_test_ext().execute_with(|| {
		let next = profile(9, SceneKind::Crowdsource, SettlementMode::FmcTaskBill);

		assert_noop!(
			ChainProfile::set_profile(RuntimeOrigin::signed(1), next),
			sp_runtime::DispatchError::BadOrigin
		);

		assert_eq!(Profile::<Test>::get().chain_id, 0);
	});
}
