# Plan 007: Specify safe offline recovery diagnostics for durable jobs and attempts

> Executor: read this entire file before changing anything. Follow the ordered
> steps and their verification gates. STOP and report when a stop condition
> occurs. Update only this plan's row in plans/README.md after verification,
> unless a reviewer maintains the index.
>
> Drift check, from the executor's checkout root:
>
> ~~~sh
> git diff --stat 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e..HEAD -- cmd/merkle-executor/state.go sdk/submission/manager.go sdk/state/store.go sdk/keeper/keeper.go docs/DESIGN.md plans/007-recovery-diagnostics-design.md
> ~~~
>
> Compare the excerpts below with current source before proceeding. Resolve
> changes from completed dependencies in this plan first; unexplained drift
> requires replanning.

## Status

- Priority: P3
- Effort: M (coarse design/spike estimate)
- Risk: LOW; no runtime changes in this spike
- Depends on: none; reconcile with 002 and 004 if their implementation changes evidence access
- Category: direction
- Planned at: 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e, 2026-10-16
- Source repository: /Users/andreikorokhov/symbiotic/merkle-executor
- Implementation status: DONE

## Working agreement and verification baseline

Commands run from the isolated executor checkout root unless a command explicitly
changes directory. Match Go 1.27.1, Cobra, go-errors wrapping, logr structured logs,
context-aware I/O and table-driven tests. Match the user-supplied AGENTS agreement
and the conventions researched in ../vault-solver/CLAUDE.md; that sibling file is
reference material, not an additional repository to edit.

The relevant design boundary in docs/DESIGN.md is: "External data does not grant
authority." Providers fill authorized numerical inputs or masked fields; they
cannot rewrite recipe bytes, targets, masks, proofs or caller identity. Signed or
ambiguous attempts retain ownership; timeouts never authorize replay of a recipe.
Use local HTTP/RPC fixtures and ephemeral Anvil only, with generated test keys.

The audit baseline at the planned revision passed make check for both modules,
make integration (15 top-level local Anvil tests), and the read-only lens and
Noon artifact checks. These are local/API-fixture results. Hosted CI, container
execution and live deployment were not established.

## Scope

Only these paths may be modified by this plan, plus this plan's status row:

- plans/007-recovery-diagnostics-design.md

No sibling repository, live credentials, operator configuration, persistent
production state, generated ABI/bindings, unrelated provider or fee policy changes
are authorized. Read reference files outside this list as necessary.

## Git workflow

Use an isolated worktree and a codex/improve-007-recovery-diagnostics-spike branch; do not
implement in the primary main checkout. Preserve the user's existing stash
without applying, dropping or rewriting it. Commit logical verified changes in
the executor worktree using conventional titles such as fix: or perf:. Never
merge, push, publish an SDK revision, create issues/PRs or deploy without the
operator's authorization. If a reviewer dispatches this plan, follow their
index-maintenance instructions.

## Why this matters

The current offline status command reports a job phase, one result hash and
provider outcomes. Before finalization, the owned signed hashes live in a separate
journal, so an operator may see a zero result hash even while a nonce remains
owned. A diagnostic report could explain the durable blocker and safe next step
without manually reading raw signatures/calldata. This spike specifies that report
and its trust limits; it does not reconcile, reset or release work.

## Current state

cmd/merkle-executor/state.go:38:
~~~go
type row struct {
    Identifier, ID, Phase, Transaction string
    Outcomes                           []string
}
~~~
Rows are assembled from keeper.Job only:
~~~go
r := row{Identifier: name, ID: job.ID, Phase: job.Phase, Transaction: job.Result.Hash.Hex()}
for _, p := range job.Progress {
    r.Outcomes = append(r.Outcomes, p.Outcome)
}
~~~
sdk/submission/manager.go:75 stores txmanager.active.v1 separately. Its private
record includes Version, ID, ChainID, Sender, Raw, Attempts, Confirmations, Terminal
and Execution. These raw fields may contain unpublished operational material and
must never be exposed by a report.

sdk/state/store.go:51 explicitly documents Read as an atomic file snapshot for
status only, without the writer process lock. Separate file reads are not an atomic
multi-record snapshot. Do not call state.Open or construct a sender/manager just
to inspect state. checkStateIdentity in cmd/merkle-executor/state.go checks saved
chain/caller offline.

sdk/submission/manager.go:139 PendingID can also reflect an admitted in-memory
unsigned request. Offline files cannot prove that such an in-memory flight does
not exist. Absence of a signed journal must never be labelled safe-to-retry.

The README lifecycle contract is:
"Restart reads saved hashes without rebroadcasting the recipe." It also states
that missing receipts and nonce consumption without an owned receipt retain
durable ownership. The reset command refuses unfinished work and an occupied
transaction journal; retain those guards.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| Current offline behavior | GOTOOLCHAIN=go1.27.1 go test ./cmd/merkle-executor -count=1 | existing offline/read-only/ownership checks pass |
| SDK durable boundaries | cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test ./state ./submission ./keeper -count=1 | existing journal and recovery tests pass |
| Baseline gates | make check && make integration | exit 0; no diagnostic command implemented |
| Artifact/scope | git diff --check && git status --short | only selected plan/design files change |

## Scope boundaries for this design spike

Only plans/007-recovery-diagnostics-design.md may be created/edited, plus the
index status row. Production source, persisted state and existing CLI output
remain unchanged. The design may recommend an additive inspection API, but must
not export the raw private journal struct or implement such an API now.

## Steps

### Step 1: Map the evidence and choose the CLI compatibility approach

Read status/reset, keeper.Job, submission.record/decode/terminal/PendingID and
state.Read. Write "Evidence and consistency" mapping each proposed field to its
durable source. Choose either an opt-in status --details JSON schema or a separate
diagnose command, and explain why existing consumers remain compatible. Choose
offline-only for the first version: no RPC, key loading, HTTP, provider Advance,
signer reservation, replay, acknowledgement or state write.

Compare a pure additive submission inspection function returning a safe DTO with
a host-local decoder. Prefer one authoritative SDK validator without importing
host code into the SDK or exporting signed raw bytes. Specify any new API signature
and candidate files precisely, including record-version and typed-identity
validation; a diagnostic must not normalize corrupt evidence into a valid job.

Verify: the current offline and SDK-boundary commands pass. The design must map
job state, journal identity and derived transaction hashes separately.

### Step 2: Specify a sanitized report and blocker classification

Write "Report schema and redaction" with an explicit version, deployment identity,
strategy/job IDs, durable phase/age if available, whether a journal owns this job,
attempt hashes/count, sender/chain/nonce where safely derived, required confirmation
depth, saved terminal-evidence height/hash and bounded provider outcome codes.
Fresh chain confirmation count and live engine phase are unknown offline. Hashes
come from locally validated signed attempts; never output signed bytes, calldata,
proofs, provider State/Data, execution logs, arbitrary stored errors or endpoint
credentials.

Write "Blockers and operator guidance" distinguishing signed-unknown,
authorization-evidence-saved, external-pending, confirmed-revert/operator-review,
orphan/mismatched owner, corrupt/unsupported record and mixed snapshot. A prepared
job without a journal is insufficient evidence to advise retry or deletion.
Completion flags from providers are persisted claims, not live payment proof.
Preserve legacy identifiers in inspection without rewriting them.

Define per-file consistency explicitly. A bounded reread may reduce concurrent
update ambiguity, but cannot promise a transactionally consistent snapshot. If
records disagree or change repeatedly, report unknown/mixed_snapshot; never derive
a safe-to-reset instruction. No background polling or unbounded retries.

Verify: the design has at least three complete fenced JSON examples: an uncertain
signed job with multiple attempts, an external-pending job after acknowledgement,
and a corrupt/orphan/mixed record. Synthetic hashes/IDs are acceptable; do not
copy private journal contents.

### Step 3: Define acceptance cases and a concrete implementation handoff

Write "Acceptance matrix": status while the daemon owns its file lock; no saved
deployment identity; wrong chain/caller; original plus replacement; an older owned
attempt winning; journal without owner; owner removed from config; a legacy ID;
unsupported journal version; corrupt signed bytes; changed payload/nonce/sender
across attempts; file change between reads; unsigned admitted work absent from
files; terminal job with still-occupied journal; provider metadata containing a
synthetic sensitive marker. Every case specifies expected fields/blocker code,
zero network/key access, zero writes and the existing fixture or exact new test
name. Preserve reset safety assertions.

Write "Trade-offs and next implementation" naming the chosen CLI/API change,
precise candidate files, compatibility/versioning impact, rough effort and
implementation verification commands. Include optional online canonical checks
as deferred work with separate explicit semantics; do not silently add them to
offline status.

Validate the artifact without writing files:
~~~sh
python3 - <<'CHECK'
import json, re
from pathlib import Path
s = Path('plans/007-recovery-diagnostics-design.md').read_text()
for heading in ['Evidence and consistency', 'Report schema and redaction', 'Blockers and operator guidance', 'Acceptance matrix', 'Trade-offs and next implementation']:
    assert heading in s, heading
examples = re.findall(r'~~~json\n(.*?)\n~~~', s, re.S)
assert len(examples) >= 3
for item in examples:
    json.loads(item)
print('Recovery design sections and JSON examples verified')
CHECK
~~~
Verify: the artifact check and baseline gates exit 0. Clearly record that passing
existing tests does not establish an implemented diagnostic feature.

## Done criteria

- [x] The design chooses a CLI surface and safe SDK/host inspection boundary.
- [x] Every field has a source and explicit offline freshness/consistency meaning.
- [x] At least three valid sanitized examples and all acceptance cases exist.
- [x] Missing or inconsistent evidence cannot advise replay, release or reset.
- [x] Raw signed/provider payloads and arbitrary causes are excluded by schema.
- [x] Applicable verification gates pass with no source/state changes.
- [x] The index marks the design spike DONE, not a shipped diagnostic command.

## Specific stop conditions

Stop if the report needs to construct a sender, acquire a writer lock or read a
signing secret. If nonce ownership cannot be established offline, report it as
unknown; do not fetch live state or invent a status. A diagnostic API must not
become another recovery/mutation path.

## Maintenance notes

Journal versions, provider progress and receipt confirmation witnesses can evolve.
Keep interpretation in one SDK boundary, preserve unknown states, and treat
snapshot freshness as user-visible data. Future online diagnostics need independent
authorization/evidence semantics and their own implementation plan.

## General stop conditions

- Source excerpts no longer describe the consuming path, other than reconciled
  dependency changes.
- A step fails twice after a reasonable focused repair.
- The change requires an out-of-scope path, a new dependency, a wire/schema
  migration, a weaker authority check or production credentials.
- The required toolchain, private module access or local Anvil is unavailable.
  Record the exact failed gate; do not replace it with live signing or chain I/O.

## Completion record

Delivered plans/007-recovery-diagnostics-design.md: selected offline CLI and
pure sanitized journal inspection seam, record/ownership sources, finite YAML
byte budgets, two-pass consistency limits, three valid JSON examples, safe
operator guidance, negative tests and future file scope. Separate cold review
verified corrected configured identity, explicit budget/path and orphan/removed
owner semantics. All JSON examples parse; existing offline/state/submission/keeper
and combined baseline gates pass. No runtime API/CLI/source/state change is part
of this design spike. See EXECUTION.md for final verification.
