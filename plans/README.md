# Improvement plans

Prepared with the improve skill on 2026-10-16 against clean main at
3184aa390edbeb66d1d6c5d1a62dc337794c4d6e.

The user selected all five audited findings and both grounded direction options.
The subsequent user instruction, "implement all", authorized execution of these
plans. All five source fixes and both Markdown design spikes are complete on
codex/improve-all in the isolated worktree
/Users/andreikorokhov/.codex/worktrees/improve-all/merkle-executor.
See [the execution record](EXECUTION.md) for commits, reviews and verification.
The advisor reviewed source changes made by separate executors and maintained
these plans. No push, PR, publication, merge or deployment was performed.
The primary checkout remains on main; pre-existing local work remains in stash
commit 5f99c99ab5c677336968e865720ee2d03773f7dd outside this implementation.
Pre-change excerpts below are retained as planning history, not current source.

## Execution order and status

| Plan | Title | Priority | Effort | Dependencies | Status | Delivery |
|---|---|---|---|---|---|---|
| [001](001-bound-initial-gas-estimation.md) | Bound initial gas estimation | P1 | S | None | DONE | Host change |
| [002](002-bound-confirmation-witness.md) | Bound canonical confirmation witness | P1 | M | 001 recommended first; technically independent | DONE | Host change |
| [003](003-classify-empty-redemption-budgets.md) | Classify empty redemption budgets as idle | P2 | S | None | DONE | Host change |
| [004](004-index-keeper-inventory.md) | Index durable inventory per iteration | P2 | M | None | DONE | SDK source; consumer delivery gate below |
| [005](005-preserve-http-cancellation.md) | Preserve sanitized HTTP cancellation | P3 | S | None | DONE | SDK source; consumer delivery gate below |
| [006](006-recipe-compatibility-spike.md) | Design authenticated compatibility inspection | P3 | M, coarse | None | DONE | Design spike only |
| [007](007-recovery-diagnostics-spike.md) | Design safe offline recovery diagnostics | P3 | M, coarse | None; recheck changed evidence access | DONE | Design spike only |

S means hours including tests; M means approximately one day for a source fix.
Direction estimates are coarse. Status vocabulary: TODO, IN PROGRESS, DONE,
BLOCKED (one-line reason), REJECTED (one-line rationale). DONE means the scope of
that row passed its criteria. For 006/007 it means an executable design handoff
exists, not that a new command is shipped. For 004/005 record SDK source completion
separately from consumer delivery below.

Completed direction deliverables:
[authenticated compatibility inspection](006-recipe-compatibility-design.md) and
[offline recovery diagnostics](007-recovery-diagnostics-design.md).
They specify future runtime work; no inspection or diagnose command is shipped.

Final local verification: GOWORK=off make check passed both modules with zero lint
issues; make integration passed all 15 top-level Anvil tests with no skips.
Temporary-workspace make check and make integration also passed against the
changed SDK. Lens/Noon artifact checks and five JSON design examples passed.
Hosted CI, container execution, live APIs and deployment were not tested.

## Dependency notes

1. Prefer 001, then 002 to remove the highest-impact sender availability risks.
   They are independent fixes. Both may edit docs/TRANSACTIONS.md; reconcile
   additive documentation drift rather than overwriting the earlier change.
2. 003 is a small independent provider-only correction.
3. 004 requires characterization and failure-injection tests before its refactor.
   Those steps are part of the plan, not a separate unchecked prerequisite.
4. 004 and 005 are independent SDK patches. Their source scopes do not require
   host module-file changes. Coordinate additive sdk/README.md changes.
5. Once accepted SDK patches are ready, a normal authorized release must make
   their immutable commit reachable, then update root go.mod/go.sum to that exact
   published SDK revision and rerun with GOWORK=off. A temporary workspace is
   local compatibility evidence and does not deliver the fixes to the daemon.
6. 006 and 007 are independent design spikes. Their subsequent runtime
   implementation needs a new self-contained handoff with reconciled excerpts,
   exact scope and concrete acceptance tests. They do not expand the current
   source-fix plans or authorize rewriting recipes/resetting ownership.

## SDK consumer delivery gate

Status: TODO -- SDK source accepted and verified; publication and immutable host
pin update remain pending.

This explicit delivery task prevents source-only SDK verification from being
reported as daemon delivery. It is separate from the completed source-fix scopes.
The local implementation request did not include publication or push; root
go.mod/go.sum are unchanged.

The authorized delivery executor must use an isolated worktree. Confirm one
reviewed SDK revision includes both 004 and 005 plus the lint
follow-up (local source head d6690d0daded2cb6ab71f035b4124f22c141f438), publish it
through the repository's normal workflow when authorized, and capture its actual
full SHA. Do not invent a version or use a production replace directive. With that real published
SHA in a task-specific variable, from the delivery checkout root:

~~~sh
GOWORK=off GOTOOLCHAIN=go1.27.1 go get "github.com/symbioticfi/merkle-executor/sdk@$task_sdk_commit"
GOWORK=off go list -m -f '{{.Path}} {{.Version}} {{.Dir}}' github.com/symbioticfi/merkle-executor/sdk
GOWORK=off make check
GOWORK=off make integration
~~~

Set task_sdk_commit to the verified published full SHA, not a literal placeholder.
Expected: go.mod/go.sum pin an immutable version resolving that commit, no replace
or committed go.work exists, both modules pass check and all local Anvil tests
pass. Inspect every module-file diff; unrelated upgrades are out of scope.
Only root go.mod, root go.sum, the SDK delivery note in docs/MERKLE-SDK.md, and this
index's delivery record are in scope for that delivery task. If publication or
private-module access is unavailable, mark delivery BLOCKED and preserve source
results; do not call local-workspace success a published-consumer pass.
Do not edit vault-solver's separate module; its adoption is another host's release.

Record here when available: reviewed SDK SHA, immutable required version, host
commit, local check/integration results, separately observed hosted CI, and any
release blocker. Neither a successful source commit nor this planning request
authorizes merge/deployment.

## Evidence and risk summary

| Finding | Audited behavior before implementation | Confidence | Fix risk |
|---|---|---|---|
| 001 | Initial estimation receives a process-owned context without its own budget | High | Low: preserve ownership until unsigned result arrives |
| 002 | Latest-to-receipt walks plus latest-head equality can fail repeatedly under latency/head growth | High code confidence; conditional production trigger | Medium: canonical proof/reorg semantics |
| 003 | Valid zero amounts reach constructors and become ordinary errors | High | Low: classify after valid economics, keep constructors strict |
| 004 | Full job loading was quadratic; H256/S256 reads fell from 66049 to 257 | High operation-count confidence; local benchmark recorded | Medium: same-iteration reservations and uncertain writes |
| 005 | Built-in HTTP errors lose context sentinel identity | High | Low: preserve safe sentinels only |
| 006 | Pure requirements and documented unsupported shapes can support inspection | Grounded option | Design/API maintenance cost |
| 007 | Job-only status omits active journal attempts/blocker evidence | Grounded option | Redaction and per-file snapshot consistency |

## Baseline and exclusions

At the planned revision, the advisor reran make check successfully for both modules
with zero lint issues and make integration successfully for all 15 top-level local
Anvil tests. Read-only lens and source-pinned Noon artifact checks also passed.
These are local EVM and HTTP/RPC fixture results. Do not reuse them as proof that
a later implementation passed: every executor reruns its own gates.

The audit covered handwritten daemon/SDK/provider packages and repository tooling,
configuration, architectural intent and docs. Excluded: stashed work, generated
binding internals, a fresh vulnerability-database scan, hosted CI/container
execution, live APIs and deployed permissions/settlement. No live signing was used.

## Findings considered and rejected

- Ambiguous nonce/transaction retention: intentional protection against replay of
  repeatable recipes; never remove it as a liveness optimization.
- Separate issuer request and claim recipes: deliberate asynchronous lifecycle.
- Exclusive target composition restrictions: deliberate receipt attribution guard.
- No batch fallback and fixed protocol/runtime endpoints: documented policy.
- Restrictive numerical prefixes, exact-result witnesses and unsupported shapes:
  documented execution support limits, not authority bypasses.
- Missing CoW authorization validation in the lower-level constructor: the daemon
  automatic wrapper checks exact canonical authorization evidence.
- RFQ API status or used nonce as settlement: current code requires exact
  canonical Reactor evidence rather than trusting those hints.
- Lens overflow or code-override identity divergence: pinned arithmetic is checked
  and the original-executor full result witness rejects divergence.
- SDK source versus pinned-module verification drift: root gates explicitly check
  both modules, and their SDK source matched at the audited revision.
- Manually malformed public helper objects: no configured-host reachability was
  established, so not promoted into a speculative runtime vulnerability.
- Cosmetic documentation/metric wording and broad refactors without concrete
  impact: not worth separate plans.
