# Local verification

Validated on 2026-09-28 on macOS arm64 with Go 1.27.1 against the adapters
encoding at `bd5e235899de5e175a0001a913d47596f6f4f801`:

| Command | Observed result |
| --- | --- |
| `make check` | Build, race tests, vet and lint passed. |
| `make integration` | All four local Anvil tests passed. |
| `make generate` | All 21 generated Go artifacts reproduced byte for byte. |
| Recipe and issuer fixture generators | All seven fixture files reproduced byte for byte; adapter source hashes matched the compiled artifacts. |
| `bin/merkle-executor check-config --config config/example.yaml` | Example configuration accepted. |

The Anvil tests deploy the current RecipeExecutor bytecode with account/token
fixtures. They exercise event discovery, permission proofs, result bindings,
all 12 operations, approval reset, direct calls and skipped gates. Packed-value
coverage includes maximum `uint216` literals, `uint32` binding bounds and full
`uint256` inputs/results/writes. Independent cast vectors verify recipe hashing,
selectors and the event topic. Tests also preserve high operand bytes and trailing
ABI bytes in the commitment and submitted calldata. The service test runs
planning, a signed local transaction and restart recovery through the application
entrypoint. Planning succeeds without valid signing credentials and does not
create transaction state; restarting does not repeat completed work.

The issuer lifecycle test uses two strategies on one account: a request consumes
its exact share allowance, records confirmed submission and releases the account
lock; its claim strategy stays idle until the local issuer fulfills the request,
then claims the available shares and receives output. Both survive restarts
without resubmission. This executes the real RecipeExecutor with local issuer
contracts, not the deployed Centrifuge protocol.

Provider tests use local HTTP/RPC fixtures and independent encoding/hash vectors.
Liquid Lane uses the refreshed connector ABI and `outputAsset()` read; its fixture
checks matching and mismatched payment assets. This is not a live connector/backend
integration. The new issuer tests prepare all 15 published LL assets plus sUSN and all 29
published Securitize EVM routes (25 wallet routes and four atomic offramps).
They assert pinned targets and call arguments, generated calldata, identity,
readiness, claim caps and units. Regression tests cover Figure exchange-rate
changes between preview and execution, Pareto APR0 and closed-pool claims, skipped
Manage calls, receipt identity and payment evidence. Noon tests check independent
request/claim calldata vectors, fixed USN entitlement, live withdrawal-period
boundaries, zero request IDs, role revocation after a request, invalid topology
and exact confirmed call identity. The Anvil issuer lifecycle remains a local
Centrifuge-style fixture; no Noon contract is deployed in that integration test.
Statement coverage for the issuer packages is 91.9% (`redemptions`) and 90.8%
(`securitize`).
Transaction and keeper tests cover ambiguous sends, retained nonce ownership,
durable receipt/log recovery, canonical confirmations, receipt reorgs and
configuration changes. The reviewed catalogues record public-source evidence and
read-only topology checks separately from local fixture tests.

No live transactions or orders were submitted. Hosted CI and the Docker image
were not run. This evidence does not verify production API availability, operator
configuration, deployed permissions or issuer-specific settlement workflows.
See [provider boundaries](PROVIDERS.md) before configuring an integration.


## Local transaction engine — 2026-10-08

`make check` passed after the transaction-engine integration: build, complete race/coverage suite, vet and lint (zero issues). The local engine retains fee, gas, nonce, replacement, receipt and shutdown regressions. Source-parity scaffolding was removed when the engine became independently maintained.

`make integration` passed with the real RecipeExecutor application fixtures and the adapted upstream Anvil suite. This includes same-nonce fee replacements, silent relay acceptance, foreign nonce consumption, three same-key replicas and dropped pending calls after restart. Late receipt recovery is tested at the durable journal boundary, which owns it. The application flow also covers issuer request/claim, confirmed execution evidence and no duplicate completed work after restart. The engine permits fresh business calls after abandonment; Merkle's durable adapter intentionally retains uncertain repeatable recipes until it has canonical evidence.

New adapter regressions cover persistence before original/replacement broadcasts, an older signed variant winning, caller cancellation before signing, restart without recipe replay, canonical receipt re-inclusion with refreshed block/log/status evidence, and recovery checking a newer hash when an older receipt read fails. Configuration and RPC fixtures cover invalid/NaN fee policy, duration overflow, next-block overrides, typed invalid-params causes, classification and credential redaction for every new RPC method. An independent review found no remaining concrete blocker after these fixes.

These checks use ephemeral local chains and local HTTP/RPC fixtures. No production transactions or orders were submitted; production deployment and operator configuration were not verified.

After the local-engine cleanup, `make check` and `make integration` passed again; lint reported zero issues and engine statement coverage was 92.2%. Both the duplicated operation-label and invalid receipt-status cost regressions failed before their fixes and pass afterward. Journal tests cover late successful and reverted receipts after active tracking ends, uncertain-job admission/acknowledgement blocking, no recovery rebroadcast and durable terminal evidence. Independent review found no remaining blockers. The primary checkout's tracked patch and untracked-file hashes remain unchanged.


## Curator permissions and account discovery — 2026-10-13

The executor-only configuration regression failed against the prior decoder because
`curator` was unknown. It passes with required curator settings and strict rejection
of the removed strategy `account`/`tree` fields. Local fixtures cover pinned startup
account discovery, distinct target/account association, deduplication/chunking,
canonical closure, atomic failure and recovery fingerprint rejection without saved
state changes. SDK fixtures reject source errors, empty/mismatched trees and invalid
unused leaves before providers run, preserve cancellation and legacy local trees,
and reject metadata/reorg changes while permission acquisition is in flight.

The real local Anvil service plans and sends using backend-shaped HTTP permissions
and a derived account. HTTP503, wrong-chain permissions and an accepted-root change
during HTTP each prevent signing/broadcast, leave sender nonce and recipient balance
unchanged, and create no prepared/authorized transaction journal. A healthy retry
executes the existing transfer assertions. Completed-work restart makes no new curator
request. The local issuer request/claim lifecycle also uses remote permissions and
retains its balance, canonical receipt and restart assertions.

Small daemon HTTP/RPC fixtures prove Merkl reward acquisition and CoW quote acquisition
still contact their own APIs using the derived account; signed Liquid Lane daemon
registration is absent. Existing provider fixtures continue covering their protocol
semantics. Permission-source presend tests confirm validation does not refetch trees.

`make check` passed for both modules: build, race/coverage tests, vet and lint with
zero issues. `make integration` passed all ten top-level local Anvil tests (two SDK,
four application and four transaction-engine tests, including their lifecycle
subcases), with no skips. `make generate` reproduced the generated artifacts without
diff; `git diff --check` passed. These initial gates used the local two-module
workspace; published-module verification follows below.

This is local EVM and API-fixture evidence. The curator route is sourced from the
recorded backend revision and was absent from the inspected latest stage. Backend
deployment availability, hosted CI and live third-party transactions are not claimed.

The permission-source SDK was published at revision
`80f0c0d6d4a3f93c52b7c92501162c083ac4dd46`. With `GOWORK=off`, the host
resolved and pinned `v0.0.0-20261013102206-80f0c0d6d4a3`; module checksums
were refreshed without a replace directive. With that published pin and `GOWORK=off`,
`make check` passed both modules with zero lint issues, `make integration` passed
all ten top-level local Anvil tests with no skips, and `make generate` passed.
`git diff --exit-code -- api sdk/bindings` confirmed no generated drift;
`git diff --check` also passed. Independent full review found no actionable issue.
These standalone checks do not establish hosted CI or deployment availability.


## Liquid Lane RFQ consumer — 2026-10-13

The daemon registration regression first failed with unknown provider
`liquidlane-rfq`; it passes after registration. A cancellation-before-Fill fixture
also failed before contradictory evidence was rejected. The dense-history regression
first failed with a range-log limit error for 1,025 unrelated Fill events. It now
checkpoints a complete subdivided range and finds the later matching Fill after
restart, including when the head nonce is already used. Single-block overflow fails
explicitly; indexed account cancellation filtering ignores unrelated cancellations.

Focused provider tests pass for the independent cast EIP712 vector and go-ethereum
apitypes parity, exact 804-byte authorization, typed schema/domain/policy rejection,
leading-zero numeric identity, staged/direct parity and pin mismatch, failed-batch
no-fallback, three safe plus two isolated Advance getters, HTTP cancellation/bounds/
redaction, persisted authority tampering, incomplete/complete pagination, ambiguous
publication, canonical receipt membership, cancellation/Fill ordering, deadline
equality, post-fill signature/budget changes, corrupt cursors and orphan rewind.

`TestAnvilLiquidLaneRFQConsumerRestartAndFill` passes with deployed local Multicall3,
the real RecipeExecutor artifact and test account/token/Reactor contracts. Plan
requests a quote but sends no authorization or order. Send executes the approved
Reactor allowance and exact Sign, then preserves an accepted-but-502 publication as
pending. Restart looks up the saved identity without a new quote, curator tree,
authorization nonce or POST; API-only filled status stays pending. Before filler
execution the account retains input1,000 and output0 with allowance1,000. A canonical
fixture Fill transfers input1,000 and output950 and settles the job. A further
restart repeats no external mutation. The token balance, nonce and durable job
assertions are part of the test, not inferred from a successful command.

`make generate` and RFQ fixture regeneration preserved all 26 generated file hashes.
`git diff --check` passed; SDK sources and go.mod/go.sum remain unchanged. The new
fixtures reproduce the pinned account's hash program and Reactor request settlement;
protocol authentication is deliberately simplified in the local Reactor fixture.
[Fixture limitations and reproduction](../internal/providers/liquidlanerfq/testdata/README.md)
and [wire/source provenance](../api/schemas/LIQUID_LANE_RFQ.md) describe that boundary.
With the unchanged published SDK pin and `GOWORK=off`, the coordinator independently
ran `make check`: host and SDK builds, race/coverage tests, vet and lint all passed
with zero lint issues. New consumer provider statement coverage was 84.2%.
`GOWORK=off make integration` passed all eleven top-level Anvil tests with no failures
or skips. Logs are retained in the ignored `.superpowers/sdd/007-root/` directory.
Independent review prompted the adaptive dense-range regression and corrected the
wire cursor schema to string-or-null; regenerated Go output was unchanged.
No live RFQ order, chain mutation, production deployment or hosted CI result is
established by these local checks.

## Automatic native Noon USN redemption (2026-10-13)

The authenticated `redeem-usn` route completes the separate USN→USDC leg while leaving
sUSN request/claim behavior unchanged. Deterministic tests cover canonical route
configuration, full-amount quote units, whitelist/collateral readiness, malformed
and unavailable reads, treasury balance and allowance, block-budget boundaries,
prior-block counter reset, exact managed calldata and burn/payout receipt identity.
Automatic-provider regressions failed before this adaptation: native selection
returned no route, and recovery attempted to pack the claim ABI with a missing
request ID. They now pass alongside bounded-amount, pure invalid-identity,
policy/calldata/input/account tampering and missing terminal-evidence tests.
The normal host test uses address-only YAML, curator HTTP permissions, discovered
account/caller, one shared numerical lens, fixed automatic registrations and
read-bounded USN amount. It confirms the native transaction through the existing
sender, persists `redeemed` under the canonical address and does not resend after
restart.

The source-pinned local test compiles the unmodified verified RedeemHandlerV2
with native solc **0.8.28+commit.7893614a**, **Paris**, optimizer **200**, and the
original literal-content metadata settings. The existing real RecipeExecutor
and permission-enforcing account fixture execute exact approval, redemption,
allowance cleanup and minimum-output checks. Local token/oracle fixtures prove
a 1.234567890123456789-USN burn produces 1.233332 USDC at a 99.9% peg. Tests also
reject whitelist removal, an empty treasury, missing treasury allowance,
exhausted block budget, stale oracle data and a changed quote below the recipe's
minimum. Mined reverting executions prove both the post-burn allowance failure
and the post-payout minimum-output failure roll back balances, approvals and
logs. The real full-amount quote for 2,999,999,999,999 wei USN at a 99.99% peg
is one USDC base unit, where scaling the one-token rate would produce two.
Below-$1 collateral also preserves the handler's $1 quote clamp and differs
from its collateral `getRate` view.

```sh
python3 internal/providers/redemptions/testdata/noon/generate.py --check
make check
make integration
```

`make integration` starts owned ephemeral Anvil processes; the native test sets
its local chain ID to 1 and installs locally compiled runtime at the reviewed
route addresses. It uses no mainnet fork, live keys or external chain writes.
The handler runtime/source is real; USN/USDC token behavior, oracle and account
are local fixtures. These tests do not verify live account enrollment, production
USN/USDC pause, blacklist or whitelist restrictions, deployment liquidity,
future upgrades or issuer availability.
