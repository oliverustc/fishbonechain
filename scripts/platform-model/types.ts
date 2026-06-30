/**
 * Platform Business Model — Stage 15 type draft.
 *
 * Dependency-free type documentation using JSDoc @typedef annotations.
 * This file passes `node --check` (no TypeScript-only syntax). It is
 * backend-neutral and used for schema review, documentation, and future
 * implementation.
 *
 * @module platform-model
 */

// ── Type Aliases ──

/**
 * @typedef {string} HexHash
 */
/**
 * @typedef {string} ChainKey
 */
/**
 * @typedef {string} PlatformId
 */
/**
 * @typedef {string} IsoTimestamp
 */
/**
 * @typedef {string} ChainId
 */

// ── Enums & Literal Unions ──

/**
 * @typedef {"data_owner"|"data_requester"|"verifier"|"admin"} UserRole
 */
/**
 * @typedef {"DataTrade"|"Crowdsource"|"CrossDomainFlow"|"VerifiableTraining"} SceneKind
 */
/**
 * @typedef {"draft"|"published"|"retired"} DatasetStatus
 */
/**
 * @typedef {"active"|"suspended"|"retired"} DataAssetStatus
 */
/**
 * @typedef {"data_trade"|"data_collection"|"cross_domain_flow"|"verifiable_training"} BusinessModule
 */
/**
 * @typedef {"pending"|"active"|"completed"|"failed"|"disputed"|"cancelled"} TaskStatus
 */
/**
 * @typedef {"running"|"completed"|"failed"|"disputed"|"timeout"} WorkflowRunStatus
 */
/**
 * @typedef {"dry_run"|"negative"|"live_chain"|"postcheck"} EvidenceCategory
 */
/**
 * @typedef {"passed"|"failed"|"skipped"} EvidenceStatus
 */
/**
 * @typedef {"proof_generation"|"data_preprocessing"|"anonymization"|"verification"|"training"} JobType
 */
/**
 * @typedef {"queued"|"running"|"completed"|"failed"} JobStatus
 */
/**
 * @typedef {"MainEscrow"|"FmcAssisted"|"Hybrid"|"None"} SettlementMode
 */

// ── Nested Types ──

/**
 * @typedef {object} FieldSpec
 * @property {string} field_name
 * @property {"uint64"|"bytes"|"string"} field_type
 * @property {boolean} has_salt
 */

/**
 * @typedef {object} ImtConfig
 * @property {number} depth
 * @property {string} schema_version
 */

/**
 * @typedef {object} ArtifactRef
 * @property {string} path
 * @property {HexHash|null} digest
 * @property {string} artifact_type
 */

/**
 * @typedef {object} EvidenceChainEventRef
 * @property {string} pallet
 * @property {string} variant
 * @property {number} block_number
 * @property {number} event_index
 */

/**
 * @typedef {object} EvidenceConstraint
 * @property {number} round_index
 * @property {string} field_name
 * @property {HexHash} proof_digest
 * @property {HexHash} business_input_hash
 * @property {HexHash} vk_hash
 * @property {HexHash} public_input_hash
 * @property {boolean} on_chain_bound
 */

/**
 * @typedef {object} SettlementRecord
 * @property {number} completed_rounds
 * @property {number} remaining_rounds
 */

/**
 * @typedef {object} ScenarioOutcome
 * @property {string} type
 * @property {string[]} events
 * @property {string|null} description
 */

// ── Core Objects ──

/**
 * @typedef {object} User
 * @property {PlatformId} user_id
 * @property {string} display_name
 * @property {UserRole} role
 * @property {IsoTimestamp} created_at
 * @property {IsoTimestamp} updated_at
 */

/**
 * @typedef {object} ChainAccount
 * @property {PlatformId} account_id
 * @property {PlatformId} user_id
 * @property {ChainId} chain_id
 * @property {ChainKey} address
 * @property {SceneKind} scene_kind
 * @property {IsoTimestamp|null} verified_at
 * @property {IsoTimestamp} created_at
 */

/**
 * @typedef {object} Dataset
 * @property {PlatformId} dataset_id
 * @property {PlatformId} owner_account_id
 * @property {string} name
 * @property {string} description
 * @property {string} schema_version
 * @property {FieldSpec[]} field_specs
 * @property {ImtConfig|null} imt_config
 * @property {DatasetStatus} status
 * @property {IsoTimestamp} created_at
 * @property {IsoTimestamp} updated_at
 */

/**
 * @typedef {object} DataAsset
 * @property {PlatformId} asset_id
 * @property {PlatformId} dataset_id
 * @property {PlatformId} owner_account_id
 * @property {number|null} chain_listing_id
 * @property {number} price_per_round
 * @property {number} max_rounds
 * @property {number} deposit_hint
 * @property {HexHash} request_schema_hash
 * @property {HexHash} proof_params_hash
 * @property {DataAssetStatus} status
 * @property {IsoTimestamp} created_at
 * @property {IsoTimestamp} updated_at
 */

/**
 * @typedef {object} BusinessTask
 * @property {PlatformId} task_id
 * @property {BusinessModule} module
 * @property {PlatformId} initiator_account_id
 * @property {PlatformId|null} counterparty_account_id
 * @property {Record<string,string>} reference_ids
 * @property {TaskStatus} status
 * @property {IsoTimestamp} created_at
 * @property {IsoTimestamp} updated_at
 */

/**
 * @typedef {object} WorkflowRun
 * @property {PlatformId} run_id
 * @property {PlatformId} task_id
 * @property {number|null} session_id
 * @property {number|null} escrow_id
 * @property {number} round_count
 * @property {WorkflowRunStatus} status
 * @property {IsoTimestamp|null} started_at
 * @property {IsoTimestamp|null} completed_at
 */

/**
 * @typedef {object} Evidence
 * @property {PlatformId} evidence_id
 * @property {PlatformId} run_id
 * @property {EvidenceCategory} category
 * @property {EvidenceStatus} status
 * @property {string|null} scenario
 * @property {string|null} result
 * @property {string|null} command
 * @property {string|null} log_path
 * @property {ArtifactRef[]} artifacts
 * @property {HexHash[]} chain_tx_hashes
 * @property {EvidenceChainEventRef[]} chain_event_refs
 * @property {EvidenceConstraint[]} constraints
 * @property {SettlementRecord|null} settlement
 * @property {ScenarioOutcome|null} scenario_outcome
 * @property {string|null} error
 * @property {IsoTimestamp} created_at
 */

/**
 * @typedef {object} ChainEvent
 * @property {PlatformId} event_id
 * @property {ChainId} chain_id
 * @property {number} block_number
 * @property {HexHash} block_hash
 * @property {number|null} extrinsic_index
 * @property {number} event_index
 * @property {string} pallet
 * @property {string} variant
 * @property {Record<string,*>} fields
 * @property {string|null} cursor
 * @property {IsoTimestamp} ingested_at
 */

/**
 * @typedef {object} OffchainJob
 * @property {PlatformId} job_id
 * @property {JobType} job_type
 * @property {JobStatus} status
 * @property {ArtifactRef[]} input_refs
 * @property {ArtifactRef[]} output_refs
 * @property {string|null} worker_id
 * @property {HexHash|null} digest
 * @property {string|null} error
 * @property {PlatformId|null} evidence_id
 * @property {IsoTimestamp} created_at
 * @property {IsoTimestamp|null} started_at
 * @property {IsoTimestamp|null} completed_at
 */
