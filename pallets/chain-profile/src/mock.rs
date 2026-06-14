use frame_support::{construct_runtime, derive_impl};
use sp_runtime::BuildStorage;

type Block = frame_system::mocking::MockBlock<Test>;

construct_runtime!(
	pub enum Test {
		System: frame_system,
		ChainProfile: crate,
	}
);

#[derive_impl(frame_system::config_preludes::TestDefaultConfig)]
impl frame_system::Config for Test {
	type Block = Block;
}

impl crate::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type WeightInfo = ();
}

pub fn new_test_ext() -> sp_io::TestExternalities {
	let storage = frame_system::GenesisConfig::<Test>::default().build_storage().unwrap();
	sp_io::TestExternalities::new(storage)
}
