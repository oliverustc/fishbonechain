# Web Access and User-Signed Chain Operations Plan

**Status**: Global architecture plan
**Date**: 2026-07-01

This document defines the long-term Web access model for FishboneChain. The goal
is not to depend on a specific wallet product such as MetaMask. The goal is to
provide a secure, convenient user entry point where private keys remain under
user control, while institutional backends provide indexing, identity
attestation, orchestration, and operational services.

## 1. Design Intent

FishboneChain should eventually expose the system through Web frontends and
institution-operated backends. Users should not need to run CLI scripts or talk
directly to node RPC endpoints for normal workflows.

The intended access model is:

- users access the system through a Web frontend;
- each large institution may operate its own backend and local database;
- institutional backends periodically synchronize chain state and events into
  local indexed storage;
- frontend screens primarily read from the institution backend for usability and
  performance;
- critical chain operations are signed by the user's own private key;
- the backend never stores user private keys, seed phrases, mnemonics, or
  signer material;
- users can move between computers and phones by restoring their wallet from a
  private recovery string, such as a seed phrase or mnemonic, depending on the
  wallet implementation;
- institution identity endorsement and user transaction signing are separate
  responsibilities.

This is the main security correction relative to the deprecated SciDataHub
prototype. SciDataHub's backend submitted blockchain transactions through one
server-held Fabric identity. FishboneChain must not inherit that pattern.

## 2. Core Principles

### User Keys Stay Local

User private keys must live in a wallet controlled by the user, not in the
institution backend. The frontend may request signatures, but it must not see or
persist private keys.

The user experience should resemble MetaMask's security model:

- the site requests a connection to the wallet;
- the wallet shows the account and operation to the user;
- the user approves or rejects;
- the wallet signs locally;
- the signed transaction is submitted to the chain.

For a native Substrate/FishboneChain path, the likely wallet class is a
Substrate-compatible browser/mobile wallet, such as Polkadot.js extension,
Talisman, SubWallet, or a future FishboneChain-specific wallet. MetaMask can be
considered only if FishboneChain later adds an EVM compatibility layer, a
MetaMask Snap, or a limited Ethereum-style identity-binding flow.

### Backend Is Not A Signer

Institution backends are allowed to:

- index chain state and events;
- cache derived business views;
- manage institution-local users and sessions;
- run KYC or institution identity checks;
- submit institution-owned attestations;
- coordinate IPFS/object storage;
- start off-chain jobs;
- prepare transaction intent payloads for the frontend.

Institution backends must not:

- hold user private keys;
- submit user data-trade extrinsics under a server account;
- claim local database state is chain finality;
- mutate critical trade state without a corresponding signed chain transaction;
- pass caller identity as a normal string parameter to chain logic.

### Chain Is The Recoverable Source Of Truth

The local institutional database is a cache, index, and workflow aid. Critical
facts must be recoverable from chain state, chain events, and content-addressed
artifacts.

Examples of chain-authoritative facts:

- dataset registration ownership;
- dataset permission policy;
- institution attestation status;
- trade order creation;
- escrow/reserve status;
- proof/evidence digest anchoring;
- settlement completion;
- dispute state.

Examples of backend-local or off-chain indexed facts:

- fast search indexes;
- UI-friendly denormalized views;
- institution-local workflow queues;
- cached profile metadata;
- uploaded file processing state;
- local audit logs.

## 3. Actors

### User

A researcher, data owner, data requester, verifier, or service consumer. The
user owns a wallet account and signs chain operations locally.

### Institution

A university, lab, hospital, enterprise, data center, or other large
organization that operates:

- a Web backend;
- a local database;
- a chain event indexer;
- identity verification processes;
- optional off-chain workers and storage services.

The institution may have its own chain account for endorsements and operational
actions, but that account must not replace user signatures.

### Frontend

The Web UI used by users. It reads from the institution backend, connects to a
local wallet, constructs or displays transaction intents, asks the wallet to
sign, and tracks transaction status.

### Institution Backend

The server-side application operated by an institution. It provides APIs for
query, indexing, identity workflows, transaction-intent generation, and off-chain
job orchestration.

### Wallet

The user's key manager. It stores private keys locally or in wallet-managed
secure storage. It exposes account discovery and signing capabilities to the
frontend after user approval.

### Chain

FishboneChain main chain and scene chains. Chain logic enforces identity,
attestation, permission, escrow, order, proof, and settlement rules.

## 4. High-Level Architecture

```text
User Browser / Mobile
  ├─ Web Frontend
  │   ├─ reads indexed views from Institution Backend
  │   ├─ requests wallet connection
  │   ├─ builds or displays chain transaction intent
  │   └─ asks Wallet to sign critical chain operations
  │
  └─ User Wallet
      ├─ stores private key locally
      ├─ can be restored from user-held secret phrase
      └─ signs FishboneChain extrinsics locally

Institution Backend
  ├─ local database / indexed views
  ├─ chain event indexer
  ├─ identity verification and attestation workflow
  ├─ off-chain job executor / proof generation coordinator
  └─ transaction intent API

FishboneChain
  ├─ institution registry / attestation
  ├─ data registry
  ├─ trade session / escrow / settlement
  ├─ evidence and proof digest anchoring
  └─ chain events consumed by institutional indexers
```

## 5. Identity And Attestation Model

There are two identities that should be explicitly linked but not confused:

1. **Institution Web Identity**
   - login account in an institution backend;
   - used for local KYC, role assignment, and institutional workflow;
   - not sufficient to perform chain operations.

2. **Chain Account**
   - Substrate `AccountId` controlled by the user's wallet;
   - used to sign chain extrinsics;
   - used by pallets through `ensure_signed(origin)`.

Recommended binding flow:

1. User logs into an institution backend.
2. User connects a wallet account in the frontend.
3. Backend issues a challenge message.
4. Frontend asks the wallet to sign the challenge.
5. Backend verifies the signature and records an account binding.
6. Institution performs identity verification or checks existing records.
7. Institution chain account submits an attestation extrinsic, such as:

```text
attest_user(
  user_account,
  attestation_type,
  scope,
  expires_at,
  metadata_hash
)
```

Data-trade pallets should then validate both:

- the transaction is signed by the user account;
- the user account has required institution attestations for the requested
  operation.

Institution attestation is institution-signed. Data-trade actions are
user-signed. These should remain separate.

## 6. Transaction Intent Model

The backend may help the frontend prepare chain operations, but it should return
transaction intent, not a server-signed transaction.

Example intent:

```json
{
  "chain_id": "child6-data-trade",
  "pallet": "tradeSession",
  "method": "createOrder",
  "args": {
    "dataset_id": "0x...",
    "price": "1000000000000",
    "policy_hash": "0x...",
    "evidence_digest": "0x..."
  },
  "required_signer": "user",
  "requires_attestation": true,
  "human_summary": "Create a data trade order for dataset ...",
  "risk_notes": [
    "This operation may reserve funds.",
    "Settlement conditions are enforced on-chain."
  ]
}
```

Frontend behavior:

1. render a human-readable confirmation;
2. construct the real chain extrinsic through the chain API;
3. ask the wallet to sign;
4. submit the signed extrinsic;
5. show pending/finalized status;
6. let the backend indexer catch up and update local database views.

The chain must derive caller identity from signed origin. The user account must
not be trusted if passed as an ordinary argument.

## 7. Read Path And Write Path

### Read Path

Most reads should go through the institution backend:

```text
Frontend -> Institution Backend -> Local DB / Indexed Views
```

This supports:

- fast filtering and search;
- institution-specific data visibility;
- lower RPC pressure;
- local UX enrichment;
- offline or delayed-consistency workflows.

The frontend should expose enough provenance for critical records, such as chain
event references, block numbers, extrinsic hashes, evidence digests, and indexed
timestamps.

### Write Path

Critical writes should go through user wallet signatures:

```text
Frontend -> Wallet signs -> FishboneChain RPC -> Chain events -> Backend indexer
```

The backend can assist, but it should not replace the user signature.

Examples of user-signed writes:

- bind chain account to institution user;
- register dataset ownership;
- update dataset access policy;
- create trade order;
- reserve/lock funds;
- submit proof/evidence digest if the user is responsible for it;
- accept settlement;
- raise dispute.

Examples of institution-signed writes:

- attest a user;
- revoke an attestation;
- register institution metadata;
- publish institution-level service capabilities;
- submit institution-operated indexer or audit metadata, if such a pallet is
  later designed.

## 8. Wallet Strategy

### Preferred First Path: Native Substrate Wallets

FishboneChain is currently a Substrate/FRAME project. The most direct first path
is to support Substrate-compatible wallets:

- Polkadot.js extension;
- Talisman;
- SubWallet;
- future FishboneChain-specific wallet or mobile wallet.

This path maps naturally to Substrate extrinsics, `AccountId`, sr25519/ed25519
keys, and `ensure_signed(origin)`.

### MetaMask Compatibility Is Optional, Not Foundational

MetaMask is useful as a design reference: user-controlled keys, local signing,
explicit approval, and portable seed-based recovery. It should not force the
chain architecture.

FishboneChain can consider MetaMask only under one of these future paths:

1. **EVM compatibility layer**
   Add an EVM-compatible runtime layer and expose selected operations as EVM
   calls. This is a major architecture decision and should not be done merely
   for wallet convenience.

2. **MetaMask Snap**
   Build a Snap that knows how to construct and sign FishboneChain/Substrate
   operations. This is product engineering work and should be deferred until the
   core chain workflow is stable.

3. **Limited identity binding**
   Use MetaMask only to sign an off-chain identity-binding message, while actual
   chain operations still use a Substrate wallet. This may create confusing
   two-wallet UX and should not be the default.

## 9. Backend Framework Direction

The current Stage 18/19 `scripts/platform-backend` is a dependency-free
development skeleton. It is valuable for validating platform object models,
event indexing, and off-chain job execution mechanics, but it should not be the
final institution backend.

When the core chain and data-trade workflows are stable, the production-oriented
backend should move to a functional, extensible Web framework stack.

Candidate backend direction:

- TypeScript;
- Fastify or NestJS;
- PostgreSQL for institution-local indexed state;
- a migration tool and typed schema layer, such as Prisma or Drizzle;
- Redis or a queue system when background jobs outgrow the single-shot executor;
- a chain indexer service that subscribes to finalized blocks and events;
- a transaction-intent API;
- no user private-key storage.

The exact framework can be chosen later. The architectural boundary matters more
than the specific framework.

## 10. Frontend Direction

The frontend should be a dApp-style Web application, not a traditional site that
delegates all authority to a backend.

Required capabilities:

- institution login;
- wallet connection;
- account binding challenge/signature flow;
- display institution attestations;
- browse indexed datasets and orders;
- construct transaction intents;
- request wallet signatures;
- track pending/finalized transactions;
- show local DB sync status versus chain finality;
- display evidence and digest provenance.

Candidate frontend direction:

- React/Next.js or Vue/Nuxt;
- wallet adapter abstraction for Substrate wallets;
- a clear split between read APIs and signed write operations;
- transaction confirmation views that explain pallet, method, signer, fee,
  reserve/escrow impact, and risk.

## 11. Pallet/Protocol Implications

The chain design should support this Web access model from the beginning:

- all user operations use `ensure_signed(origin)`;
- pallets store `AccountId`, not plain usernames;
- institution endorsement is represented on-chain;
- attestations can expire and be revoked;
- data-trade operations check both user signature and required attestation;
- backend-local IDs should map to chain IDs and events, not replace them;
- events should be rich enough for institutional indexers to reconstruct local
  views;
- evidence and off-chain artifacts should use hashes/digests that can be
  verified independently.

Suggested future primitives:

```text
InstitutionRegistry
  register_institution
  update_institution_metadata
  revoke_institution

UserAttestation
  attest_user
  revoke_attestation
  verify_attestation

DataRegistry
  register_dataset
  update_dataset_policy
  anchor_dataset_metadata

TradeSession
  create_order
  reserve_funds
  submit_evidence_digest
  complete_settlement
  raise_dispute
```

## 12. Security Rules

Hard rules:

- never store user private keys on backend;
- never submit user trade transactions through a backend service account;
- never trust username/buyer/seller strings as chain caller identity;
- never treat local DB state as final if chain state disagrees;
- never hide signed operation details from the user;
- never make dry-run/off-chain evidence look like finalized chain proof.

Required checks:

- account binding must use a fresh challenge and wallet signature;
- transaction intent must be scoped to a chain, pallet, method, and expected
  signer;
- backend-generated intent should be revalidated client-side where practical;
- chain pallets must enforce authorization independently of the backend;
- indexer state must record block/extrinsic/event provenance.

## 13. Phased Roadmap

### Phase A: Stabilize Core Chain Workflows

- finish data registry, trade session, escrow, proof/evidence, and settlement
  behavior;
- ensure all critical calls are user-signed;
- define event schema needed by indexers.

### Phase B: Formalize Institution And Attestation Model

- design institution registry and user attestation storage;
- define account binding challenge format;
- define attestation scope, expiry, revocation, and metadata hash semantics.

### Phase C: Build Production-Oriented Backend

- replace or complement the Stage 18/19 skeleton with a framework-based backend;
- implement chain event indexer into PostgreSQL;
- expose query APIs and transaction-intent APIs;
- integrate off-chain job executor as a worker service.

### Phase D: Build Wallet-First Frontend

- implement wallet connection and account binding;
- implement dataset/order read views from backend indexes;
- implement user-signed chain operations;
- implement transaction status and finality displays.

### Phase E: Optional Multi-Wallet And Mobile Support

- add Talisman/SubWallet/Polkadot.js compatibility testing;
- evaluate WalletConnect-style mobile flows;
- evaluate MetaMask Snap or EVM compatibility only if product goals justify the
  added complexity.

## 14. Open Questions

- Should institution attestation live on the main chain, the data-trade child
  chain, or both?
- Should a user need institution attestation for all data-trade actions, or only
  regulated/high-value datasets?
- What is the minimum wallet support matrix for the first Web demo?
- Should the backend generate transaction intents only, or should the frontend
  also be able to construct all common extrinsics independently?
- How much metadata should be on-chain versus content-addressed off-chain?
- What recovery and account-rotation workflow should be supported when a user
  loses or changes a wallet?

## 15. Summary

The intended FishboneChain Web access model is wallet-first and
institution-assisted:

```text
Institution backend = indexer + identity workflow + orchestration.
User wallet = private key custody + chain signature.
Frontend = safe transaction entry point.
Chain = final state and enforceable protocol.
```

This model preserves institutional deployment flexibility while avoiding the
centralized backend-signing flaw of the SciDataHub prototype.
