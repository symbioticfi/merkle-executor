# Implementation record

Base: 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e
Worktree: /Users/andreikorokhov/.codex/worktrees/improve-all/merkle-executor
User authorization: implement all seven selected plans.

The five source fixes and two design spikes retain their accepted scopes.
Engine (001 then 002), SDK (004 then 005), and providers (003) have separate
executors. The advisor drafted 006/007 Markdown artifacts; a separate executor
reviewed them. The reviewer owns index updates and combined verification. Commit operations are serialized.

Pre-flight: engine/provider code scopes are disjoint. 001/002 share
TRANSACTIONS.md and execute sequentially. 004/005 share sdk/README.md and execute
sequentially. Direction spikes produce Markdown only. SDK source checks are
separate from the host's published-module pin and temporary workspace checks.

No live credentials, API mutations, publication, push, merge or deployment.
The original checkout and stash 5f99c99ab5c677336968e865720ee2d03773f7dd
remain unchanged.

Direction work: the advisor drafted the two Markdown-only design artifacts while
source executors worked. A separate executor reviewed them against their plans
and current source and approved the corrected artifacts. No runtime direction API
was implemented.

001: preliminary review approved; ba5b41c. Initial estimation and unsigned ownership
regressions observed RED then GREEN; full engine/txmanager race packages passed.
003: preliminary review approved; 3ea7976. Zero amounts and Figure intermediate
observed RED then GREEN; full provider race suite passed. Zero minimum remains its
existing non-idle validation error. Multi-route idle discards earlier prepared output.
004: preliminary review approved; 404db80. Linear load test observed RED then GREEN;
all keeper race tests, vet and 3 benchmark samples passed. H256/S256 reads fell
from 66049 to 257. Before/after timing is observational, not a release threshold.

Ruling: the shared isolated codex/improve-all worktree executes all source plans;
commits are serialized and path-scoped. This replaces individual worktree/branch
names while preserving source ownership and review boundaries.
Ruling: 002 adds receipt_reader_test.go to scope because new canonical proof reads
invalidate a mechanism count. Preserve distinct sweep/proof identities and all
per-read timeout assertions; no weaker production proof is allowed.

006/007 cold review requested nullable global provider indices, typed generic
composition codes at their actual boundary, a valid configured address in the
legacy-job example, explicit diagnostics.maxRecordBytes limits/path, and precise
orphan versus removed owner definitions. Those design corrections were applied.

002: reviewer and independent SDK executor approved; 36b28ee. Moving-head cases
were RED before and GREEN after; exact numbered witnesses, fresh receipt evidence,
null/error resets, positive-depth ownership and legacy/batch interruption pass.
005: reviewer approved; 8014bd7, followed by test-only d6690d0. Sentinel/redaction
regressions were RED then GREEN; actual built-in Source paths are covered.
The combined check exposed six test comparisons disallowed by errorlint; the
scoped follow-up uses errors.Is. Independent SDK lint now has zero issues.
006/007: corrected artifacts approved by the separate cold reviewer; five complete
JSON examples parsed successfully. These deliver designs only, not commands.

## Final combined verification

Source head: d6690d0daded2cb6ab71f035b4124f22c141f438. All 22 source/test/doc paths
match the combined scopes; the added receipt-reader test scope was approved above.
No module/dependency files, ABI/bindings, wire schemas or external repositories changed.

| Command | Observed result | Evidence boundary |
|---|---|---|
| GOWORK=off make check | PASS; root and SDK build, race tests, vet, lint; zero issues | Local modules; daemon uses published SDK pin |
| GOWORK=off make integration | PASS; 15 top-level Anvil tests, zero skips | Local ephemeral EVM and API/RPC fixtures |
| Temporary-workspace make check | PASS; root and SDK checks, zero issues | Daemon compiled/tested against changed local SDK |
| Temporary-workspace make integration | PASS; 15 top-level Anvil tests, zero skips | Local changed-SDK compatibility, not publication |
| python3 sdk/recipes/lens/generate.py --check | PASS | Read-only artifact verification |
| python3 internal/providers/redemptions/testdata/noon/generate.py --check | PASS | Source-pinned artifact verification |
| Parse all fenced JSON in both design artifacts | PASS; 2 compatibility + 3 diagnostics examples | Design structure only |
| git diff --check and combined path-scope check | PASS | Local patch |

The baseline integration result was obtained after all production edits; the only
subsequent source commit changes six test comparisons. Final check and workspace
integration ran after that follow-up. Logs are local ignored bin/improve-*.log.
The owned temporary Go workspace was removed after verification; no go.work or
production replace directive was added.

The ordinary root module still resolves SDK v0.0.0-20261013135138-320fd935451d.
004/005 are reviewed local SDK source changes; a normal authorized publication and
real immutable host pin update remain outstanding in the explicit delivery task.
No hosted CI, container build, live API/signing/deployment or external consumer
adoption was performed. No push, PR, merge, release or deployment.
