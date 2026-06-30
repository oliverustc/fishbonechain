// Platform Business Model — Stage 15 type draft.
// Dependency-free TypeScript interfaces for the 9 core platform objects.
// No import/require statements. Backend-neutral. Used for schema review,
// documentation, and future implementation.

// ── Type Aliases ──

type HexHash = string;
type ChainKey = string;
type PlatformId = string;
type IsoTimestamp = string;
type ChainId = string;

// ── Enums & Literal Unions ──

type UserRole = "data_owner" | "data_requester" | "verifier" | "admin";

type SceneKind = "DataTrade" | "Crowdsource" | "CrossDomainFlow" | "VerifiableTraining";

type DatasetStatus = "draft" | "published" | "retired";

type DataAssetStatus = "active" | "suspended" | "retired";

type BusinessModule = "data_trade" | "data_collection" | "cross_domain_flow" | "verifiable_training";

type TaskStatus = "pending" | "active" | "completed" | "failed" | "disputed" | "cancelled";

type WorkflowRunStatus = "running" | "completed" | "failed" | "disputed" | "timeout";

type EvidenceCategory = "dry_run" | "negative" | "live_chain" | "postcheck";

type EvidenceStatus = "passed" | "failed" | "skipped";

type JobType = "proof_generation" | "data_preprocessing" | "anonymization" | "verification" | "training";

type JobStatus = "queued" | "running" | "completed" | "failed";

type SettlementMode = "MainEscrow" | "FmcAssisted" | "Hybrid" | "None";

// ── Nested Types ──

interface FieldSpec {
  field_name: string;
  field_type: "uint64" | "bytes" | "string";
  has_salt: boolean;
}

interface ImtConfig {
  depth: number;
  schema_version: string;
}

interface ArtifactRef {
  path: string;
  digest: HexHash | null;
  artifact_type: string;
}

interface EvidenceChainEventRef {
  pallet: string;
  variant: string;
  block_number: number;
  event_index: number;
}

interface EvidenceConstraint {
  round_index: number;
  field_name: string;
  proof_digest: HexHash;
  business_input_hash: HexHash;
  vk_hash: HexHash;
  public_input_hash: HexHash;
  on_chain_bound: boolean;
}

interface SettlementRecord {
  completed_rounds: number;
  remaining_rounds: number;
}

interface ScenarioOutcome {
  type: string;
  events: string[];
  description: string | null;
}

// ── Core Objects ──

interface User {
  user_id: PlatformId;
  display_name: string;
  role: UserRole;
  created_at: IsoTimestamp;
  updated_at: IsoTimestamp;
}

interface ChainAccount {
  account_id: PlatformId;
  user_id: PlatformId;
  chain_id: ChainId;
  address: ChainKey;
  scene_kind: SceneKind;
  verified_at: IsoTimestamp | null;
  created_at: IsoTimestamp;
}

interface Dataset {
  dataset_id: PlatformId;
  owner_account_id: PlatformId;
  name: string;
  description: string;
  schema_version: string;
  field_specs: FieldSpec[];
  imt_config: ImtConfig | null;
  status: DatasetStatus;
  created_at: IsoTimestamp;
  updated_at: IsoTimestamp;
}

interface DataAsset {
  asset_id: PlatformId;
  dataset_id: PlatformId;
  owner_account_id: PlatformId;
  chain_listing_id: number | null;
  price_per_round: number;
  max_rounds: number;
  deposit_hint: number;
  request_schema_hash: HexHash;
  proof_params_hash: HexHash;
  status: DataAssetStatus;
  created_at: IsoTimestamp;
  updated_at: IsoTimestamp;
}

interface BusinessTask {
  task_id: PlatformId;
  module: BusinessModule;
  initiator_account_id: PlatformId;
  counterparty_account_id: PlatformId | null;
  reference_ids: Record<string, string>;
  status: TaskStatus;
  created_at: IsoTimestamp;
  updated_at: IsoTimestamp;
}

interface WorkflowRun {
  run_id: PlatformId;
  task_id: PlatformId;
  session_id: number | null;
  escrow_id: number | null;
  round_count: number;
  status: WorkflowRunStatus;
  started_at: IsoTimestamp | null;
  completed_at: IsoTimestamp | null;
}

interface Evidence {
  evidence_id: PlatformId;
  run_id: PlatformId;
  category: EvidenceCategory;
  status: EvidenceStatus;
  scenario: string | null;
  result: string | null;
  command: string | null;
  log_path: string | null;
  artifacts: ArtifactRef[];
  chain_tx_hashes: HexHash[];
  chain_event_refs: EvidenceChainEventRef[];
  constraints: EvidenceConstraint[];
  settlement: SettlementRecord | null;
  scenario_outcome: ScenarioOutcome | null;
  error: string | null;
  created_at: IsoTimestamp;
}

interface ChainEvent {
  event_id: PlatformId;
  chain_id: ChainId;
  block_number: number;
  block_hash: HexHash;
  extrinsic_index: number | null;
  event_index: number;
  pallet: string;
  variant: string;
  fields: Record<string, unknown>;
  cursor: string | null;
  ingested_at: IsoTimestamp;
}

interface OffchainJob {
  job_id: PlatformId;
  job_type: JobType;
  status: JobStatus;
  input_refs: ArtifactRef[];
  output_refs: ArtifactRef[];
  worker_id: string | null;
  digest: HexHash | null;
  error: string | null;
  evidence_id: PlatformId | null;
  created_at: IsoTimestamp;
  started_at: IsoTimestamp | null;
  completed_at: IsoTimestamp | null;
}
