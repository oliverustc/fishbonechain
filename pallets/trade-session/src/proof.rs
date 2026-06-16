use alloc::vec::Vec;
use codec::{Decode, DecodeWithMemTracking, Encode, MaxEncodedLen};
use frame_support::pallet_prelude::RuntimeDebug;
use scale_info::TypeInfo;
use sp_runtime::traits::Hash;

use crate::types::{ConstraintKind, ListingId, ProofSystem};

/// Pluggable data-trade proof verifier.
pub trait DataTradeProofVerifier<Hash> {
	fn verify_payment_commitment(previous: Hash, next: Hash, proof_hash: Hash) -> bool;
	fn verify_data_proof(proof_hash: Hash) -> bool;
	fn verify_plaintext_hash(data_hash: Hash, expected_hash: Hash) -> bool;
	fn verify_signature(proof_hash: Hash, signature_hash: Hash) -> bool;
}

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

/// Listing provider trait.
pub trait ListingProvider<AccountId, Balance, Hash> {
	fn listing_exists(listing_id: ListingId) -> bool;
	fn listing_owner(listing_id: ListingId) -> Option<AccountId>;
	fn listing_active(listing_id: ListingId) -> bool;
	fn listing_terms(listing_id: ListingId) -> Option<(Balance, u32, Balance, Hash)>;
}

pub struct NoopListingProvider;
impl<AccountId, Balance, Hash> ListingProvider<AccountId, Balance, Hash> for NoopListingProvider {
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

/// Proof bundle for one round of data trade.
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
	pub proof_system: ProofSystem,
	pub constraint_kind: ConstraintKind,
	pub ch_proof_hash: Hash,
	pub ro_proof_hash: Hash,
	pub public_input_hash: Hash,
	pub vk_hash: Hash,
	pub ro_depth: u32,
}

/// Compute proof digest matching Go/JS `FISHBONE:DATA_TRADE:ZK_PROOF:v1`.
pub fn compute_zk_proof_digest<H: Hash>(
	proof_system: ProofSystem,
	constraint_kind: ConstraintKind,
	ro_depth: u32,
	request_hash: H::Output,
	session_id: u32,
	round_index: u32,
	vk_hash: H::Output,
	ch_proof_hash: H::Output,
	ro_proof_hash: H::Output,
	public_input_hash: H::Output,
) -> H::Output {
	let mut data = Vec::new();
	data.extend(b"FISHBONE:DATA_TRADE:ZK_PROOF:v1");
	data.push(proof_system.code());
	data.push(constraint_kind.code());
	data.extend(&ro_depth.to_le_bytes());
	data.extend(request_hash.as_ref());
	data.extend(&session_id.to_le_bytes());
	data.extend(&round_index.to_le_bytes());
	data.extend(vk_hash.as_ref());
	data.extend(ch_proof_hash.as_ref());
	data.extend(ro_proof_hash.as_ref());
	data.extend(public_input_hash.as_ref());
	<H as Hash>::hash(&data)
}

/// Compute attestation payload digest matching Go/JS `FISHBONE:DATA_TRADE:ZK_ATTEST:v1`.
pub fn compute_zk_attestation_digest<H: Hash>(
	session_id: u32,
	round_index: u32,
	proof_digest: H::Output,
	accepted: bool,
	verifier_account: &[u8],
) -> H::Output {
	let mut data = Vec::new();
	data.extend(b"FISHBONE:DATA_TRADE:ZK_ATTEST:v1");
	data.extend(&session_id.to_le_bytes());
	data.extend(&round_index.to_le_bytes());
	data.extend(proof_digest.as_ref());
	data.push(if accepted { 1 } else { 0 });
	data.extend(verifier_account);
	<H as Hash>::hash(&data)
}
