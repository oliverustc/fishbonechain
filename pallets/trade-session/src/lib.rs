#![cfg_attr(not(feature = "std"), no_std)]

extern crate alloc;

#[cfg(test)]
mod mock;

#[cfg(test)]
mod tests;

pub mod proof;
pub mod types;

pub use pallet::*;
pub use proof::{AlwaysPassVerifier, DataTradeProofVerifier, ListingProvider, NoopListingProvider};
pub use types::*;

pub type BalanceOf<T> =
	<<T as Config>::Currency as frame_support::traits::Currency<<T as frame_system::Config>::AccountId>>::Balance;

#[frame_support::pallet]
pub mod pallet {
	use frame_support::{
		pallet_prelude::*,
		traits::{Get, ReservableCurrency},
	};
	use frame_system::pallet_prelude::*;

	use crate::proof::*;
	use crate::types::*;

	pub use crate::BalanceOf;

	pub trait WeightInfo {
		fn create_session() -> frame_support::weights::Weight;
		fn accept_session() -> frame_support::weights::Weight;
		fn open_round() -> frame_support::weights::Weight;
		fn submit_payment_proof() -> frame_support::weights::Weight;
		fn submit_data_proof() -> frame_support::weights::Weight;
		fn submit_proof_signature() -> frame_support::weights::Weight;
		fn submit_data_delivery_hash() -> frame_support::weights::Weight;
		fn submit_payment_preimage() -> frame_support::weights::Weight;
		fn claim_settlement() -> frame_support::weights::Weight;
		fn dispute_invalid_proof() -> frame_support::weights::Weight;
		fn dispute_invalid_plaintext() -> frame_support::weights::Weight;
		fn claim_last_payment() -> frame_support::weights::Weight;
		fn attest_data_proof() -> frame_support::weights::Weight;
	}

	impl WeightInfo for () {
		fn create_session() -> Weight { Weight::from_parts(20_000, 0) }
		fn accept_session() -> Weight { Weight::from_parts(10_000, 0) }
		fn open_round() -> Weight { Weight::from_parts(15_000, 0) }
		fn submit_payment_proof() -> Weight { Weight::from_parts(20_000, 0) }
		fn submit_data_proof() -> Weight { Weight::from_parts(20_000, 0) }
		fn submit_proof_signature() -> Weight { Weight::from_parts(15_000, 0) }
		fn submit_data_delivery_hash() -> Weight { Weight::from_parts(15_000, 0) }
		fn submit_payment_preimage() -> Weight { Weight::from_parts(15_000, 0) }
		fn claim_settlement() -> Weight { Weight::from_parts(50_000, 0) }
		fn dispute_invalid_proof() -> Weight { Weight::from_parts(30_000, 0) }
		fn dispute_invalid_plaintext() -> Weight { Weight::from_parts(30_000, 0) }
		fn claim_last_payment() -> Weight { Weight::from_parts(30_000, 0) }
		fn attest_data_proof() -> Weight { Weight::from_parts(15_000, 0) }
	}

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
		type Currency: ReservableCurrency<Self::AccountId>;
		type ListingProvider: ListingProvider<Self::AccountId, BalanceOf<Self>, Self::Hash>;
		type ProofVerifier: DataTradeProofVerifier<Self::Hash>;
		type VerifierAuthority: Get<Self::AccountId>;
		type WeightInfo: WeightInfo;
	}

	#[pallet::storage]
	pub type Sessions<T: Config> = StorageMap<
		_,
		Blake2_128Concat,
		SessionId,
		TradingSession<T::AccountId, BalanceOf<T>, T::Hash>,
	>;

	#[pallet::storage]
	pub type Rounds<T: Config> = StorageDoubleMap<
		_,
		Blake2_128Concat,
		SessionId,
		Blake2_128Concat,
		RoundIndex,
		RoundState<T::AccountId, T::Hash>,
	>;

	#[pallet::storage]
	pub type NextSessionId<T: Config> = StorageValue<_, SessionId, ValueQuery>;

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T: Config> {
		SessionCreated {
			session_id: SessionId,
			requester: T::AccountId,
			data_owner: T::AccountId,
			listing_id: ListingId,
			escrow_id: EscrowId,
		},
		SessionAccepted { session_id: SessionId },
		RoundOpened { session_id: SessionId, round_index: RoundIndex },
		PaymentProofSubmitted { session_id: SessionId, round_index: RoundIndex },
		DataProofSubmitted { session_id: SessionId, round_index: RoundIndex },
		ProofSignatureSubmitted { session_id: SessionId, round_index: RoundIndex },
		DataDelivered { session_id: SessionId, round_index: RoundIndex },
		PaymentPreimageSubmitted { session_id: SessionId, round_index: RoundIndex },
		RoundCompleted { session_id: SessionId, round_index: RoundIndex },
		SettlementClaimed {
			session_id: SessionId,
			actor: T::AccountId,
			remaining_rounds: u32,
		},
		SessionPunished { session_id: SessionId },
		DataProofAttested { session_id: SessionId, round_index: RoundIndex, accepted: bool },
		LastPaymentClaimed { session_id: SessionId, actor: T::AccountId, round_index: RoundIndex },
	}

	#[pallet::error]
	pub enum Error<T> {
		SessionIdOverflow,
		SessionNotFound,
		RoundNotFound,
		NotRequester,
		NotDataOwner,
		InvalidSessionStatus,
		InvalidRoundStatus,
		ListingNotFound,
		ListingNotActive,
		ListingOwnerMismatch,
		ListingTermsMismatch,
		UnsupportedSettlementMode,
		RoundStepsOutOfOrder,
		InvalidHashPreimage,
		SettlementRoundsExceedCompleted,
		NotVerifier,
		InvalidProofAttestation,
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		// ── Phase 2: create session ──────────────────────────────────────────

		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::create_session())]
		pub fn create_session(
			origin: OriginFor<T>,
			listing_id: ListingId,
			escrow_id: EscrowId,
			data_owner: T::AccountId,
			request_hash: T::Hash,
			price_per_round: BalanceOf<T>,
			max_rounds: u32,
			hash_chain_anchor: T::Hash,
			settlement_mode: TradeSettlementMode,
		) -> DispatchResult {
			let requester = ensure_signed(origin)?;

			// Validate listing
			ensure!(
				T::ListingProvider::listing_exists(listing_id),
				Error::<T>::ListingNotFound
			);
			ensure!(
				T::ListingProvider::listing_active(listing_id),
				Error::<T>::ListingNotActive
			);
			ensure!(
				T::ListingProvider::listing_owner(listing_id) == Some(data_owner.clone()),
				Error::<T>::ListingOwnerMismatch
			);
			let (listing_price, listing_rounds, _listing_deposit, _listing_proof_hash) =
				T::ListingProvider::listing_terms(listing_id)
					.ok_or(Error::<T>::ListingNotFound)?;
			ensure!(listing_price == price_per_round, Error::<T>::ListingTermsMismatch);
			ensure!(listing_rounds == max_rounds, Error::<T>::ListingTermsMismatch);

			ensure!(
				settlement_mode == TradeSettlementMode::MainEscrow,
				Error::<T>::UnsupportedSettlementMode,
			);

			let session_id = NextSessionId::<T>::get();
			let next_id = session_id.checked_add(1).ok_or(Error::<T>::SessionIdOverflow)?;

			let data_owner_for_event = data_owner.clone();
			Sessions::<T>::insert(
				session_id,
				TradingSession {
					listing_id,
					escrow_id,
					requester: requester.clone(),
					data_owner,
					request_hash,
					price_per_round,
					max_rounds,
					hash_chain_anchor,
					latest_payment_preimage: None,
					completed_rounds: 0,
					status: SessionStatus::Requested,
					settlement_mode,
				},
			);
			NextSessionId::<T>::put(next_id);
			Self::deposit_event(Event::SessionCreated {
				session_id,
				requester,
				data_owner: data_owner_for_event,
				listing_id,
				escrow_id,
			});
			Ok(())
		}

		// ── Phase 2: DO accepts ─────────────────────────────────────────────

		#[pallet::call_index(1)]
		#[pallet::weight(T::WeightInfo::accept_session())]
		pub fn accept_session(
			origin: OriginFor<T>,
			session_id: SessionId,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Sessions::<T>::try_mutate(session_id, |maybe_session| -> DispatchResult {
				let session = maybe_session.as_mut().ok_or(Error::<T>::SessionNotFound)?;
				ensure!(session.data_owner == who, Error::<T>::NotDataOwner);
				ensure!(session.status == SessionStatus::Requested, Error::<T>::InvalidSessionStatus);
				session.status = SessionStatus::Accepted;
				Ok(())
			})?;
			Self::deposit_event(Event::SessionAccepted { session_id });
			Ok(())
		}

		// ── Phase 3: open round ──────────────────────────────────────────────

		#[pallet::call_index(2)]
		#[pallet::weight(T::WeightInfo::open_round())]
		pub fn open_round(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			payment_commitment_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.requester == who, Error::<T>::NotRequester);
			ensure!(
				session.status == SessionStatus::Accepted
					|| session.status == SessionStatus::Ready
					|| session.status == SessionStatus::InDelivery,
				Error::<T>::InvalidSessionStatus,
			);
			ensure!(round_index < session.max_rounds, Error::<T>::RoundStepsOutOfOrder);

			Rounds::<T>::insert(
				session_id,
				round_index,
				RoundState {
					session_id,
					round_index,
					payment_commitment_hash,
					proof_hash: None,
					proof_signature_hash: None,
					delivered_data_hash: None,
					payment_preimage_hash: None,
					status: RoundStatus::Opened,
					last_actor: Some(who),
				},
			);

			// Transition session to InDelivery
			Sessions::<T>::mutate(session_id, |maybe_session| {
				if let Some(s) = maybe_session {
					if s.status == SessionStatus::Accepted || s.status == SessionStatus::Ready {
						s.status = SessionStatus::InDelivery;
					}
				}
			});

			Self::deposit_event(Event::RoundOpened { session_id, round_index });
			Ok(())
		}

		// ── Phase 3: DR submits payment proof ────────────────────────────────

		#[pallet::call_index(3)]
		#[pallet::weight(T::WeightInfo::submit_payment_proof())]
		pub fn submit_payment_proof(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			proof_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.requester == who, Error::<T>::NotRequester);
			ensure!(session.status == SessionStatus::InDelivery, Error::<T>::InvalidSessionStatus);

			Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
				let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
				ensure!(round.status == RoundStatus::Opened, Error::<T>::RoundStepsOutOfOrder);

				// Verify payment commitment (Phase 1: always passes)
				let verified = T::ProofVerifier::verify_payment_commitment(
					round.payment_commitment_hash,
					proof_hash,
					proof_hash,
				);
				ensure!(verified, Error::<T>::RoundStepsOutOfOrder);

				round.proof_hash = Some(proof_hash);
				round.status = RoundStatus::PaymentProofSubmitted;
				round.last_actor = Some(who);
				Ok(())
			})?;

			Self::deposit_event(Event::PaymentProofSubmitted { session_id, round_index });
			Ok(())
		}

		// ── Phase 3: DO submits data proof ───────────────────────────────────

		#[pallet::call_index(4)]
		#[pallet::weight(T::WeightInfo::submit_data_proof())]
		pub fn submit_data_proof(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			proof_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.data_owner == who, Error::<T>::NotDataOwner);
			ensure!(session.status == SessionStatus::InDelivery, Error::<T>::InvalidSessionStatus);

			Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
				let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
				ensure!(
					round.status == RoundStatus::PaymentProofSubmitted,
					Error::<T>::RoundStepsOutOfOrder,
				);

				let verified = T::ProofVerifier::verify_data_proof(proof_hash);
				ensure!(verified, Error::<T>::RoundStepsOutOfOrder);

				round.proof_hash = Some(proof_hash);
				round.status = RoundStatus::DataProofSubmitted;
				round.last_actor = Some(who);
				Ok(())
			})?;

			Self::deposit_event(Event::DataProofSubmitted { session_id, round_index });
			Ok(())
		}

		// ── Phase 3: DR signs proof ──────────────────────────────────────────

		#[pallet::call_index(5)]
		#[pallet::weight(T::WeightInfo::submit_proof_signature())]
		pub fn submit_proof_signature(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			signature_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.requester == who, Error::<T>::NotRequester);
			ensure!(session.status == SessionStatus::InDelivery, Error::<T>::InvalidSessionStatus);

			Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
				let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
				ensure!(
					round.status == RoundStatus::DataProofVerified,
					Error::<T>::RoundStepsOutOfOrder,
				);

				let proof_hash = round.proof_hash.unwrap_or_default();
				let verified = T::ProofVerifier::verify_signature(proof_hash, signature_hash);
				ensure!(verified, Error::<T>::RoundStepsOutOfOrder);

				round.proof_signature_hash = Some(signature_hash);
				round.status = RoundStatus::ProofSigned;
				round.last_actor = Some(who);
				Ok(())
			})?;

			Self::deposit_event(Event::ProofSignatureSubmitted { session_id, round_index });
			Ok(())
		}

		// ── Phase 3: DO delivers data hash ───────────────────────────────────

		#[pallet::call_index(6)]
		#[pallet::weight(T::WeightInfo::submit_data_delivery_hash())]
		pub fn submit_data_delivery_hash(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			data_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.data_owner == who, Error::<T>::NotDataOwner);
			ensure!(session.status == SessionStatus::InDelivery, Error::<T>::InvalidSessionStatus);

			Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
				let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
				ensure!(
					round.status == RoundStatus::ProofSigned,
					Error::<T>::RoundStepsOutOfOrder,
				);

				round.delivered_data_hash = Some(data_hash);
				round.status = RoundStatus::DataDelivered;
				round.last_actor = Some(who);
				Ok(())
			})?;

			Self::deposit_event(Event::DataDelivered { session_id, round_index });
			Ok(())
		}

		// ── Phase 3: DR submits payment preimage ─────────────────────────────

		#[pallet::call_index(7)]
		#[pallet::weight(T::WeightInfo::submit_payment_preimage())]
		pub fn submit_payment_preimage(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			preimage_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.requester == who, Error::<T>::NotRequester);
			ensure!(session.status == SessionStatus::InDelivery, Error::<T>::InvalidSessionStatus);

			Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
				let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
				ensure!(
					round.status == RoundStatus::DataDelivered,
					Error::<T>::RoundStepsOutOfOrder,
				);

				round.payment_preimage_hash = Some(preimage_hash);
				round.status = RoundStatus::Completed;
				round.last_actor = Some(who);
				Ok(())
			})?;

			Sessions::<T>::mutate(session_id, |maybe_session| {
				if let Some(s) = maybe_session {
					s.completed_rounds += 1;
					s.latest_payment_preimage = Some(preimage_hash);
				}
			});

			Self::deposit_event(Event::RoundCompleted { session_id, round_index });
			Ok(())
		}

		// ── Phase 4: DO claims settlement ────────────────────────────────────

		#[pallet::call_index(8)]
		#[pallet::weight(T::WeightInfo::claim_settlement())]
		pub fn claim_settlement(
			origin: OriginFor<T>,
			session_id: SessionId,
			latest_preimage_hash: T::Hash,
			remaining_rounds: u32,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			Sessions::<T>::try_mutate(session_id, |maybe_session| -> DispatchResult {
				let session = maybe_session.as_mut().ok_or(Error::<T>::SessionNotFound)?;
				ensure!(session.data_owner == who, Error::<T>::NotDataOwner);
				ensure!(
					session.status == SessionStatus::InDelivery,
					Error::<T>::InvalidSessionStatus,
				);
				ensure!(
					remaining_rounds < session.max_rounds,
					Error::<T>::RoundStepsOutOfOrder,
				);

				let claimed_paid_rounds = session
					.max_rounds
					.checked_sub(remaining_rounds)
					.ok_or(Error::<T>::RoundStepsOutOfOrder)?;
				ensure!(
					claimed_paid_rounds <= session.completed_rounds,
					Error::<T>::SettlementRoundsExceedCompleted,
				);

				session.latest_payment_preimage = Some(latest_preimage_hash);
				session.status = SessionStatus::SettlementClaimed;
				Ok(())
			})?;
			Self::deposit_event(Event::SettlementClaimed {
				session_id,
				actor: who,
				remaining_rounds,
			});
			Ok(())
		}

		// ── Dispute: invalid proof ───────────────────────────────────────────

		#[pallet::call_index(9)]
		#[pallet::weight(T::WeightInfo::dispute_invalid_proof())]
		pub fn dispute_invalid_proof(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			_proof_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.requester == who, Error::<T>::NotRequester);
			ensure!(
				session.status == SessionStatus::InDelivery,
				Error::<T>::InvalidSessionStatus,
			);

			// Mark the round as disputed and session as punished
			Rounds::<T>::mutate(session_id, round_index, |maybe_round| {
				if let Some(round) = maybe_round {
					round.status = RoundStatus::Disputed;
					round.last_actor = Some(who.clone());
				}
			});

			Sessions::<T>::mutate(session_id, |maybe_session| {
				if let Some(s) = maybe_session {
					s.status = SessionStatus::Punished;
				}
			});

			Self::deposit_event(Event::SessionPunished { session_id });
			Ok(())
		}

		// ── Dispute: invalid plaintext hash ──────────────────────────────────

		#[pallet::call_index(10)]
		#[pallet::weight(T::WeightInfo::dispute_invalid_plaintext())]
		pub fn dispute_invalid_plaintext(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			data_hash: T::Hash,
			expected_hash: T::Hash,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.requester == who, Error::<T>::NotRequester);
			ensure!(
				session.status == SessionStatus::InDelivery,
				Error::<T>::InvalidSessionStatus,
			);

			// Verify the plaintext hash mismatch
			let verified = T::ProofVerifier::verify_plaintext_hash(data_hash, expected_hash);
			ensure!(!verified, Error::<T>::RoundStepsOutOfOrder); // must NOT match

			Rounds::<T>::mutate(session_id, round_index, |maybe_round| {
				if let Some(round) = maybe_round {
					round.status = RoundStatus::Disputed;
					round.last_actor = Some(who.clone());
				}
			});

			Sessions::<T>::mutate(session_id, |maybe_session| {
				if let Some(s) = maybe_session {
					s.status = SessionStatus::Punished;
				}
			});

			Self::deposit_event(Event::SessionPunished { session_id });
			Ok(())
		}

		// ── Phase 4: DO claims last payment (DR refuses) ────────────────────

		#[pallet::call_index(11)]
		#[pallet::weight(T::WeightInfo::claim_last_payment())]
		pub fn claim_last_payment(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			let session = Sessions::<T>::get(session_id).ok_or(Error::<T>::SessionNotFound)?;
			ensure!(session.data_owner == who, Error::<T>::NotDataOwner);
			ensure!(
				session.status == SessionStatus::InDelivery,
				Error::<T>::InvalidSessionStatus,
			);

			// Verify round was at least ProofSigned (data delivered)
			Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
				let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
				ensure!(
					round.status == RoundStatus::DataDelivered,
					Error::<T>::RoundStepsOutOfOrder,
				);
				Ok(())
			})?;

			Sessions::<T>::mutate(session_id, |maybe_session| {
				if let Some(s) = maybe_session {
					s.status = SessionStatus::SettlementClaimed;
				}
			});

			Self::deposit_event(Event::LastPaymentClaimed {
				session_id,
				actor: who,
				round_index,
			});
			Ok(())
		}

		// ── Verifier attestation (Stage 2) ──────────────────────────────────

		#[pallet::call_index(12)]
		#[pallet::weight(T::WeightInfo::attest_data_proof())]
		pub fn attest_data_proof(
			origin: OriginFor<T>,
			session_id: SessionId,
			round_index: RoundIndex,
			proof_hash: T::Hash,
			accepted: bool,
		) -> DispatchResult {
			let who = ensure_signed(origin)?;
			ensure!(who == T::VerifierAuthority::get(), Error::<T>::NotVerifier);

			Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
				let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
				ensure!(
					round.status == RoundStatus::DataProofSubmitted,
					Error::<T>::RoundStepsOutOfOrder,
				);
				ensure!(round.proof_hash == Some(proof_hash), Error::<T>::InvalidProofAttestation);
				round.status = if accepted {
					RoundStatus::DataProofVerified
				} else {
					RoundStatus::DataProofRejected
				};
				round.last_actor = Some(who);
				Ok(())
			})?;

			Self::deposit_event(Event::DataProofAttested { session_id, round_index, accepted });
			Ok(())
		}
	}
}
