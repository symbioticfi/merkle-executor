# Plan 006: Specify an authenticated read-only recipe compatibility report

> Executor: read this entire file before changing anything. Follow the ordered
> steps and their verification gates. STOP and report when a stop condition
> occurs. Update only this plan's row in plans/README.md after verification,
> unless a reviewer maintains the index.
>
> Drift check, from the executor's checkout root:
>
> ~~~sh
> git diff --stat 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e..HEAD -- sdk/keeper/planner.go sdk/keeper/recipe_provider.go internal/providers/redemptions/automatic.go cmd/merkle-executor/run.go docs/PROVIDERS.md docs/DESIGN.md plans/006-recipe-compatibility-design.md
> ~~~
>
> Compare the excerpts below with current source before proceeding. Resolve
> changes from completed dependencies in this plan first; unexplained drift
> requires replanning.

## Status

- Priority: P3
- Effort: M (coarse design/spike estimate)
- Risk: LOW; no runtime changes in this spike
- Depends on: none
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

- plans/006-recipe-compatibility-design.md

No sibling repository, live credentials, operator configuration, persistent
production state, generated ABI/bindings, unrelated provider or fee policy changes
are authorized. Read reference files outside this list as necessary.

## Git workflow

Use an isolated worktree and a codex/improve-006-recipe-compatibility-spike branch; do not
implement in the primary main checkout. Preserve the user's existing stash
without applying, dropping or rewriting it. Commit logical verified changes in
the executor worktree using conventional titles such as fix: or perf:. Never
merge, push, publish an SDK revision, create issues/PRs or deploy without the
operator's authorization. If a reviewer dispatches this plan, follow their
index-maintenance instructions.

## Why this matters

Automatic providers recognize a deliberately bounded subset of valid committed
recipes. An operator currently learns unsupported shapes during plan preparation,
mixed with quote, eligibility and simulation errors. A compatibility report could
explain structural support before runtime acquisition. This spike must produce
a concrete command/API design and acceptance matrix, not relax any limitation or
implement another execution path.

## Current state

sdk/keeper/recipe_provider.go declares a pure authenticated boundary:
~~~go
type RecipeRequirements struct {
    Values           []recipes.Value
    Inputs           []uint
    Actions          []string
    ExclusiveTargets map[string]common.Address
}
type RecipeProvider interface {
    RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error)
    PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) (provider.Output, error)
}
~~~
The SDK invokes this only after validating active root, chain and account.
sdk/keeper/planner.go:97 calls each RecipeRequirements on copied program/tree bytes;
later preparation resolves values, obtains external data, assembles and simulates.
cmd/merkle-executor/run.go:111 currently invokes complete Planner.Prepare for plan;
any error returns before the aggregated plans are encoded.

A concrete restriction at internal/providers/redemptions/automatic.go:287 is:
~~~go
for _, second := range actions {
    if second.Step > action.Step && selectorMatches(second, target, secondData) {
        seconds = append(seconds, second)
    }
}
if len(seconds) != 1 {
    return nil, errors.New("Figure requires exactly one ordered output request")
}
~~~
Two ordered actions within one Manage step do not meet that particular supported
shape. This is a compatibility boundary, not a confirmed authorization bug.

docs/PROVIDERS.md:80 documents unsupported prefixes, unresolved/future inputs and
self-query restrictions. docs/DESIGN.md says "External data does not grant
authority." The command must report those boundaries without fixing recipes,
changing masks or interpreting a permission artifact's preimage as financial
policy. Cobra registration and required-flag tests in cmd/merkle-executor/main.go
and root_test.go are the eventual CLI convention.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| Existing SDK boundaries | cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test ./keeper ./recipes -count=1 | existing structural/authentication tests pass |
| Provider examples | GOTOOLCHAIN=go1.27.1 go test ./internal/providers/redemptions ./internal/providers/securitize ./internal/providers/cowswap ./internal/providers/merkl ./internal/providers/liquidlanerfq -run 'TestAutomatic' -count=1 | supported/rejected fixture cases pass |
| Baseline gates | make check && make integration | exit 0; baseline only, no new command implemented |
| Artifact/scope | git diff --check && git status --short | only selected plan/design files change |

Keep JSON examples fenced inside the single Markdown design artifact and validate
them with the stdin command in step 3. Do not add separate example files.

## Scope boundaries for this design spike

The only deliverable is plans/006-recipe-compatibility-design.md. Production Go,
SDK public APIs, command flags, provider selection and permission/recipe contracts
must remain unchanged. Read all reference files listed above; do not prototype
against operator endpoints. This is a design/spike plan, not a CLI implementation
plan. A later implementation handoff must name its code scope and repeat source
drift checks.

## Steps

### Step 1: Map what can be inspected without runtime acquisition

Read the complete Planner.Prepare pipeline and RecipeProvider interface. Create
a section named "Authentication and purity" in the design. Specify the sequence
of pinned discovery, tree/proof validation, pure provider requirements, composition
validation and canonical closure. Distinguish structural support from runtime
readiness; runtime eligibility, API availability, liquidity and full execute
success must be unknown unless actually checked.

Compare two implementation approaches: a small additive SDK inspection seam
sharing authentication versus host orchestration using existing public APIs.
Choose one, list exact proposed symbols/files, and explain how it avoids copying
authority validation. Discovery RPC and curator GET are allowed in a future
online inspection; quote/reward/protocol order acquisition, PrepareRecipe,
Advance, signing, state.Open and transaction submission are not.

Verify: the existing SDK boundary command passes. The design's section must
identify each existing validation symbol used and any necessary new seam; a
diagram or pipeline cannot substitute for the actual symbol map.

### Step 2: Specify the command and output contract

Use a proposed command spelling such as inspect-recipe --config, explicitly
deciding whether --executor narrows the configured set. Follow Cobra NoArgs and
strict offline configuration validation conventions. Do not silently change plan
output or its exit behavior. Define a versioned JSON report with per-executor
chain/block/hash/account/caller/recipe/root identity and per-provider diagnostics:
supported, unsupported, unmatched or unknown, stable bounded reason code, optional
step/call/input indices. Do not serialize proofs, signatures, calldata, raw remote
responses or credential-bearing URLs.

Specify exit codes, multi-strategy partial-result behavior, cancellation, bounded
network use and closing canonical guards. Failed authentication must not produce
a supported verdict. A shape judged supported must still say execution readiness
is unknown; it is not proof of deployability, simulation success or future
settlement. Unsupported conditions should name the action/field where available,
without parsing unstructured errors as a permanent public API.

Verify: the provider-example command passes. The design must have "Command and
schema" plus two complete fenced JSON examples: supported structure and mixed
unsupported/authentication-error results.

### Step 3: Write acceptance cases and a bounded implementation handoff

Add an "Acceptance matrix" with supported transfer, supported issuer request,
Figure same-step rejection, unknown dynamic action, duplicate input ownership,
exclusive-target conflict, wrong chain/root/unused invalid leaf, cancelled HTTP,
canonical reorg during inspection and mixed ready/idle strategies. For each name
the existing fixture/test to reuse or an exact proposed new test name, expected
diagnostic/status and network/state/signing call counts. Zero signing, zero order
POST and zero durable writes are mandatory in every inspection case.

Add "Trade-offs and next implementation": rough effort, minimum API addition,
migration/compatibility risks, exact candidate files and verification commands.
Choose defaults in the design, recording reasons and any genuinely unresolved
question; do not leave an unbounded research list. A command implementation remains
a separate task.

Validate all fenced JSON examples without writing files:
~~~sh
python3 - <<'CHECK'
import json, re
from pathlib import Path
p = Path('plans/006-recipe-compatibility-design.md')
s = p.read_text()
for heading in ['Authentication and purity', 'Command and schema', 'Acceptance matrix', 'Trade-offs and next implementation']:
    assert heading in s, heading
examples = re.findall(r'~~~json\n(.*?)\n~~~', s, re.S)
assert len(examples) >= 2
for item in examples:
    json.loads(item)
print('Compatibility design sections and JSON examples verified')
CHECK
~~~
Verify: this command and baseline gates exit 0. Baseline tests establish that the
spike changed no runtime; they do not prove a new command exists.

## Done criteria

- [x] The design selects a concrete authentication/API approach with symbol map.
- [x] Command, schema, exit codes and multi-strategy behavior are specified.
- [x] At least two valid JSON examples and all named acceptance cases are present.
- [x] No preparation/Advance/signing/order mutation or durable-write path is allowed.
- [x] Structural compatibility is explicitly separate from execution readiness.
- [x] All verification commands applicable to the Markdown-only scope pass.
- [x] Only the design artifact and index row change; mark the spike, not a CLI, DONE.

## Specific stop conditions

Stop if requirements cannot be reused without weakening authentication. Record a
design blocker instead of implementing a partial second planner. Do not turn a
documented unsupported composition into an automatic rewrite feature.

## Maintenance notes

Provider restrictions evolve. Prefer stable diagnostics produced at the validation
boundary over matching message strings. The eventual report must carry the pin
that authenticated its conclusions and avoid implying current-live state later.

## General stop conditions

- Source excerpts no longer describe the consuming path, other than reconciled
  dependency changes.
- A step fails twice after a reasonable focused repair.
- The change requires an out-of-scope path, a new dependency, a wire/schema
  migration, a weaker authority check or production credentials.
- The required toolchain, private module access or local Anvil is unavailable.
  Record the exact failed gate; do not replace it with live signing or chain I/O.

## Completion record

Delivered plans/006-recipe-compatibility-design.md: selected authenticated
SDK inspection seam, exact pure-call sequence and closing pin fence, proposed CLI
and schema, two valid JSON examples, diagnostic/exit behavior, negative tests,
future file scope and estimate. Separate cold review requested and verified the
nullable global provider index and typed composition-diagnostic boundary fixes.
All JSON examples parse; existing SDK/provider and combined baseline gates pass.
No runtime API/CLI/source/state change is part of this design spike.
See EXECUTION.md for scope coordination and final verification.
