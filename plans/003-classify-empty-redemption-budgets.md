# Plan 003: Report empty authenticated redemption budgets as normal idle

> Executor: read this entire file before changing anything. Follow the ordered
> steps and their verification gates. STOP and report when a stop condition
> occurs. Update only this plan's row in plans/README.md after verification,
> unless a reviewer maintains the index.
>
> Drift check, from the executor's checkout root:
>
> ~~~sh
> git diff --stat 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e..HEAD -- internal/providers/redemptions/automatic.go internal/providers/redemptions/automatic_test.go internal/providers/securitize/automatic.go internal/providers/securitize/automatic_test.go docs/PROVIDERS.md
> ~~~
>
> Compare the excerpts below with current source before proceeding. Resolve
> changes from completed dependencies in this plan first; unexplained drift
> requires replanning.

## Status

- Priority: P2
- Effort: S (hours including tests)
- Risk: LOW
- Depends on: none
- Category: bug / dx
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

- internal/providers/redemptions/automatic.go
- internal/providers/redemptions/automatic_test.go
- internal/providers/securitize/automatic.go
- internal/providers/securitize/automatic_test.go
- docs/PROVIDERS.md

No sibling repository, live credentials, operator configuration, persistent
production state, generated ABI/bindings, unrelated provider or fee policy changes
are authorized. Read reference files outside this list as necessary.

## Git workflow

Use an isolated worktree and a codex/improve-003-classify-empty-redemption-budgets branch; do not
implement in the primary main checkout. Preserve the user's existing stash
without applying, dropping or rewriting it. Commit logical verified changes in
the executor worktree using conventional titles such as fix: or perf:. Never
merge, push, publish an SDK revision, create issues/PRs or deploy without the
operator's authorization. If a reviewer dispatches this plan, follow their
index-maintenance instructions.

## Why this matters

An emptied account is a normal state for a balance-derived recipe. Automatic
issuer and Securitize providers turn its zero budget into an invalid configuration
error, causing repeated error observations and failed readiness. Return the
existing idle sentinel at the automatic boundary, while strict constructors and
invalid economic constraints continue failing.

## Current state

internal/providers/redemptions/automatic.go:410:
~~~go
if route.Amount != nil {
    amount, err := recipes.EconomicResult(*route.Amount, results)
    if err != nil {
        return provider.Output{}, err
    }
    cfg.Amount = amount.String()
    cfg.AmountInput = route.Amount.Input
}
~~~
It later calls New(cfg, p.deps). The strict constructor at provider.go:128 rejects
zero: "redemption amount must be positive uint256". Figure intermediate amounts
are also required positive; zero derived intermediate quantities need the same
classification if they arise from a valid selected route.

internal/providers/securitize/automatic.go:259:
~~~go
amount, err := recipes.EconomicResult(route.Amount, results)
if err != nil {
    return provider.Output{}, err
}
cfg.Amount = amount.String()
cfg.AmountInput = route.Amount.Input
~~~
Its constructor at provider.go:93 also requires a positive amount.
sdk/provider/provider.go:75 defines ErrNotReady as normal idle when budget is
unavailable. sdk/keeper/keeper.go:201 treats it as idle and does not return a failed
iteration. The exemplar at internal/providers/cowswap/automatic.go:172 is:
~~~go
if amount.Sign() == 0 || minimum.Sign() == 0 {
    return provider.Output{}, provider.ErrNotReady
}
~~~
Use only the amount part of that exemplar: a zero minimum output is not
automatically invalid or idle for every issuer.

EconomicResult in sdk/recipes/derivation.go:455 validates lower and upper bounds.
A lower-bound violation must remain a real error, even if a balance is zero.
No claim ID, queue epoch or withdrawal request ID may be classified as a budget:
zero is valid for some of those identities.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| New boundary cases | GOTOOLCHAIN=go1.27.1 go test -race ./internal/providers/redemptions ./internal/providers/securitize -run 'TestAutomatic.*ZeroBudget' -count=1 | zero budget is idle, invalid economics remain errors |
| Complete provider regressions | GOTOOLCHAIN=go1.27.1 go test -race ./internal/providers/... -count=1 | all pass |
| Keeper sentinel behavior | GOTOOLCHAIN=go1.27.1 make -C sdk test | idle/invalid-provider distinction stays passing |
| Full gates | make check && make integration | exit 0 |
| Scope | git diff --check && git diff --name-only | only allowed paths |

## Steps

### Step 1: Characterize zero and invalid financial values

Reuse automaticRecipe/sealAutomaticTree and existing fixture readers in both
automatic_test.go files. Add TestAutomaticIssuerZeroBudget and
TestAutomaticSecuritizeZeroBudget, table-driven across request/native/atomic routes
that actually own a numerical amount. Include Figure intermediate zero if the
existing supported recipe can resolve one. For each valid zero budget require
errors.Is(err, provider.ErrNotReady), no returned action data and no protocol I/O
beyond the reads already used to derive the numerical witness.

Add distinct assertions for missing result, negative or invalid result where
accepted by the helper boundary, and a zero balance violating a positive lower
bound: errors must remain non-idle. Include a supported claim whose request ID or
epoch is zero and ensure its original semantics remain intact.

Verify: the new boundary command fails on the idle-sentinel assertion before the
fix. Existing strict-constructor tests continue rejecting explicit amount zero.

### Step 2: Classify only successfully resolved zero amounts

Immediately after successful EconomicResult resolution and before assigning a
runtime configuration/constructing a provider, return provider.ErrNotReady when
the selected financial amount is zero. Apply the same rule to a required Figure
financial intermediate. Do not alter New, EconomicResult, selection templates,
claim identifiers, target ownership or any recipe/mask byte.

For multi-route recipes, a selected route with no budget leaves the entire
preparation idle with empty output; do not invent a skip or partially execute
the other routes. Keep existing bound-validation error precedence.

Verify: run the new boundary command, then complete provider regressions. Tighten
the existing native Noon zero assertion near automatic_test.go:680 to assert the
sentinel explicitly.

### Step 3: Document the idle boundary and run consumers

Document in docs/PROVIDERS.md that a valid selected financial route with no
available amount is idle, whereas malformed data or unmet committed lower bounds
are errors. Preserve the distinction between submission, requested, paid and
settled outcomes.

Verify: run keeper sentinel behavior and the full gates. No SDK publication or
runtime policy/schema migration is needed for this provider-only classification.

## Test plan and done criteria

- [x] Both automatic families return the idle sentinel for valid zero budgets.
- [x] Zero/bad explicit constructor input still fails strict validation.
- [x] Missing results and violated bounds remain errors, not idle.
- [x] Zero request IDs/epochs remain valid where previously supported.
- [x] Idle preparation returns no partial actions and makes no external mutation.
- [x] All listed gates pass; scope and index are correct.

## Specific stop conditions

Stop if a zero amount cannot be distinguished from a missing/malformed economic
witness without changing SDK arithmetic. Do not loosen constructors, fill a
different amount, add skip instructions or change recipe bytes to make execution
succeed.

## Maintenance notes

New automatic providers should classify empty resolved budgets before strict
runtime construction. Keep amount/identity distinctions explicit in tests; do
not infer idle from arbitrary error strings.

## General stop conditions

- Source excerpts no longer describe the consuming path, other than reconciled
  dependency changes.
- A step fails twice after a reasonable focused repair.
- The change requires an out-of-scope path, a new dependency, a wire/schema
  migration, a weaker authority check or production credentials.
- The required toolchain, private module access or local Anvil is unavailable.
  Record the exact failed gate; do not replace it with live signing or chain I/O.

## Completion record

Implemented in 3ea7976. Valid zero financial amounts, including Figure's
required intermediate, return ErrNotReady with empty output; strict constructors,
invalid economics and zero request/epoch identities preserve their semantics.
New issuer/Securitize sentinel cases were observed RED then GREEN; the complete
provider race suite passes, including multi-route output disposal.
Changed the two automatic.go/test pairs and PROVIDERS.md. Reviewer approved.
Combined final gates are in EXECUTION.md. Evidence is local RPC/API fixtures.
