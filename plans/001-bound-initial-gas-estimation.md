# Plan 001: Bound initial gas estimation without releasing admitted work early

> Executor: read this entire file before changing anything. Follow the ordered
> steps and their verification gates. STOP and report when a stop condition
> occurs. Update only this plan's row in plans/README.md after verification,
> unless a reviewer maintains the index.
>
> Drift check, from the executor's checkout root:
>
> ~~~sh
> git diff --stat 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e..HEAD -- internal/txengine/horizon.go internal/txengine/horizon_test.go internal/txmanager/recovery_policy_test.go docs/TRANSACTIONS.md
> ~~~
>
> Compare the excerpts below with current source before proceeding. Resolve
> changes from completed dependencies in this plan first; unexplained drift
> requires replanning.

## Status

- Priority: P1
- Effort: S (hours including tests)
- Risk: LOW
- Depends on: none
- Category: bug
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

- internal/txengine/horizon.go
- internal/txengine/horizon_test.go
- internal/txmanager/recovery_policy_test.go
- docs/TRANSACTIONS.md

No sibling repository, live credentials, operator configuration, persistent
production state, generated ABI/bindings, unrelated provider or fee policy changes
are authorized. Read reference files outside this list as necessary.

## Git workflow

Use an isolated worktree and a codex/improve-001-bound-initial-gas-estimation branch; do not
implement in the primary main checkout. Preserve the user's existing stash
without applying, dropping or rewriting it. Commit logical verified changes in
the executor worktree using conventional titles such as fix: or perf:. Never
merge, push, publish an SDK revision, create issues/PRs or deploy without the
operator's authorization. If a reviewer dispatches this plan, follow their
index-maintenance instructions.

## Why this matters

Initial next-block estimation and its latest-state fallback receive the manager's
long-lived context. A read RPC that keeps that request open indefinitely blocks
the sole admission worker before any transaction is signed. The keeper's caller
timeout does not cancel admitted work, so every later send remains blocked until
the RPC recovers or the process restarts. Give this unsigned operation a finite
budget while keeping existing ownership and cancellation semantics.

## Current state

- internal/txengine/horizon.go:451, callGas:
  ~~~go
  if req.GasLimit != 0 {
      return req.GasLimit, nil
  }
  gas, headroomBps, err := m.estimateHorizonGas(ctx, req, parent)
  ~~~
- horizon.go:593 already provides the intended context pattern for stalled calls:
  ~~~go
  deadline := time.Now().Add(m.gasEstimateTimeout())
  if !pending.deadline.IsZero() && pending.deadline.Before(deadline) {
      deadline = pending.deadline
  }
  ctx, cancel := context.WithDeadline(ctx, deadline)
  defer cancel()
  return m.estimateHorizonGas(ctx, pending.req, parent)
  ~~~
- internal/txengine/txmanager.go:1859 derives gasEstimateTimeout from the existing
  maximum and half the configured replacement interval; reuse it without adding
  a YAML field or changing fixed daemon policy.
- internal/txmanager/txmanager.go:56 sends an engine Request with To, Data and Label,
  without an absolute request deadline. The initial worker calls broadcast
  synchronously at internal/txengine/txmanager.go:523.
- internal/txmanager/recovery_policy_test.go:134,
  TestCancelledUnsignedAdmissionKeepsIdentityUntilCompletion, cancels its caller
  after admission, verifies the unsigned identity remains owned, then manually
  releases a blocked estimate. Keep that safety assertion.
- Error exemplar in callGas:
  ~~~go
  return 0, errors.Errorf("estimate gas %q: %w", req.Label, err)
  ~~~
  Preserve wrapping and redaction. Do not log calldata or raw endpoint credentials.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| Engine regression | GOTOOLCHAIN=go1.27.1 go test -race ./internal/txengine -run 'TestInitialGasEstimate' -count=1 | all new cases pass after the fix |
| Journal ownership | GOTOOLCHAIN=go1.27.1 go test -race ./internal/txmanager -run 'Test.*Unsigned' -count=1 | ownership survives caller cancellation; timeout eventually resolves |
| Full gates | make check && make integration | exit 0, no failed or skipped Anvil tests |
| Scope/whitespace | git diff --check && git diff --name-only | exit 0; only allowed paths |

## Steps

### Step 1: Add bounded initial-estimate regressions

In horizon_test.go reuse horizonChain, hungEstimates and startManagerForTest from
the stalled-estimate tests. Create TestInitialGasEstimateTimeout, table-driven for
a hung next-block estimate and unsupported overrides followed by a hung fallback.
Configure a short existing replacement interval to obtain a short estimate budget.
Wait for the result with a test safety timeout; the production operation must time
out first. Count signed calls and broadcasts: both must remain zero.

Add TestInitialGasEstimateSharedBudget using a backend that spends part of the
budget rejecting overrides, then blocks in fallback. The fallback must see the
same deadline, not a fresh full timeout. Add TestInitialGasEstimateRequestDeadline
and a supplied-GasLimit case proving no estimator is called.

Verify: the engine regression command must fail before implementation on the
unbounded case, with the test safety timeout assertion rather than a suite hang.
Other existing tests must still pass. Capture that focused failure.

### Step 2: Budget the complete initial estimate

After the supplied-GasLimit early return in callGas, create one context bounded by
min(now + gasEstimateTimeout(), nonzero req.Deadline, any parent deadline).
Cancel it with defer. Pass that context to the single estimateHorizonGas call so
both override and fallback share the deadline. Keep the existing stalled path,
headroom, revert classification, fees, RPC routing and signing callbacks intact.

Verify: the engine regression command exits 0 for next-block, fallback, earlier
deadline, shared budget and explicit GasLimit cases. Do not make caller
cancellation stop an admitted process-owned operation.

### Step 3: Verify journal ownership across timeout

Extend recovery_policy_test.go with
TestUnsignedEstimateTimeoutReleasesOnlyAfterOutcome. Before the engine outcome
arrives, a cancelled caller must still see Pending and the same ID; a competing
request and acknowledgement remain blocked. After the timed-out unsigned outcome
is joined, there must be no signed journal, no broadcast, and a later healthy
request must be admitted. This is safe because signing never occurred; do not
generalize the release to signed or uncertain work.

Verify: run the journal ownership command, then the two full gates. Document the
finite unsigned-estimation budget in docs/TRANSACTIONS.md without implying a bound
on canonical settlement or permission to retry signed recipes.

## Test plan and done criteria

- [x] All named new tests exist and pass with -race.
- [x] The existing cancellation/ownership regression remains intact.
- [x] Initial override and fallback share one deadline; no new configuration.
- [x] A timed-out unsigned estimate signs/broadcasts zero transactions.
- [x] A later healthy request is admitted after joining the timeout result.
- [x] make check, make integration and git diff --check exit 0.
- [x] Scope matches the list and the index is updated.

## Specific stop conditions

Stop if the backend ignores context cancellation and solving it would require
unbounded helper goroutines or dropping an admitted identity. Stop if the fix
needs to clear any signed journal. A temporary estimate timeout is not evidence
that a previously signed transaction failed.

## Maintenance notes

New estimate variants must remain inside the same budget. Review that the timeout
is placed around estimation, not around the entire signed lifecycle. Future
deadline changes must preserve the distinction between caller cancellation and
manager-owned work.

## General stop conditions

- Source excerpts no longer describe the consuming path, other than reconciled
  dependency changes.
- A step fails twice after a reasonable focused repair.
- The change requires an out-of-scope path, a new dependency, a wire/schema
  migration, a weaker authority check or production credentials.
- The required toolchain, private module access or local Anvil is unavailable.
  Record the exact failed gate; do not replace it with live signing or chain I/O.

## Completion record

Implemented in ba5b41c. The initial estimator has one request/process-capped
budget shared by override and fallback. Explicit gas bypass is unchanged.
New initial-estimate cases and the unsigned ownership regression were observed
RED before the change and GREEN afterward; full engine/txmanager race tests pass.
Zero signing/broadcast on timeout and admission only after outcome join are covered.
Changed horizon.go, horizon_test.go, recovery_policy_test.go and TRANSACTIONS.md.
Independent cross-review approved. Combined final gates are in EXECUTION.md.
