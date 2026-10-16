# Plan 002: Bound confirmation ancestry by depth while preserving canonical evidence

> Executor: read this entire file before changing anything. Follow the ordered
> steps and their verification gates. STOP and report when a stop condition
> occurs. Update only this plan's row in plans/README.md after verification,
> unless a reviewer maintains the index.
>
> Drift check, from the executor's checkout root:
>
> ~~~sh
> git diff --stat 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e..HEAD -- internal/txengine/txmanager.go internal/txengine/txmanager_test.go internal/txengine/batch_test.go internal/txengine/receipt_reader_test.go internal/txengine/receipt_failure_log_test.go internal/txengine/abandonment_test.go docs/TRANSACTIONS.md
> ~~~
>
> Compare the excerpts below with current source before proceeding. Resolve
> changes from completed dependencies in this plan first; unexplained drift
> requires replanning.

## Status

- Priority: P1
- Effort: M (about a day including tests)
- Risk: MED
- Depends on: 001 recommended first; no source dependency
- Category: perf / bug
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

- internal/txengine/txmanager.go
- internal/txengine/txmanager_test.go
- internal/txengine/batch_test.go
- internal/txengine/receipt_reader_test.go
- internal/txengine/receipt_failure_log_test.go
- internal/txengine/abandonment_test.go
- docs/TRANSACTIONS.md

No sibling repository, live credentials, operator configuration, persistent
production state, generated ABI/bindings, unrelated provider or fee policy changes
are authorized. Read reference files outside this list as necessary.

## Git workflow

Use an isolated worktree and a codex/improve-002-bound-confirmation-witness branch; do not
implement in the primary main checkout. Preserve the user's existing stash
without applying, dropping or rewriting it. Commit logical verified changes in
the executor worktree using conventional titles such as fix: or perf:. Never
merge, push, publish an SDK revision, create issues/PRs or deploy without the
operator's authorization. If a reviewer dispatches this plan, follow their
index-maintenance instructions.

## Why this matters

waitForConfirmations walks from the latest head to the receipt and then requires
that exact latest head to remain unchanged. After a confirmation-RPC outage,
ancestry requests grow with the receipt's age. If a walk takes at least one block
interval, normal chain progress can make every final equality check fail.
Canonical inclusion already transfers the lifecycle into a depth wait, so the
pending timeout does not free this lane. Bound the proof by confirmation depth,
without treating depth timeout or missing data as success.

## Current state

At internal/txengine/txmanager.go:1615:
~~~go
head := headBefore.Number.Uint64()
included := receipt.BlockNumber.Uint64()
if head >= included && head-included >= confirmations {
    if err := m.confirmReceiptAncestry(ctx, headBefore, receipt); err != nil {
        // existing reorg/error handling
    } else {
        headAfter, afterErr := m.confirmationHead(ctx)
        // current success requires headBefore.Hash() == headAfter.Hash()
    }
}
~~~
The actual loop at :1737 is:
~~~go
for current.Number.Uint64() > included {
    lookupCtx, cancel := context.WithTimeout(ctx, m.receiptReadTimeout())
    parent, err := parentHeader(lookupCtx, current.ParentHash)
    cancel()
    // exact hash and number linkage is checked
    current = parent
}
~~~
Read both complete functions before editing; the comment above summarizes a branch,
rather than replacing its validation. confirmCanonicalReceipt at :1697 also walks
from latest and needs the same bounded inclusion treatment.

sdk/submission/receipt.go:14 is the semantic reference: use receipt block + depth
as a witness, walk exact parent hashes, reread the receipt, then reread the witness
by number. It proves canonicality without requiring the chain to stop.
Do not edit SDK code under this plan.

Existing TestConfirmationsRequireStableHead and
TestConfirmationOpeningBatchKeepsFinalHeadFence encode the current latest-head
mechanism. Replace their mechanism-specific assertions with equally strong
numbered-witness reorg assertions, preserving batched and nonbatched coverage.
TestConfirmationsRejectReceiptFromDifferentFork must continue rejecting its fork.

docs/DESIGN.md requires "Only canonical confirmed receipts advance workflows."
Match confirmationOpening's go-errors wrapping and per-read timeout patterns.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| New proof regressions | GOTOOLCHAIN=go1.27.1 go test -race ./internal/txengine -run 'TestConfirmationWitness' -count=1 | advancing-head and reorg cases pass |
| Existing proof/lifetime suite | GOTOOLCHAIN=go1.27.1 go test -race ./internal/txengine ./internal/txmanager -count=1 | all pass |
| Consumer finality | GOTOOLCHAIN=go1.27.1 make -C sdk test | existing independent journal evidence tests pass |
| Full gates | make check && make integration | exit 0 |
| Scope | git diff --check && git diff --name-only | no whitespace errors or out-of-scope changes |

## Steps

### Step 1: Reproduce the moving-head failure deterministically

Add TestConfirmationWitnessOldReceiptAdvancingHead with a valid receipt at height
100, required depth 2 and a latest head thousands of blocks ahead. Script latest
to advance on every observation; numbered headers at 100 and 102 stay canonical.
Count HeaderByHash reads and ensure the test's safety deadline bounds a failure.
Run both a BatchReader backend and a legacy backend. Add
TestConfirmationWitnessDepthBoundedReads comparing gaps of 10 and 10,000 blocks:
the bound must depend on depth, not the gap.

Verify: the new proof regression command fails on the baseline rather than
waiting indefinitely. Existing fork tests remain passing before implementation.

### Step 2: Replace latest-to-receipt walks with numbered witnesses

Refactor the engine's inclusion and depth checks. For inclusion use the receipt
block's numbered canonical header (depth 0); for depth use a descendant at
receipt.BlockNumber + confirmations. Validate numbers before arithmetic, guard
overflow, validate every parent hash/number link and the final receipt block hash.
Use existing per-read budgets and batch capability for independent opening reads.
Normal latest-head advancement after capturing the witness must be accepted.

Before success, reread the receipt and require matching transaction hash, valid
status, block number and block hash. Preserve refreshed receipt/log evidence when
reinclusion changes it; the closing numbered witness must still match. Missing
witness/depth is pending; malformed headers, transport errors and reorganized
receipts never count as success. Retain the two-consecutive-null receipt rule and
read-error reset behavior. Preserve interruption/reverted outcome semantics.

Verify: run the new proof regressions. For a healthy depth-2 check, assert at most
two parent HeaderByHash reads per attempt independent of latest height. Run the
existing proof/lifetime suite; any expected call-count changes need replacement
safety assertions, not deleted tests.

### Step 3: Close the negative matrix and document behavior

Add TestConfirmationWitnessReorgRejected for an orphan witness, inconsistent
parent number/hash, altered receipt status/log identity, receipt disappearance
and head too shallow. Cover a witness reorg during the final receipt reread and
closure. Add depth-0 inclusion coverage for an old receipt so preliminary
canonical checking is bounded too. Keep the positive-depth pending-timeout
retention regression in abandonment_test.go.

Verify: run the existing proof/lifetime and SDK finality commands, then make check
and make integration. Document that finality is proven against a stable numbered
descendant, not an unchanging latest block.

## Test plan and done criteria

- [x] Advancing latest head does not prevent confirmation of a canonical witness.
- [x] Parent-read count is O(depth), including old receipts and depth-0 inclusion.
- [x] Fork, missing-receipt, malformed-header and witness-reorg cases cannot confirm.
- [x] Original and refreshed receipt evidence remains correctly attributed.
- [x] Existing batch and legacy backend paths remain covered.
- [x] All command-table gates pass and the index is updated.

## Specific stop conditions

Stop if changing the witness requires weakening exact parent linkage or accepting
a cached terminal receipt without fresh canonical evidence. Do not solve liveness
by limiting the walk and treating its truncated result as confirmation. If a
different public engine API is required, replan before expanding scope.

## Maintenance notes

The SDK has an independent confirmation implementation. Keep the same proof
invariants; avoid coupling engine outcomes to business settlement. Tests must
distinguish harmless latest-head growth from reorganization of the witness.

## General stop conditions

- Source excerpts no longer describe the consuming path, other than reconciled
  dependency changes.
- A step fails twice after a reasonable focused repair.
- The change requires an out-of-scope path, a new dependency, a wire/schema
  migration, a weaker authority check or production credentials.
- The required toolchain, private module access or local Anvil is unavailable.
  Record the exact failed gate; do not replace it with live signing or chain I/O.

## Completion record

Implemented in 36b28ee. Exact numbered witnesses replace latest-head equality
and old-receipt walks; fresh receipt/status/log evidence and the closing witness
remain mandatory. New advancing-head cases were observed RED then GREEN for
batch and legacy backends; depth-zero and depth-two RPC bounds are covered.
Full engine/txmanager race tests and independent SDK finality tests pass.
Changed the six reconciled engine/test/doc paths, including the separately
approved receipt_reader_test.go assertion update below. Independent cross-review
approved. Combined final gates are in EXECUTION.md.

## Execution scope reconciliation

2026-10-16: the full engine suite exposed TestReceiptSweepGivesEveryHashFullTimeout
counting exactly 52 reads. Fresh inclusion/depth-zero proofs correctly add three
receipt rereads. The reviewer approved only this additional test file: retain the
52 distinct sweep reads, identify the three winning-hash proof reads separately,
and preserve the full timeout assertion on every read. No production exception.
