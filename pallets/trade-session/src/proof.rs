use codec::{Decode, DecodeWithMemTracking, Encode, MaxEncodedLen};
use frame_support::pallet_prelude::RuntimeDebug;
use scale_info::TypeInfo;

use crate::types::ListingId;

/// Pluggable data-trade proof verifier.
/// Phase 1: mock verifier always passes (except plaintext hash check).
/// Phase 2: gnark-generated CH/RO proof verifier.
pub trait DataTradeProofVerifier<Hash> {
	fn verify_payment_commitment(previous: Hash, next: Hash, proof_hash: Hash) -> bool;
	fn verify_data_proof(proof_hash: Hash) -> bool;
	fn verify_plaintext_hash(data_hash: Hash, expected_hash: Hash) -> bool;
	fn verify_signature(proof_hash: Hash, signature_hash: Hash) -> bool;
}

/// Always-pass mock verifier (Phase 1).
pub struct AlwaysPassVerifier;
impl<Hash: PartialEq + Copy> DataTradeProofVerifier<Hash> for AlwaysPassVerifier {
	fn verify_payment_commitment(_: Hash, _: Hash, _: Hash) -> bool {
		true
	}
	fn verify_data_proof(_: Hash) -> bool {
		true
	}
	fn verify_plaintext_hash(data_hash: Hash, expected_hash: Hash) -> bool {
		data_hash == expected_hash
	}
	fn verify_signature(_: Hash, _: Hash) -> bool {
		true
	}
}

/// Listing provider trait — decouples trade-session from data-registry storage.
pub trait ListingProvider<AccountId, Balance, Hash> {
	fn listing_exists(listing_id: ListingId) -> bool;
	fn listing_owner(listing_id: ListingId) -> Option<AccountId>;
	fn listing_active(listing_id: ListingId) -> bool;
	fn listing_terms(listing_id: ListingId) -> Option<(Balance, u32, Balance, Hash)>;
}

/// No-op listing provider (returns false for all) — used as default/mock.
pub struct NoopListingProvider;
impl<AccountId, Balance, Hash> ListingProvider<AccountId, Balance, Hash>
	for NoopListingProvider
{
	fn listing_exists(_: ListingId) -> bool {
		false
	}
	fn listing_owner(_: ListingId) -> Option<AccountId> {
		None
	}
	fn listing_active(_: ListingId) -> bool {
		false
	}
	fn listing_terms(_: ListingId) -> Option<(Balance, u32, Balance, Hash)> {
		None
	}
}

/// Constraint kind for data trade proofs.
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
pub enum ConstraintKind {
	Range,
	Subset,
	Substr,
}

/// Proof bundle for one round of data trade.
/// Phase 1: stored as hashes only; mock verifier checks public_input_hash != default.
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
pub struct ProofBundle<Hash> {
	pub constraint_kind: ConstraintKind,
	pub ch_proof_hash: Hash,
	pub ro_proof_hash: Hash,
	pub public_input_hash: Hash,
}
