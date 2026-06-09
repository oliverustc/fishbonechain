// BABE consensus runtime definition (--features babe builds).
// Included by lib.rs via include!() so types land in the crate root scope.
// pallet_aura is kept (always a dep) so SessionKeys { aura, grandpa } still resolves.
// pallet_babe handles block authoring; OnTimestampSet = () prevents AURA slot assertion.
#[frame_support::runtime]
mod runtime {
	#[runtime::runtime]
	#[runtime::derive(
		RuntimeCall,
		RuntimeEvent,
		RuntimeError,
		RuntimeOrigin,
		RuntimeFreezeReason,
		RuntimeHoldReason,
		RuntimeSlashReason,
		RuntimeLockId,
		RuntimeTask,
		RuntimeViewFunction
	)]
	pub struct Runtime;

	#[runtime::pallet_index(0)]
	pub type System = frame_system;

	#[runtime::pallet_index(1)]
	pub type Timestamp = pallet_timestamp;

	#[runtime::pallet_index(2)]
	pub type Aura = pallet_aura;

	#[runtime::pallet_index(11)]
	pub type Babe = pallet_babe;

	#[runtime::pallet_index(12)]
	pub type Authorship = pallet_authorship;

	#[runtime::pallet_index(3)]
	pub type Grandpa = pallet_grandpa;

	#[runtime::pallet_index(4)]
	pub type Balances = pallet_balances;

	#[runtime::pallet_index(5)]
	pub type TransactionPayment = pallet_transaction_payment;

	#[runtime::pallet_index(6)]
	pub type Sudo = pallet_sudo;

	#[runtime::pallet_index(7)]
	pub type Template = pallet_fishbone_template;

	#[runtime::pallet_index(8)]
	pub type Ccmc = pallet_ccmc;

	#[runtime::pallet_index(9)]
	pub type Fmc = pallet_fmc;

	#[runtime::pallet_index(10)]
	pub type Crowdsource = pallet_crowdsource;
}
