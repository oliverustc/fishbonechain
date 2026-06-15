use crate::{mock::*, types::ListingStatus, Error, Event, Listings, NextListingId};
use frame_support::{assert_noop, assert_ok};

fn description() -> frame_support::BoundedVec<u8, frame_support::traits::ConstU32<512>> {
	b"temperature dataset".to_vec().try_into().unwrap()
}

fn default_params() -> (sp_core::H256, sp_core::H256) {
	(sp_core::H256::repeat_byte(2), sp_core::H256::repeat_byte(3))
}

#[test]
fn data_owner_can_publish_listing() {
	new_test_ext().execute_with(|| {
		let root = sp_core::H256::repeat_byte(1);
		let (req_hash, proof_hash) = default_params();

		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			root,
			description(),
			1_000,
			10,
			5_000,
			req_hash,
			proof_hash,
		));

		let listing = Listings::<Test>::get(0).expect("listing exists");
		assert_eq!(listing.owner, 1);
		assert_eq!(listing.imt_root, root);
		assert_eq!(listing.description, description());
		assert_eq!(listing.price_per_round, 1_000);
		assert_eq!(listing.max_rounds, 10);
		assert_eq!(NextListingId::<Test>::get(), 1);
		System::assert_has_event(Event::DataPublished { listing_id: 0, owner: 1, price_per_round: 1_000, max_rounds: 10 }.into());
	});
}

#[test]
fn listing_id_auto_increments() {
	new_test_ext().execute_with(|| {
		let (req_hash, proof_hash) = default_params();
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			sp_core::H256::repeat_byte(1),
			description(),
			100,
			5,
			500,
			req_hash,
			proof_hash,
		));
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(2),
			sp_core::H256::repeat_byte(2),
			description(),
			100,
			5,
			500,
			req_hash,
			proof_hash,
		));

		assert!(Listings::<Test>::contains_key(0));
		assert!(Listings::<Test>::contains_key(1));
		assert_eq!(NextListingId::<Test>::get(), 2);
	});
}

#[test]
fn owner_can_update_imt_root() {
	new_test_ext().execute_with(|| {
		let old_root = sp_core::H256::repeat_byte(1);
		let new_root = sp_core::H256::repeat_byte(9);
		let (req_hash, proof_hash) = default_params();
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			old_root,
			description(),
			100,
			5,
			500,
			req_hash,
			proof_hash,
		));

		assert_ok!(DataRegistry::update_imt_root(RuntimeOrigin::signed(1), 0, new_root));

		assert_eq!(Listings::<Test>::get(0).unwrap().imt_root, new_root);
		System::assert_has_event(Event::ImtRootUpdated { listing_id: 0, new_root }.into());
	});
}

#[test]
fn non_owner_cannot_update_imt_root() {
	new_test_ext().execute_with(|| {
		let (req_hash, proof_hash) = default_params();
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			sp_core::H256::repeat_byte(1),
			description(),
			100,
			5,
			500,
			req_hash,
			proof_hash,
		));

		assert_noop!(
			DataRegistry::update_imt_root(
				RuntimeOrigin::signed(2),
				0,
				sp_core::H256::repeat_byte(9)
			),
			Error::<Test>::NotListingOwner
		);
	});
}

#[test]
fn publish_listing_includes_trade_terms() {
	new_test_ext().execute_with(|| {
		let root = sp_core::H256::repeat_byte(1);
		let request_schema_hash = sp_core::H256::repeat_byte(2);
		let proof_params_hash = sp_core::H256::repeat_byte(3);

		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			root,
			description(),
			1_000,
			10,
			5_000,
			request_schema_hash,
			proof_params_hash,
		));

		let listing = Listings::<Test>::get(0).expect("listing exists");
		assert_eq!(listing.owner, 1);
		assert_eq!(listing.imt_root, root);
		assert_eq!(listing.price_per_round, 1_000);
		assert_eq!(listing.max_rounds, 10);
		assert_eq!(listing.deposit_hint, 5_000);
		assert_eq!(listing.request_schema_hash, request_schema_hash);
		assert_eq!(listing.proof_params_hash, proof_params_hash);
	});
}

#[test]
fn publish_data_rejects_zero_price() {
	new_test_ext().execute_with(|| {
		let (req_hash, proof_hash) = default_params();
		assert_noop!(
			DataRegistry::publish_data(
				RuntimeOrigin::signed(1),
				sp_core::H256::repeat_byte(1),
				description(),
				0,
				10,
				5_000,
				req_hash,
				proof_hash,
			),
			Error::<Test>::InvalidTradeTerms
		);
	});
}

#[test]
fn publish_data_rejects_zero_rounds() {
	new_test_ext().execute_with(|| {
		let (req_hash, proof_hash) = default_params();
		assert_noop!(
			DataRegistry::publish_data(
				RuntimeOrigin::signed(1),
				sp_core::H256::repeat_byte(1),
				description(),
				1_000,
				0,
				5_000,
				req_hash,
				proof_hash,
			),
			Error::<Test>::InvalidTradeTerms
		);
	});
}

#[test]
fn publish_data_rejects_zero_deposit() {
	new_test_ext().execute_with(|| {
		let (req_hash, proof_hash) = default_params();
		assert_noop!(
			DataRegistry::publish_data(
				RuntimeOrigin::signed(1),
				sp_core::H256::repeat_byte(1),
				description(),
				1_000,
				10,
				0,
				req_hash,
				proof_hash,
			),
			Error::<Test>::InvalidTradeTerms
		);
	});
}

#[test]
fn owner_can_suspend_and_retire_listing() {
	new_test_ext().execute_with(|| {
		let (req_hash, proof_hash) = default_params();
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			sp_core::H256::repeat_byte(1),
			description(),
			100,
			5,
			500,
			req_hash,
			proof_hash,
		));

		assert_ok!(DataRegistry::set_listing_status(
			RuntimeOrigin::signed(1),
			0,
			ListingStatus::Suspended,
		));
		assert_eq!(Listings::<Test>::get(0).unwrap().status, ListingStatus::Suspended);
		System::assert_has_event(Event::ListingStatusChanged { listing_id: 0, status: ListingStatus::Suspended }.into());

		assert_ok!(DataRegistry::set_listing_status(
			RuntimeOrigin::signed(1),
			0,
			ListingStatus::Retired,
		));
		assert_eq!(Listings::<Test>::get(0).unwrap().status, ListingStatus::Retired);
	});
}

#[test]
fn non_owner_cannot_change_listing_status() {
	new_test_ext().execute_with(|| {
		let (req_hash, proof_hash) = default_params();
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			sp_core::H256::repeat_byte(1),
			description(),
			100,
			5,
			500,
			req_hash,
			proof_hash,
		));

		assert_noop!(
			DataRegistry::set_listing_status(RuntimeOrigin::signed(2), 0, ListingStatus::Suspended),
			Error::<Test>::NotListingOwner
		);
	});
}
