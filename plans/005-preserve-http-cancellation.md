# Plan 005: Preserve sanitized HTTP cancellation identity through permission acquisition

> Executor: read this entire file before changing anything. Follow the ordered
> steps and their verification gates. STOP and report when a stop condition
> occurs. Update only this plan's row in plans/README.md after verification,
> unless a reviewer maintains the index.
>
> Drift check, from the executor's checkout root:
>
> ~~~sh
> git diff --stat 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e..HEAD -- sdk/protocolio/io.go sdk/protocolio/io_test.go sdk/permissions/source_test.go sdk/README.md
> ~~~
>
> Compare the excerpts below with current source before proceeding. Resolve
> changes from completed dependencies in this plan first; unexplained drift
> requires replanning.

## Status

- Priority: P3
- Effort: S (hours including tests)
- Risk: LOW
- Depends on: none; release together with 004 when both are accepted
- Category: bug / docs
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

- sdk/protocolio/io.go
- sdk/protocolio/io_test.go
- sdk/permissions/source_test.go
- sdk/README.md

No sibling repository, live credentials, operator configuration, persistent
production state, generated ABI/bindings, unrelated provider or fee policy changes
are authorized. Read reference files outside this list as necessary.

## Git workflow

Use an isolated worktree and a codex/improve-005-preserve-http-cancellation branch; do not
implement in the primary main checkout. Preserve the user's existing stash
without applying, dropping or rewriting it. Commit logical verified changes in
the executor worktree using conventional titles such as fix: or perf:. Never
merge, push, publish an SDK revision, create issues/PRs or deploy without the
operator's authorization. If a reviewer dispatches this plan, follow their
index-maintenance instructions.

## Why this matters

The SDK HTTP client replaces transport and body-read failures with new redacted
errors. That correctly hides URLs and payloads but discards context cancellation
and deadline identity. The built-in permission source forwards these errors, so
errors.Is cannot satisfy the SDK's documented contract. Retain only safe context
sentinels while continuing to redact arbitrary underlying transport errors.

## Current state

sdk/protocolio/io.go:101:
~~~go
resp, e := c.http.Do(req)
if e != nil {
    return 0, errors.New("protocol HTTP transport failed")
}
~~~
At :109:
~~~go
b, e := io.ReadAll(io.LimitReader(resp.Body, c.limit+1))
if e != nil {
    return resp.StatusCode, errors.New("read protocol response")
}
~~~
JSON derives a finite child context with context.WithTimeout at :88. Its HTTP
client refuses redirects and bounds response size. Preserve those behaviors,
status values and sanitized public messages.

sdk/permissions/source.go:62 calls JSON and forwards its error. The planner in
sdk/keeper/planner.go:217 already wraps with go-errors:
~~~go
return nil, errors.Errorf("permission source failed: %w", err)
~~~
Follow that wrapping style only for the safe context sentinel. Do not wrap a raw
url.Error, connection error, response body or arbitrary custom cancellation cause.

sdk/README.md:117 says: "Source errors must omit credentials, complete URLs and
remote payloads; wrapped causes preserve cancellation." Existing
TestPlannerPermissionSourceVerifiesAllBytesBeforeProviders checks a custom source
returning context.Canceled. The built-in source test currently checks only a
nonnil error; extend the actual HTTP path.

## Commands you will need

| Purpose | Command | Expected |
|---|---|---|
| HTTP sentinel/redaction cases | cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test -race ./protocolio -run 'TestHTTP' -count=1 | cancellation, deadline and redaction assertions pass |
| Built-in source | cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test -race ./permissions ./keeper -run 'Test.*Source' -count=1 | end-to-end identity and existing authority checks pass |
| SDK full gates | make -C sdk check && make -C sdk integration | exit 0 |
| Root baseline | make check && make integration | exit 0; host resolves its immutable published SDK |
| Scope | git diff --check && git diff --name-only | only allowed paths |

## Steps

### Step 1: Add real HTTP cancellation and body-read cases

Extend io_test.go using httptest/local RoundTripper/body fixtures. Add
TestHTTPCancellationIdentity and TestHTTPDeadlineIdentity for cancellation before
dispatch, while waiting for headers and during response body reading. Use channel
coordination or context-driven fake transports rather than long sleeps. Add a
plain transport error and a custom context cancellation cause carrying a synthetic
sensitive marker. Assert errors.Is against only context.Canceled or
context.DeadlineExceeded when appropriate; the sensitive marker, complete URL,
credential and response body must not appear in rendered errors or exposed raw
transport causes. Preserve status=0 for transport failure and the received status
for body failure.

Verify: the HTTP command fails on sentinel checks before the fix. Existing
TestHTTPRedactsAndBounds and redirect cases remain passing.

### Step 2: Preserve the safe sentinel without reopening raw causes

At transport/body failures, inspect the operation context and safe sentinel
identity from the returned error. Return the same bounded message wrapping only
context.Canceled or context.DeadlineExceeded. Do not use context.Cause if it may
be an arbitrary payload-bearing value. If no safe cancellation/deadline applies,
keep the existing new sanitized error. Keep the actual failure as an error even
if a context expires concurrently; never convert it to success.

Verify: the HTTP command passes; noncontext errors remain noncontext errors and
the complete redaction matrix remains passing. Do not change CallData, retry
policy, timeout values, redirects or authentication under this plan.

### Step 3: Exercise the built-in permission source and local consumer

Add TestSourceCancellationIdentity to sdk/permissions/source_test.go using Source
with its real protocolio.Client. Test caller cancellation and deadline during
transport and body phases; errors.Is must reach the sentinel through this actual
source. Leave invalid roots/chains, oversize/malformed JSON and provider-before-
authentication guards unchanged.

Run the SDK full gates. To validate the changed SDK in the host without modifying
the committed dependency, use this temporary workspace from checkout root:
~~~sh
task_checkout="$PWD"
task_workspace="$(mktemp -d)"
(cd "$task_workspace" && GOTOOLCHAIN=go1.27.1 go work init "$task_checkout" "$task_checkout/sdk")
GOWORK="$task_workspace/go.work" make check
GOWORK="$task_workspace/go.work" make integration
~~~
Remove only that owned temporary workspace. Run the root baseline afterward.
Update the SDK documentation with the exact safe-cause contract if needed.

Verify: all listed source/SDK/workspace gates exit 0. Record that published-module
delivery requires normal authorized SDK publication and a daemon immutable pin
refresh; do not claim workspace tests updated the production module. Coordinate
this release with 004 if both source changes are accepted, as explained in the
index. No module-file edits or publication are in this plan's source scope.

## Test plan and done criteria

- [x] Built-in HTTP and permission source preserve both context sentinels.
- [x] Header and body failure phases have meaningful coverage.
- [x] Arbitrary transport/custom cancellation causes remain redacted.
- [x] Status values, response bounds, redirects and timeout policy are unchanged.
- [x] SDK and local-workspace consumer gates pass; root baseline still passes.
- [x] Source completion and pending published-consumer delivery are distinguished.
- [x] Scope, whitespace and index checks pass.

## Specific stop conditions

Stop if retaining identity requires exposing a raw error or response. Do not
change public retry semantics or conflate cancellation with normal provider idle.
If a downstream consumer needs a new interface, report it and replan.

## Maintenance notes

Future sanitized I/O boundaries should explicitly decide which sentinels are safe
to preserve. Keep tests at both the transport boundary and a consuming API;
testing a custom source alone will not catch this regression.

## General stop conditions

- Source excerpts no longer describe the consuming path, other than reconciled
  dependency changes.
- A step fails twice after a reasonable focused repair.
- The change requires an out-of-scope path, a new dependency, a wire/schema
  migration, a weaker authority check or production credentials.
- The required toolchain, private module access or local Anvil is unavailable.
  Record the exact failed gate; do not replace it with live signing or chain I/O.

## Completion record

Implemented in 8014bd7 with test-only lint follow-up d6690d0. HTTP failures
wrap only safe cancellation/deadline sentinels while retaining redacted messages,
status behavior and existing bounds/redirect/timeout policy. Real built-in source
and transport/body cases, custom causes and redaction assertions were observed
RED then GREEN. Focused protocolio/permissions/keeper race tests and vet pass.
The combined gate caught six errorlint comparisons in the new tests; they now
use errors.Is and SDK lint reports zero issues. Reviewer approved both diffs.
Changed protocolio/io.go/tests, permissions/source_test.go and sdk/README.md.
Combined baseline/workspace gates are in EXECUTION.md. Source is complete;
published SDK adoption remains the separate delivery gate in README.md.
