# FishboneChain Platform Business Model

**Status**: Stage 15 design baseline. This document defines the platform-level business object model, shared across all business modules. It is NOT a backend implementation — no database schema, API server, authentication, or UI is implied. Web/backend records are orchestration and audit metadata, not chain finality.

**Date**: 2026-06-29
**Stage**: Stage 15
**Roadmap**: `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`

## 1. Model Principles

1. **User-signed chain actions only.** Every chain state change (listing, session, escrow, settlement, dispute) originates from a user-signed transaction. The platform backend never holds private keys or signs on behalf of users.

2. **Backend is orchestration and indexing, not trust replacement.** Backend records user profiles, business task metadata, evidence references, and chain event indexes. Database state is never protocol finality; chain events and evidence digests are the recoverable source of truth.

3. **Chain events are recoverable state source.** Platform state can be reconstructed from chain events alone. The event indexer is an infrastructure layer that normalizes events from multiple chains into platform records.

4. **Evidence is audit metadata, not chain finality.** Every business operation produces evidence: inputs, chain transactions, events, off-chain artifacts, digests, proofs, attestations, results, and errors. Evidence serves paper, debugging, and frontend display.

5. **Scene-specific extensions, not data-trade hard-coding.** Data trade is the first business module, not the platform default. The 9 core objects are shared abstractions; each business module extends them with module-specific fields and state machines.

## 2. Core Objects

### 2.1 User

**Purpose**: Represents a human or organizational participant on the platform. A User owns one or more chain accounts but exists independently of any single chain identity.

| Field | Type | Description |
|-------|------|-------------|
| `user_id` | `PlatformId` | Platform-unique user identifier |
| `display_name` | `string` | Human-readable name |
| `role` | `UserRole` | `data_owner`, `data_requester`, `verifier`, `admin` |
| `created_at` | `IsoTimestamp` | Registration timestamp |
| `updated_at` | `IsoTimestamp` | Last profile update timestamp |

**What must NOT be stored or trusted**:
- Private keys or mnemonics.
- The user's ability to sign transactions (must be verified on-chain).
- User role as authorization for chain actions (only chain account signatures authorize extrinsics).

### 2.2 ChainAccount

**Purpose**: Links a User to one or more blockchain accounts (public keys/addresses) on specific chains.

| Field | Type | Description |
|-------|------|-------------|
| `account_id` | `PlatformId` | Platform account record identifier |
| `user_id` | `PlatformId` | Owning User reference |
| `chain_id` | `ChainId` | Which chain this account lives on |
| `address` | `ChainKey` | SS58-encoded public key |
| `scene_kind` | `SceneKind` | `DataTrade`, `Crowdsource`, etc. (from chain profile) |
| `verified_at` | `IsoTimestamp \| null` | When chain signature verification was last confirmed |
| `created_at` | `IsoTimestamp` | Binding creation timestamp |

**What must NOT be stored or trusted**:
- Private keys.
- Cached balances without an on-chain query refresh.
- Address ownership without a recent signature challenge.

### 2.3 Dataset

**Purpose**: A collection of structured data records owned by a Data Owner. This is a metadata object describing what data exists, not the data itself.

| Field | Type | Description |
|-------|------|-------------|
| `dataset_id` | `PlatformId` | Platform-unique dataset identifier |
| `owner_account_id` | `PlatformId` | Owning ChainAccount reference |
| `name` | `string` | Human-readable name |
| `description` | `string` | Dataset description |
| `schema_version` | `string` | Schema version for record structure |
| `field_specs` | `FieldSpec[]` | Per-field name, type (`uint64`...), salt presence |
| `imt_config` | `ImtConfig \| null` | IMT configuration if structured Merkle tree is used |
| `status` | `DatasetStatus` | `draft`, `published`, `retired` |
| `created_at` | `IsoTimestamp` | Creation timestamp |
| `updated_at` | `IsoTimestamp` | Last metadata update |

**What must NOT be stored or trusted**:
- Raw data records (these belong off-chain or encrypted).
- Computed IMT roots without verifiable derivation from stored records.
- Claims about data accuracy or completeness without evidence.

### 2.4 DataAsset

**Purpose**: A Data Owner's published offering of a specific dataset to potential requesters. This is the platform abstraction over a chain listing.

| Field | Type | Description |
|-------|------|-------------|
| `asset_id` | `PlatformId` | Platform-unique data asset identifier |
| `dataset_id` | `PlatformId` | Source Dataset reference |
| `owner_account_id` | `PlatformId` | Publishing ChainAccount reference |
| `chain_listing_id` | `number \| null` | Chain-specific listing ID (e.g., `pallet-data-registry` listing) |
| `price_per_round` | `number` | Price per round in chain-native units |
| `max_rounds` | `number` | Maximum trade rounds |
| `deposit_hint` | `number` | Suggested deposit amount |
| `request_schema_hash` | `HexHash` | Hash of accepted request schema |
| `proof_params_hash` | `HexHash` | Hash of proof parameters |
| `status` | `DataAssetStatus` | `active`, `suspended`, `retired` |
| `created_at` | `IsoTimestamp` | Publication timestamp |
| `updated_at` | `IsoTimestamp` | Last status change |

**What must NOT be stored or trusted**:
- The chain listing state without on-chain query confirmation.
- Asset status as authorization for chain operations.

### 2.5 BusinessTask

**Purpose**: A business-level task that groups one or more workflow runs. Represents a user-initiated business goal (e.g., "acquire factory_sensors temperature data, multi-range").

| Field | Type | Description |
|-------|------|-------------|
| `task_id` | `PlatformId` | Platform-unique task identifier |
| `module` | `BusinessModule` | `data_trade`, `data_collection`, `cross_domain_flow`, `verifiable_training` |
| `initiator_account_id` | `PlatformId` | ChainAccount that created the task |
| `counterparty_account_id` | `PlatformId \| null` | Counterparty ChainAccount (e.g., DR in data trade) |
| `reference_ids` | `Record<string, string>` | Module-specific foreign keys (`dataset_id`, `asset_id`, `escrow_id`, etc.) |
| `status` | `TaskStatus` | `pending`, `active`, `completed`, `failed`, `disputed`, `cancelled` |
| `created_at` | `IsoTimestamp` | Task creation timestamp |
| `updated_at` | `IsoTimestamp` | Last status change |

**What must NOT be stored or trusted**:
- Task status as protocol finality (chain events are the authority).
- Settlement amounts without chain event confirmation.

### 2.6 WorkflowRun

**Purpose**: One execution of a BusinessTask, typically bound to a specific chain session or workflow instance. Multiple WorkflowRuns may exist for the same task (e.g., retries, multi-round).

| Field | Type | Description |
|-------|------|-------------|
| `run_id` | `PlatformId` | Platform-unique run identifier |
| `task_id` | `PlatformId` | Parent BusinessTask reference |
| `session_id` | `number \| null` | Chain session ID (e.g., `pallet-trade-session` session) |
| `escrow_id` | `number \| null` | Chain escrow ID (e.g., `pallet-main-escrow` escrow) |
| `round_count` | `number` | Number of rounds in this run |
| `status` | `WorkflowRunStatus` | `running`, `completed`, `failed`, `disputed`, `timeout` |
| `started_at` | `IsoTimestamp \| null` | Start timestamp |
| `completed_at` | `IsoTimestamp \| null` | Completion timestamp |

**What must NOT be stored or trusted**:
- Run status without corresponding Evidence records.
- Round completion counts without chain event verification.

### 2.7 Evidence

**Purpose**: Immutable record of what happened during a workflow run. Contains inputs, chain transactions, events, off-chain artifacts, digests, and results. Evidence is the primary paper, debugging, and audit artifact.

| Field | Type | Description |
|-------|------|-------------|
| `evidence_id` | `PlatformId` | Platform-unique evidence identifier |
| `run_id` | `PlatformId` | Parent WorkflowRun reference |
| `category` | `EvidenceCategory` | `dry_run`, `negative`, `live_chain`, `postcheck` |
| `status` | `EvidenceStatus` | `passed`, `failed`, `skipped` |
| `scenario` | `string \| null` | business scenario name (e.g., `happy`, `invalid-proof-dispute`) |
| `result` | `string \| null` | evidence-declared result |
| `command` | `string \| null` | Full command that produced this evidence |
| `log_path` | `string \| null` | Path to run log (relative to output root) |
| `artifacts` | `ArtifactRef[]` | Off-chain artifact references |
| `chain_tx_hashes` | `HexHash[]` | Chain transaction hashes |
| `chain_event_refs` | `EvidenceChainEventRef[]` | Chain event references (event type, block, index) |
| `constraints` | `EvidenceConstraint[]` | Per-constraint proof binding records |
| `settlement` | `SettlementRecord \| null` | Settlement outcome |
| `scenario_outcome` | `ScenarioOutcome \| null` | Dispute/exception result |
| `error` | `string \| null` | Error summary if any |
| `created_at` | `IsoTimestamp` | Evidence record creation timestamp |

**Per-constraint proof binding (`EvidenceConstraint`)**:

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `round_index` | `number` | Round within the session | Chain `RoundState` index |
| `field_name` | `string` | Data field name under constraint | Request + dataset schema |
| `proof_digest` | `HexHash` | ZK proof digest (computed by pallet `compute_zk_proof_digest`) | Chain `RoundState` / evidence JSON |
| `business_input_hash` | `HexHash` | Business witness input hash | Chain `RoundState` / `business_input_hash` field |
| `vk_hash` | `HexHash` | Verifying key hash | Chain `RoundState` `vk_hash` field |
| `public_input_hash` | `HexHash` | Public input hash | Chain `RoundState` `public_input_hash` field |
| `on_chain_bound` | `boolean` | Whether the proof digest was bound into accepted chain state/events (digest and metadata binding only; not on-chain Groth16 verification) | Chain events from `submit_data_proof` / `attest_data_proof` |

**What must NOT be stored or trusted**:
- Evidence records as replacements for chain state queries (they record, not enforce).
- `proof_digest` as on-chain Groth16 verification (it is a digest binding, verified off-chain via `fishbone-zk verify`).

### 2.8 ChainEvent

**Purpose**: A normalized record of a single on-chain event, indexed from a specific chain at a specific block. ChainEvents enable platform state reconstruction and cross-chain event correlation.

| Field | Type | Description |
|-------|------|-------------|
| `event_id` | `PlatformId` | Platform-unique event identifier |
| `chain_id` | `ChainId` | Source chain identifier |
| `block_number` | `number` | Block number |
| `block_hash` | `HexHash` | Block hash |
| `extrinsic_index` | `number \| null` | Extrinsic index within the block |
| `event_index` | `number` | Event index within the extrinsic |
| `pallet` | `string` | Pallet name (e.g., `tradeSession`, `mainEscrow`) |
| `variant` | `string` | Event variant (e.g., `SessionCreated`, `EscrowSettled`) |
| `fields` | `Record<string, unknown>` | Event field values (JSON-serializable) |
| `cursor` | `string \| null` | Indexer cursor for resumption |
| `ingested_at` | `IsoTimestamp` | When the event was indexed |

**What must NOT be stored or trusted**:
- Event `fields` as the only source of state (replay must be possible from chain).
- Event ordering across chains without block timestamp normalization.

### 2.9 OffchainJob

**Purpose**: An off-chain computation task (proof generation, data preprocessing, verification, model training). OffchainJobs are created by the platform backend, executed by workers, and produce artifacts referenced by Evidence records.

| Field | Type | Description |
|-------|------|-------------|
| `job_id` | `PlatformId` | Platform-unique job identifier |
| `job_type` | `JobType` | `proof_generation`, `data_preprocessing`, `anonymization`, `verification`, `training` |
| `status` | `JobStatus` | `queued`, `running`, `completed`, `failed` |
| `input_refs` | `ArtifactRef[]` | Input artifact references |
| `output_refs` | `ArtifactRef[]` | Output artifact references |
| `worker_id` | `string \| null` | Executing worker identifier |
| `digest` | `HexHash \| null` | Output artifact digest (if applicable) |
| `error` | `string \| null` | Error message if failed |
| `evidence_id` | `PlatformId \| null` | Associated Evidence record |
| `created_at` | `IsoTimestamp` | Job creation timestamp |
| `started_at` | `IsoTimestamp \| null` | Execution start timestamp |
| `completed_at` | `IsoTimestamp \| null` | Completion timestamp |

**What must NOT be stored or trusted**:
- Job completion as proof of correctness without verification of output digest.
- Worker identity as a trust assertion.

## 3. Cross-Object Relationships

```
User 1──n ChainAccount
ChainAccount 1──n DataAsset
ChainAccount 1──n Dataset
ChainAccount 1──n BusinessTask (as initiator or counterparty)
Dataset 1──n DataAsset
BusinessTask 1──n WorkflowRun
WorkflowRun 1──n Evidence
WorkflowRun 1──n ChainEvent (via session_id, escrow_id correlation)
Evidence 1──n ChainEvent (via evidence_id links)
Evidence 1──n OffchainJob (via evidence_id links)
OffchainJob 1──n ArtifactRef (input/output)
```

Key invariants:

- Every `ChainEvent` is provably traceable to a specific chain, block, and extrinsic.
- Every `Evidence` record links to at least one `WorkflowRun` and is backed by a command log and/or chain transactions.
- `DataAsset.chain_listing_id` references a `pallet-data-registry` listing; `WorkflowRun.session_id` references a `pallet-trade-session` session; `WorkflowRun.escrow_id` references a `pallet-main-escrow` escrow.
- Platform record IDs (`PlatformId`) are never chain state IDs. The mapping from platform records to chain state is via explicit reference fields.

## 4. Data Trade Mapping

Data trade (CDT) is the first module mapped onto the shared platform model. This table shows how current chain objects and evidence fields map to platform objects.

### 4.1 Chain State → Platform Objects

| Chain Object / Field | Source | Platform Object | Platform Field |
|----------------------|--------|-----------------|----------------|
| DO's chain account address | `pallet-data-registry` listing `owner` | `ChainAccount` | `address` |
| DR's chain account address | `pallet-trade-session` session creator | `ChainAccount` | `address` |
| Listing (Active/Suspended/Retired) | `pallet-data-registry` | `DataAsset` | `chain_listing_id`, `status` |
| Listing `price_per_round` | `pallet-data-registry` listing | `DataAsset` | `price_per_round` |
| Listing `max_rounds` | `pallet-data-registry` listing | `DataAsset` | `max_rounds` |
| Listing `deposit_hint` | `pallet-data-registry` listing | `DataAsset` | `deposit_hint` |
| Listing `request_schema_hash` | `pallet-data-registry` listing | `DataAsset` | `request_schema_hash` |
| Listing `proof_params_hash` | `pallet-data-registry` listing | `DataAsset` | `proof_params_hash` |
| Session ID | `pallet-trade-session` | `WorkflowRun` | `session_id` |
| Escrow ID | `pallet-main-escrow` | `WorkflowRun` | `escrow_id` |
| Trade terms (rounds, price) | `pallet-main-escrow` escrow state | `WorkflowRun` | `round_count` |
| Session status (Active/Punished/Settled) | `pallet-trade-session` | `WorkflowRun` | `status` |
| Session dispute outcome | `pallet-trade-session` `SessionPunished` | `Evidence` | `scenario_outcome` |
| Escrow settlement | `pallet-main-escrow` `EscrowSettled` | `Evidence` | `settlement` |

### 4.2 Per-Constraint Proof Binding → Platform Objects

| Chain / Evidence Field | Source | Platform Object | Platform Field |
|------------------------|--------|-----------------|----------------|
| `proof_digest` | Chain `RoundState` / `submit_data_proof` args / evidence JSON | `EvidenceConstraint` | `proof_digest` |
| `business_input_hash` | Chain `RoundState` / `business_input_hash` field | `EvidenceConstraint` | `business_input_hash` |
| `vk_hash` | Chain `RoundState` `vk_hash` field | `EvidenceConstraint` | `vk_hash` |
| `public_input_hash` | Chain `RoundState` `public_input_hash` field | `EvidenceConstraint` | `public_input_hash` |
| `proof_system` | Chain `RoundState` (e.g., `"groth16_bn254"`) | `OffchainJob` | metadata on job type |
| `constraint_kind` | Chain `RoundState` (e.g., `"range"`, `"multi_range"`) | `EvidenceConstraint` | implicit from request type |
| `ro_depth` | Chain `RoundState` | `OffchainJob` | metadata |
| `ch_proof_hash` | Chain `RoundState` | `EvidenceConstraint` | can join with `proof_digest` |
| `ro_proof_hash` | Chain `RoundState` | `EvidenceConstraint` | can join with `proof_digest` |
| `verifier_attestation_hash` | Chain `RoundState` / `attest_data_proof` args | `EvidenceChainEventRef` | event variant `DataProofAttested` |

### 4.3 Stage 14 Evidence → Platform Objects

| Stage 14 `summary.json` field | Evidence field | Platform Object | Platform Field |
|-------------------------------|---------------|-----------------|----------------|
| `summary.scenarios[]` | — | `Evidence` | one per scenario |
| `scenario.id` | `id` | `Evidence` | `evidence_id` component |
| `scenario.category` | `category` | `Evidence` | `category` |
| `scenario.status` | `status` | `Evidence` | `status` |
| `scenario.command` | `command` | `Evidence` | `command` |
| `scenario.log_path` | `log_path` | `Evidence` | `log_path` |
| `scenario.evidence_path` | — | `Evidence` | artifact reference |
| `scenario.listing_id` | `listing_id` | `DataAsset` reference | `chain_listing_id` |
| `scenario.escrow_id` | `escrow_id` | `WorkflowRun` | `escrow_id` |
| `scenario.session_id` | `session_id` | `WorkflowRun` | `session_id` |
| `scenario.settlement` | `settlement` | `Evidence` | `settlement` |
| `scenario.scenario_outcome` | `scenario_outcome` | `Evidence` | `scenario_outcome` |
| `scenario.events[]` | `events` | `EvidenceChainEventRef` | event names |
| `scenario.constraints[]` | `constraints` | `EvidenceConstraint[]` | per-constraint binding |
| `scenario.constraints[].proof_digest` | `proof_digest` | `EvidenceConstraint` | `proof_digest` |
| `scenario.constraints[].business_input_hash` | `business_input_hash` | `EvidenceConstraint` | `business_input_hash` |
| `scenario.constraints[].on_chain_bound` | `on_chain_bound` | `EvidenceConstraint` | `on_chain_bound` |

### 4.4 Current Limitations Visible in Mappings

These limitations are current implementation facts tracked in `docs/architecture/data-trade-security-model.md` (cross-referenced for participant trust assumptions) and `docs/implementation/data-trade-implementation.md`:

- **No on-chain Groth16 verification**: `proof_digest` is bound on-chain; actual proof verification is off-chain via `fishbone-zk verify`. Platform object `EvidenceConstraint.proof_digest` records the digest, not a verification claim.
- **Single verifier authority (Charlie)**: `VerifierAuthority = Charlie` is a dev key. Attestation authenticity in dev mode depends on operational trust, not on-chain consensus.
- **Off-chain bridge coordination**: `MainEscrow` settlement is submitted by bridge scripts. Session-escrow binding is checked off-chain, not via trustless cross-chain Merkle proof.
- **MainEscrow only**: `FmcAssisted` and `Hybrid` settlement modes are reserved. Platform model maps `WorkflowRun.escrow_id` as nullable to support future modes.

## 5. Placeholder Module Mappings

The following modules are target abstractions for future stages. The mappings below describe the intended object model, not implemented behavior. **These are NOT claims of current support.**

### 5.1 Data Collection

**Roadmap target**: Stage 22. Generic data collection tasks, worker submissions, quality checks, and reward settlement.

**Candidate platform object extensions**:

| Concept | Platform Object | Notes |
|---------|-----------------|-------|
| Collection task definition | `BusinessTask.module = "data_collection"` | Reuses `BusinessTask` with module-specific `reference_ids` |
| Worker submission | `WorkflowRun` | Each submission is a run bound to the task |
| Quality check result | `Evidence` | Dry-run or automated quality validation |
| Reward settlement | `ChainEvent` variant | Settlement events from chain |
| Worker identity | `ChainAccount` | Worker's chain account |

**Status**: Placeholder. No chain pallet, script, or experiment exists for data collection.

### 5.2 Cross-Domain Flow

**Roadmap target**: Stage 23. Data flow records and proofs across domains, subchains, or participant scopes.

**Candidate platform object extensions**:

| Concept | Platform Object | Notes |
|---------|-----------------|-------|
| Cross-domain request | `BusinessTask.module = "cross_domain_flow"` | Source domain, target domain, authorization |
| Authorization grant | `Evidence` | Grant record with source domain proof |
| Data transfer summary | `ChainEvent` variant | On-chain anchor or digest of transfer |
| Target domain receipt | `Evidence` | Receipt of received data digest |
| Source/target chain correlation | `ChainEvent` cross-chain references | Multiple `chain_id` values in event records |

**Status**: Placeholder. No chain pallet, script, or experiment exists for cross-domain flow.

### 5.3 Verifiable Training

**Roadmap target**: Stage 24. Training tasks where input data commitments, training execution, model digest submissions, and result attestations are recorded as platform objects.

**Candidate platform object extensions**:

| Concept | Platform Object | Notes |
|---------|-----------------|-------|
| Training task | `BusinessTask.module = "verifiable_training"` | Input commitment, model architecture descriptor |
| Training job | `OffchainJob.job_type = "training"` | GPU/CPU worker execution |
| Model digest submission | `Evidence` | Model hash, evaluation metrics |
| Training attestation | `EvidenceConstraint` or new object | Verifier-signed training correctness attestation |
| Data authorization | `Evidence` | Proof that data owner authorized use |

**Status**: Placeholder. No chain pallet, script, or experiment exists for verifiable training. Initial attestations will use the existing verifier attestation pattern; stronger verifiable training proofs are advanced research.

## 6. References

- [Platform Architecture](./platform-architecture.md)
- [Data Trade Security Model](./data-trade-security-model.md) — participant trust assumptions and attack scenarios
- [Data Trade Implementation](../implementation/data-trade-implementation.md)
- [Data Trade Validation Experiment](../experiments/data-trade-validation.md)
- [Stage 14 Evidence Index](../implementation/data-trade-stage14-evidence-index.md)
- [Long-term Roadmap](../internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md)
- [Type draft](../../scripts/platform-model/types.ts) — dependency-free JSDoc type definitions for this model
