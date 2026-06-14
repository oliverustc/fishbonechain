use crate::{mock::*, Error, Event, Listings, NextListingId};
use frame_support::{assert_noop, assert_ok};

fn description() -> frame_support::BoundedVec<u8, frame_support::traits::ConstU32<512>> {
	b"temperature dataset".to_vec().try_into().unwrap()
}

#[test]
fn data_owner_can_publish_listing() {
	new_test_ext().execute_with(|| {
		let root = sp_core::H256::repeat_byte(1);

		assert_ok!(DataRegistry::publish_data(RuntimeOrigin::signed(1), root, description()));

		let listing = Listings::<Test>::get(0).expect("listing exists");
		assert_eq!(listing.owner, 1);
		assert_eq!(listing.imt_root, root);
		assert_eq!(listing.description, description());
		assert_eq!(NextListingId::<Test>::get(), 1);
		System::assert_has_event(Event::DataPublished { listing_id: 0, owner: 1 }.into());
	});
}

#[test]
fn listing_id_auto_increments() {
	new_test_ext().execute_with(|| {
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			sp_core::H256::repeat_byte(1),
			description(),
		));
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(2),
			sp_core::H256::repeat_byte(2),
			description(),
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
		assert_ok!(DataRegistry::publish_data(RuntimeOrigin::signed(1), old_root, description()));

		assert_ok!(DataRegistry::update_imt_root(RuntimeOrigin::signed(1), 0, new_root));

		assert_eq!(Listings::<Test>::get(0).unwrap().imt_root, new_root);
		System::assert_has_event(Event::ImtRootUpdated { listing_id: 0, new_root }.into());
	});
}

#[test]
fn non_owner_cannot_update_imt_root() {
	new_test_ext().execute_with(|| {
		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			sp_core::H256::repeat_byte(1),
			description(),
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
