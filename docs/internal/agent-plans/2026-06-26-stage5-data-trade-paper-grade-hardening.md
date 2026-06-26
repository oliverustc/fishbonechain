# Stage 5 Data Trade Paper-Grade Hardening Plan

> **Execution owner:** Codex. This stage is intentionally assigned to Codex rather than CodeWhale because it requires adaptive judgment across paper claims, VM evidence, proof security boundaries, and next-step prioritization.

## Goal

Turn the current data-trade prototype into a paper-grade implementation package: reproducible evidence, a precise paper-alignment gap matrix, and a defensible decision about the next technical hardening target.

This stage does **not** try to implement every remaining CDT feature. It first makes the current implementation auditable and paper-ready, then chooses the next hard technical step with clear tradeoffs.

## Current Baseline

Completed stages:

- Stage 1: VM E2E regression runner exists for `main + child6`.
- Stage 2: `BusinessRangeProof` implements circuit-level range business witness with MiMC masked-value commitment.
- Stage 3: `child6-data-trade` and `child7-business-trade` profiles exist; E2E scripts are profile-aware.
- Stage 4: security model and paper alignment doc exists at `docs/architecture/data-trade-security-model.md`.

Current critical limitations:

- No on-chain Groth16 verifier. Runtime uses `AlwaysPassVerifier` + verifier attestation.
- Verifier authority is single dev key `//Charlie`.
- Bridge/session-escrow binding is checked off-chain, not by trustless cross-chain proof.
- Full IMT membership is not implemented or coupled to the business witness.
- Only `range` constraint is implemented; subset/substr/multi-field constraints are future work.
- MainEscrow is implemented; `FmcAssisted` and `Hybrid` are reserved.

## Scope

### In Scope

- Run or refresh local validation for data-trade code paths.
- Run or attempt VM regression for `main + child6` and record exact outcome.
- Run or attempt `child7-business-trade` profile smoke and record exact outcome.
- Create a formal evidence document for reproducible data-trade status.
- Create a formal paper-alignment gap matrix.
- Decide and document the next technical hardening target.
- Update `docs/README.md` and `docs/implementation/data-trade-implementation.md` if new formal docs are added.

### Out of Scope

- Implementing on-chain Groth16 verification.
- Implementing trustless bridge / CCMC proof settlement.
- Implementing full IMT membership.
- Implementing subset/substr constraints.
- Replacing `VerifierAuthority = Charlie`.
- Changing proof digest fields or settlement semantics.
- Changing VM topology, chain IDs, keys, or deployment config unless needed only to rerun existing regression.

If any out-of-scope item becomes necessary, stop and split a new plan.

## Deliverables

1. `docs/implementation/data-trade-evidence.md`
   - Records current reproducible evidence: commands, environment, commit hashes, local tests, VM E2E outcomes, artifacts, limitations.

2. `docs/implementation/data-trade-paper-gap-matrix.md`
   - Maps CDT paper requirements to current implementation status:
     - implemented
     - prototype-supported
     - partially supported
     - not implemented
     - future work

3. Updated `docs/implementation/data-trade-implementation.md`
   - Links to the evidence and gap matrix.
   - Keeps security limitations explicit.

4. Updated `docs/README.md`
   - Adds both new formal docs.

5. Updated execution record in this plan.

6. Optional decision memo section in this plan:
   - Recommends Stage 6 target.

## Stage 5A: Reproducible Evidence

### Task 1: Establish Local Baseline

- [ ] Step 1: Confirm branch and commit.

Run:

```bash
git status --short --branch
git rev-parse HEAD
```

- [ ] Step 2: Run Rust pallet tests.

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
```

- [ ] Step 3: Run Go ZK tests.

Run:

```bash
go -C tools/data-trade-zk test ./...
```

- [ ] Step 4: Run JS syntax checks.

Run:

```bash
node --check scripts/data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/bridges/data_trade.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_attestation.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/data_trade_events.js
node --check scripts/lib/data_trade_binding.js
node --check scripts/lib/hash_chain.js
node --check scripts/lib/data_trade_sample.js
node --check scripts/lib/trade_profile.js
node --check scripts/lib/vm_regression_summary.js
node --check scripts/lib/wait_for_ws_chain.js
bash -n scripts/run_data_trade_vm_regression.sh
```

- [ ] Step 5: Record exact pass/fail output summary in this plan.

### Task 2: Run or Attempt VM Regression

- [ ] Step 1: Check whether required VM endpoints are reachable before destructive redeploy.

Suggested read-only checks:

```bash
node scripts/lib/wait_for_ws_chain.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --min-blocks 1 --timeout-ms 30000
node scripts/lib/wait_for_ws_chain.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9951 --min-blocks 1 --timeout-ms 30000
```

- [ ] Step 2: If the owner confirms the VM can be clean-reset, run the main regression.

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD_WS=ws://10.2.2.11:9950 scripts/run_data_trade_vm_regression.sh
```

Important:

- This can clean redeploy `main + child6`.
- Record start/end time, commit hash, and summary path.
- If the VM is unavailable or should not be reset, do not fake success. Record the blocker and preserve local test evidence.

- [ ] Step 3: Attempt child7 profile smoke only if child7 VM deployment is expected to be alive or safe to redeploy.

Run:

```bash
node scripts/zk_real_data_trade_flow.js --profile child7-business-trade
```

Record whether this was run, skipped, or blocked.

### Task 3: Write Evidence Document

- [ ] Step 1: Create `docs/implementation/data-trade-evidence.md`.

Required sections:

- Scope
- Current commit
- Local validation
- VM validation
- E2E scenarios covered
- Artifacts and output paths
- Known limitations
- Reproduction commands

- [ ] Step 2: Be explicit about failures or skipped VM runs.

Do not use green checkmarks for skipped commands.

## Stage 5B: Paper Gap Matrix

### Task 4: Build Paper Requirement Matrix

- [ ] Step 1: Read:

```text
references/data_trade_paper/main.tex
docs/architecture/cdt.md
docs/architecture/data-trade-security-model.md
docs/implementation/data-trade-implementation.md
```

- [ ] Step 2: Create `docs/implementation/data-trade-paper-gap-matrix.md`.

Required columns:

| Paper Requirement | Current Implementation | Status | Evidence | Gap | Candidate Next Step |

Use these status values only:

- `implemented`
- `prototype-supported`
- `partially-supported`
- `not-implemented`
- `future-work`

Minimum rows:

- DO/DR roles
- DC data registry / listing
- VC session state machine
- MainEscrow funds and deposits
- Hash-chain payment / settlement
- Multi-round delivery
- DR dispute invalid proof
- DR dispute invalid plaintext
- DO claim last payment
- ZK proof artifact generation
- Circuit-level range business witness
- Root obfuscation proof
- Full IMT membership
- Custom constraint kinds: range
- Custom constraint kinds: subset/substr
- On-chain ZK verification
- Verifier authority / attestation
- Multi-verifier quorum
- Trustless cross-chain settlement
- Multiple data-trade child chains
- FmcAssisted / Hybrid settlement

- [ ] Step 3: Add a short "Paper Wording Guidance" section.

This section should say what the thesis/paper can safely claim now and what must be phrased as future work.

## Stage 5C: Next Hardening Decision

### Task 5: Compare Candidate Stage 6 Targets

Evaluate these options:

1. **Verifier quorum / threshold attestation**
   - Pros: improves trust model without full on-chain verifier.
   - Cons: still not fully trustless.

2. **Full IMT membership + business witness coupling**
   - Pros: closest to CDT technical core.
   - Cons: more circuit and artifact complexity.

3. **Trustless bridge settlement with CCMC/Merkle proof**
   - Pros: strongest FishboneChain architecture contribution.
   - Cons: broad runtime/bridge/security scope.

4. **On-chain Groth16 verifier or verifier pallet**
   - Pros: strongest proof-verification claim.
   - Cons: likely high integration/performance cost.

5. **FmcAssisted / Hybrid settlement**
   - Pros: integrates data trading with FishboneChain funding model.
   - Cons: less central to CDT proof security than IMT/verifier/bridge.

Use this scoring:

- Paper relevance
- Security improvement
- Engineering risk
- Demonstrability
- Time cost

### Task 6: Recommend Stage 6

- [ ] Step 1: Add recommendation to this plan's Execution Record.
- [ ] Step 2: If the owner agrees, create a separate Stage 6 plan.

Do not start Stage 6 implementation inside this plan.

## Acceptance Criteria

- Local tests and syntax checks are run and recorded, or failures are recorded precisely.
- VM regression is run or explicitly marked blocked/skipped with reason.
- `docs/implementation/data-trade-evidence.md` exists and is indexed.
- `docs/implementation/data-trade-paper-gap-matrix.md` exists and is indexed.
- `docs/implementation/data-trade-implementation.md` links to the evidence and gap matrix.
- No document claims on-chain Groth16 verification, trustless bridge settlement, full IMT membership, subset/substr support, or production verifier quorum unless implemented.
- Stage 6 recommendation is recorded but not implemented here.

## Validation Commands

Minimum validation before completing this stage:

```bash
test -f docs/implementation/data-trade-evidence.md
test -f docs/implementation/data-trade-paper-gap-matrix.md
rg -n "data-trade-evidence|data-trade-paper-gap-matrix" docs/README.md docs/implementation/data-trade-implementation.md
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
go -C tools/data-trade-zk test ./...
```

JS syntax checks from Task 1 should also be rerun if any scripts change.

## Execution Record

### 2026-06-26 Stage 5A/5B Evidence Pass

- Branch: `main`
- Commit at start: `afe0720a19ebd22b908f9206fd25817381cf76c4`
- Local validation:
  - `SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry`: 12 passed.
  - `SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session`: 19 passed.
  - `SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow`: 9 passed.
  - `go -C tools/data-trade-zk test ./...`: passed.
  - Data-trade JS `node --check` commands and `bash -n scripts/run_data_trade_vm_regression.sh`: passed.
- VM read-only reachability:
  - `main` at `ws://10.2.2.11:9944`: reachable and advancing.
  - `child6` at `ws://10.2.2.11:9950`: timed out.
  - `child7` at `ws://10.2.2.11:9951`: timed out.
- VM regression:
  - Current destructive clean redeploy was not run in this pass because child RPC was unavailable and clean reset should be an explicit operational decision.
  - Historical full VM regression exists at `target/data-trade-vm-regression/summary.md`, status `passed`, covering base happy, invalid proof, requester refuses payment, dev-zk attested, and real gnark ZK paths on 2026-06-16.
- Plan correction:
  - The initial suggested `wait_for_ws_chain.js --ws` command was wrong; the script requires `--main` and `--child`. Task 2 was updated to the actual command shape.
- Documents created:
  - `docs/implementation/data-trade-evidence.md`
  - `docs/implementation/data-trade-paper-gap-matrix.md`
- Formal docs updated:
  - `docs/implementation/data-trade-implementation.md`
  - `docs/README.md`
