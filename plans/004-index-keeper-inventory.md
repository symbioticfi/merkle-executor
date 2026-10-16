# Plan 004: Load durable jobs once per keeper iteration and update reservations safely

> Executor: read this entire file before changing anything. Follow the ordered
> steps and their verification gates. STOP and report when a stop condition
> occurs. Update only this plan's row in plans/README.md after verification,
> unless a reviewer maintains the index.
>
> Drift check, from the executor's checkout root:
>
> ~~~sh
> git diff --stat 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e..HEAD -- sdk/keeper/keeper.go sdk/keeper/inventory.go sdk/keeper/inventory_test.go sdk/keeper/inventory_benchmark_test.go sdk/README.md
> ~~~
>
> Compare the excerpts below with current source before proceeding. Resolve
> changes from completed dependencies in this plan first; unexplained drift
> requires replanning.

## Status

- Priority: P2
- Effort: M (about a day including tests)
- Risk: MED
- Depends on: none
- Category: perf
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

- sdk/keeper/keeper.go
- sdk/keeper/inventory.go
- sdk/keeper/inventory_test.go
- sdk/keeper/inventory_benchmark_test.go
- sdk/README.md

No sibling repository, live credentials, operator configuration, persistent
production state, generated ABI/bindings, unrelated provider or fee policy changes
are authorized. Read reference files outside this list as necessary.

## Git workflow

Use an isolated worktree and a codex/improve-004-index-keeper-inventory branch; do not
implement in the primary main checkout. Preserve the user's existing stash
without applying, dropping or rewriting it. Commit logical verified changes in
the executor worktree using conventional titles such as fix: or perf:. Never
merge, push, publish an SDK revision, create issues/PRs or deploy without the
operator's authorization. If a reviewer dispatches this plan, follow their
index-maintenance instructions.

## Why this matters

CheckRecovery loads every historical job, then accountBusy repeats those full
JSON loads for each strategy eligible for new preparation. When 256 settled
strategies are past their interval but idle, one Tick can read/decode 65,792 full
jobs plus 257 inventories. Historical names are retained indefinitely. Build a
verified iteration-local view so account locks and ordering do not require
quadratic file/JSON work. This is a control-flow operation count, not a measured
latency claim.

## Current state

sdk/keeper/keeper.go:129:
~~~go
func (k *Keeper) accountBusy(s Strategy) (bool, error) {
    names, err := k.inventory()
    if err != nil {
        return false, err
    }
    for _, name := range names {
        if name == s.Config.Name {
            continue
        }
        var job Job
        found, err := k.Store.Load(key(name), &job)
        // active prepared/authorized jobs reserve the same account
~~~
CheckRecovery at :61 loads all jobs independently. Tick at :173 calls it, may
reload jobs for occupied-sender ordering, then tickStrategy at :216 loads its own
job. After the interval expires, a settled job falls through to accountBusy and
Prepare. ErrNotReady leaves the old settled job in storage, repeating this scan
next time. inventory at :107 preserves historical names for recovery safety.

sdk/state/store.go:67 reads complete JSON under a mutex; Job includes Plan,
provider data and receipt logs. Do not change that store format or fsync behavior.

The persistence order in keeper.go is the exemplar:
~~~go
if err := k.Store.Save(key(s.Config.Name), job); err != nil {
    return err
}
if err := k.Sender.Acknowledge(ctx, job.ID); err != nil {
    return err
}
~~~
docs/DESIGN.md requires durable progress before acknowledgement and rejects
removed/changed active workflows. Keep CheckRecovery as a public fresh-check API.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| Inventory tests | cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test -race ./keeper -run 'TestInventory' -count=1 | linear loads and reservation transitions pass |
| Keeper safety | cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test -race ./keeper -count=1 | all recovery/composition/global-error cases pass |
| Benchmarks | cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test ./keeper -run '^$' -bench '^BenchmarkKeeperIdleInventory$' -benchmem -count=3 | sizes and allocation results printed |
| SDK full gates | make -C sdk check && make -C sdk integration | exit 0 |
| Root baseline | make check && make integration | exit 0; host still uses its published SDK pin |
| Scope | git diff --check && git diff --name-only | only allowed paths |

Each table command starts from checkout root, so the cd commands do not chain
between rows.

## Steps

### Step 1: Add safety characterization before changing iteration state

In new inventory_test.go wrap recoveryHarness's Store with load counters and
fault injection; reuse its real temporary state.Store and recoverySender.
Characterize active-account blocking, removed/changed active workflow rejection,
legacy terminal sender ownership, case-insensitive account identity, sender-first
ordering, prepare refresh and save-before-acknowledgement. These tests must pass
before the refactor.

Add TestInventoryLinearLoads with historical H=1,16,256 and a smaller current S,
settled jobs past their interval, and a waitingPlanner returning ErrNotReady.
Set up valid jobs; count Load calls after setup. Define a generous deterministic
linear bound, at most 2*H + 2*S + 16 loads, independent of payload size. This test
should fail before the refactor. Add BenchmarkKeeperIdleInventory for H=16 and256
with representative nonsecret fixture provider payloads, retaining identical work
and recording before results.

Verify: keeper safety passes; only the new linear bound fails. The benchmark
command completes and prints load/alloc/time observations, without a timing gate.

### Step 2: Introduce one verified ephemeral inventory per Tick

In a new private inventory.go, collect names, found/missing jobs and an active
account-owner index. Normalize account keys as common.Address and distinguish
strategy/job identities. Build this view once at Tick entry by loading and
validating the complete persisted inventory; never skip corrupt/removed records
to get faster execution. Public CheckRecovery still performs a fresh complete
check whenever called independently, reusing private validation helpers.

Use this view for sender-first ordering, own-job lookup and account admission.
Keep it local to a Tick invocation; do not introduce cross-tick caching, a durable
secondary index, public API changes, pruning or background goroutines. Do not
change stored phases, Config fingerprints or inventory keys.

Verify: the inventory test and keeper safety commands pass. Inspect load counts
for all H/S combinations and verify they meet the stated bound.

### Step 3: Make every durable transition update or invalidate the view

After a successful job Save/Delete, update the view and account reservations
before another strategy is considered. A completed request must release its
account only at the same phase boundary as before; a new prepared/authorized job
must immediately reserve it. Consult current Sender.PendingID for nonce ownership;
never treat a freed account as permission to bypass an occupied signer.

Job copies contain maps and slices. Do not let an in-memory mutation become an
authoritative reservation before persistence succeeds. On any uncertain Save or
Delete failure, invalidate/discard the view and abort further admission for the
iteration. The next Tick must rebuild from disk because Save may fail after
rename. Preserve the original error and sanitized global observations without
duplicating payload-bearing logs.

Add TestInventoryWriteFailureInvalidatesIteration and
TestInventoryReservationTracksDurableTransitions. Inject errors both before
write and after a real write to simulate uncertain persistence. Assert no later
strategy sends during that iteration and a subsequent Tick reloads actual disk
state. Add TestInventoryFreshBetweenTicks to prove external completed transitions
cannot remain hidden by cache reuse.

Verify: all inventory and keeper safety commands pass with -race. Compare
benchmark results with step 1; operation count is the release gate, not a fragile
wall-clock speed ratio.

### Step 4: Validate the local SDK and record the consumer delivery boundary

Run the SDK full gates. Run host tests against the changed local SDK in a
temporary, uncommitted workspace. From checkout root:
~~~sh
task_checkout="$PWD"
task_workspace="$(mktemp -d)"
(cd "$task_workspace" && GOTOOLCHAIN=go1.27.1 go work init "$task_checkout" "$task_checkout/sdk")
GOWORK="$task_workspace/go.work" make check
GOWORK="$task_workspace/go.work" make integration
~~~
Remove only that owned temporary workspace afterward. Do not create/overwrite an
existing operator workspace. sdk/Makefile intentionally sets GOWORK=off for its
independent gates.

Update sdk/README.md to explain the per-iteration scope and uncertain-write
invalidation. The daemon's committed go.mod still pins a published SDK revision.
These workspace checks prove local compatibility, not delivery. Record source
completion separately from the release gate: normal authorized SDK publication
and an immutable host pin update are required before this reaches the daemon.
The index describes the combined delivery gate with plan 005.

Verify: the temporary-workspace and root baseline gates exit 0; no go.work,
production replace, go.mod or go.sum change is committed by this plan.

## Test plan and done criteria

- [x] Safety characterization passes before and after the refactor.
- [x] Linear load bounds pass for configured and historical strategy counts.
- [x] Save/Delete uncertainty prevents any further iteration admission.
- [x] Same-iteration locks track durable transitions; no cross-tick cache remains.
- [x] SDK and local-workspace consumer check/integration gates pass.
- [x] Source completion is recorded with immutable-consumer delivery still explicit.
- [x] Scope, whitespace and index checks pass.

## Specific stop conditions

Stop if safe invalidation requires durable schema migration, store-wide snapshots,
or changes to the exported Store/Sender contracts. Do not prune historical names,
acknowledge ambiguous work, or swallow state errors. If host publication cannot be
verified, report delivery pending rather than claiming the daemon uses the fix.

## Maintenance notes

New job transitions must update this view only after durable success. Count-based
tests should protect asymptotic behavior without hardcoding an exact number of
internal helper calls. Future concurrency requires a new design; this view assumes
the existing serialized Tick/job ownership contract.

## General stop conditions

- Source excerpts no longer describe the consuming path, other than reconciled
  dependency changes.
- A step fails twice after a reasonable focused repair.
- The change requires an out-of-scope path, a new dependency, a wire/schema
  migration, a weaker authority check or production credentials.
- The required toolchain, private module access or local Anvil is unavailable.
  Record the exact failed gate; do not replace it with live signing or chain I/O.

## Completion record

Implemented in 404db80. Tick loads one complete validated ephemeral inventory;
durable transitions update reservations, uncertain writes invalidate and abort,
and independent CheckRecovery/next Tick reload storage. New safety cases passed
before/after; the linear bound failed before and passed after the refactor.
H256/S256 reads fell from 66049 to 257. Three local benchmark samples changed
2.46-6.26 s/op and about 1.098 GB/op to 14.28-14.58 ms/op and 10.93 MB/op;
these timing observations are not release thresholds. Full keeper race tests,
vet, SDK gates and independent cross-review pass.
Changed keeper.go, new inventory.go/tests/benchmark and sdk/README.md.
Combined baseline/workspace gates are in EXECUTION.md. Source is complete;
published SDK adoption remains the separate delivery gate in README.md.
