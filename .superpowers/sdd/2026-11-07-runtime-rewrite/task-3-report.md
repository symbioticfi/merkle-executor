# Task 3: transaction driver

Implemented in the isolated rewrite checkout only. No original-checkout files, old managers, live credentials, remote APIs or deployments were touched.

## Contract and ownership

- `sdk/runtime/transaction.go` defines the agreed Intent/Transaction/TransactionResult/PersistTransaction/Driver contract. Attempts are signed wire bytes (`[]hexutil.Bytes`) in signing order. Canonical terminal values are `confirmed` and `reverted`.
- `internal/transaction` is a new driver directly over separate ReadBackend and WriteBackend interfaces plus the existing host Signer. There is no queue, alternate send mode, cancellation transaction, abandoned-nonce reuse mechanism or copied old manager lifecycle.
- Reserve fences every other job using this daemon driver instance. Execute/Resume each acquire one observer flag, so two callers cannot independently track or sign the same reservation. Exclusive process/state ownership and wiring every signer job through this instance remain service/coordinator responsibilities.
- Unsigned admission is persisted before estimate/sign work. Every signed original/replacement is validated and persisted before broadcast. Signed-journal persistence failure returns Pending, never NotSubmitted, because an atomic rename may already have completed. Any persistence failure poisons the instance and prevents subsequent admission/release until durable truth is reloaded by restarting ownership.
- Release is explicit. Pending signed work refuses release; canonical terminal evidence must have been freshly verified and accepted by the callback. Restored terminal records do not gain release authority from saved fields alone. The runtime callback must persist the workflow phase and transaction evidence atomically.
- An admitted unsigned Resume returns NotSubmitted without network mutation/signing. Its durable job identity remains until the runtime commits its halt/refresh decision and calls Release. Signed Resume observes every saved hash and never signs or broadcasts.

## Preserved behavior

Retained/adapted pure EIP-1559 horizon, tip/fullness, replacement growth, gas-headroom, next-block override classification and sent-head arithmetic from txengine, with their meaningful table-driven regression fixtures. Original cap reserves one 9/8 replacement; replacements grow both fee caps by at least ceil(9/8), retain sender/chain/nonce/to/value/data, and remain under the configured ceiling. At the ceiling the driver can rebroadcast the last exact bytes, rather than weakening fee-growth requirements.

A transport-ambiguous active send gets its exact-byte retry before fee replacement. Nonce-conflict responses stop mutation. Fresh and replacement signing close slow estimate/fee phases with the write endpoint's mined nonce; pending nonce counts are absent from the interface. A canonical owned nonce floor prevents lagged write-endpoint reads from reusing confirmed ownership.

New-block validity/congestion/stall evidence drives replacements. Missing fee windows use the bounded configured replacement interval. Stalls first re-estimate; they only grow gas when the raw estimate exceeds the signed limit, otherwise two exact rebroadcasts precede repricing. Unsupported next-block overrides fall back to latest-state estimates with wider configured headroom; revert/transport errors do not trigger that fallback. Gas-headroom multiplication now avoids uint64 intermediate overflow.

Execute, Resume and cancellation drain call one observer. It checks every owned variant, so an older original can win after replacements. The observer uses `chainio.VerifyReceipt` for the bounded canonical inclusion/descendant witness and `chainio.ReceiptExecution` for log identity/cloning; it contains no second ancestry implementation. Configured confirmations are supplied to that verifier. Reverted receipts also require canonical confirmation and durable evidence. Missing/invalid/orphaned receipts, foreign nonce consumption and deadlines remain pending ownership.

Receipt reads and mined-nonce reads can straddle mining or lag separately. Observed higher mined nonce stops all mutation but continues bounded observation, allowing the owned receipt to become visible rather than prematurely returning a foreign-nonce result. Unknown/foreign execution never becomes success.

Process cancellation stops mutation and grants observation alone a bounded shutdown drain. Read, estimate, broadcast, pending tracking and drain all have explicit budgets.

## Explicit host configuration

`Config` and `HorizonConfig` have lowerCamel YAML tags. Production contains no hidden defaulting. The host supplies the approved YAML defaults and calls `Validate`/`New` for sending, or `ValidatePolicy` for simulation. Validation covers finite/ordered tips and cap, positive durations, bounded confirmation/history/horizon counts, percentile and gas-headroom ordering. `ValidatePolicy` does not invent a spending ceiling for simulation.

Config fields: MaxFeeGwei, Confirmations, PollInterval, BroadcastTimeout, ReplacementInterval, PendingTimeout, ShutdownTimeout, ReadTimeout, EstimateTimeout, Horizon.

Horizon fields: MaxBlocks, BlockTime, TipFloorGwei, FullBlockTipGwei, CongestedTipFloorGwei, CongestedTipCapGwei, CongestedRewardBlocks, CongestedRewardPercentile, EscalateAfterFullBlocks, StallAfterBlocks, GasHeadroomBps, FallbackGasHeadroomBps.

## Evidence

Tests/interface were written before the driver implementation. The initial test run failed on missing runtime/driver definitions. Additional assertion failures were observed before fixes for a signer rewriting its input in place, initial estimate/mined-nonce race, terminal-recovery reorg release, gas arithmetic overflow, owned-receipt/mined-nonce observation race, and unnecessary stalled-call gas replacement.

Latest completed focused checks after all code/config changes:

- `go test -race -cover ./internal/transaction -count=1`: PASS, 85.9% statement coverage (after independent-review fixes).
- `go test -race -tags=integration ./internal/transaction -run TestAnvil -count=1 -v`: PASS, fresh and observation-only restart subtests. Each starts an ephemeral local Anvil, verifies persisted exact wire bytes before sending, rejects completion before two descendants, verifies canonical receipt/evidence persistence, and confirms restart performs no rebroadcast/signing.
- `go vet ./internal/transaction`: PASS.
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run ./internal/transaction/...`: PASS, 0 issues.

Failure-injection fixtures cover original/replacement/evidence fsync failure, unsigned admitted restart, ambiguous exact-byte retry, same-nonce immutable replacement, older variant success, fee ceiling, unreadable fee window fallback, gas override support/fallback/revert/transport distinctions, short depth/orphaned/removed-log receipts, foreign nonce retention, bounded hanging estimate and cancellation drain, concurrent reservation/observer exclusion, and malformed explicit policy.

A full `go test ./...` was also attempted during concurrent implementation. Every other package passed; `cmd/merkle-executor` failed compilation because its old automatic-provider factory expected provider.Provider while factories now return provider.Integration, and `internal/config` failed while its new Runtime/Reads/Transaction fields were being written. These are parent wiring work, not modified here.

A subsequent repeated-race attempt encountered concurrent `sdk/runtime/confirmation.go` build failures at lines 9 and 12: recipes.Reader lacked HeaderByHash required by chainio.SnapshotReader. No Task3 code had changed after its passing focused checks. Parent must rerun repository gates after wiring settles.

## Remaining integration work / limits

- Parent owns service wiring, complete YAML defaults, shutdown/storage ordering, legacy import and removing old managers in Task 6. Add the new transaction package to `make integration` before final gates.
- Parent final `make check` and `make integration` are still required after all parallel tasks land; this task does not claim those repository gates passed.
- Shared canonical closing/ancestry/reorg fixture coverage belongs to Task 1; the driver consumes that verifier and tests its own fail-closed integration.
- This is local fixture and local Anvil evidence only. No live API, production deployment, external chain or live-key evidence exists.


## Independent-review remediation

The two findings in `task-3-independent-review.md` were verified against the implementation. All three supplied reproductions were ported into permanent `nonce_policy_test.go` tests and first produced the reported failing assertions in the integrated checkout (no overlay required).

- A closing mined-nonce check can no longer lose its stop-mutation transition when a subsequent endpoint response lags. `track` permanently drops active mutation state on any `ErrNonceConsumed` returned by `advance`; later observation errors preserve that consumed-nonce classification. The driver continues observing and retains ownership.
- One shared final nonce guard now closes every active mutation path after preparation: replacement signing, ambiguous exact retry, stalled exact retry after its estimate, and fee-capped exact retry. No signing/rebroadcast resumes after a guard has observed consumption.
- Signed recovery validates structure, signature, sender, chain, nonce, destination, zero value and calldata without comparing historical signed bytes to the current spending ceiling. New signatures still enforce the current ceiling before persistence; an additional regression checks that distinction. A canonically confirmed attempt signed at 18.04 Gwei now resumes successfully under a valid 1-Gwei current ceiling.
- Removed the unused `derived` import from `internal/providers/cowswap/automatic_test.go` at the parent’s explicit request; no provider behavior changed.

Post-fix completed verification:

- `go test -race -cover ./internal/transaction -count=1`: PASS, 85.9% statement coverage.
- `go test -race ./internal/transaction -run 'TestClosingNonceConsumption|TestStalledRebroadcastCloses' -count=10`: PASS.
- `go test -race -tags=integration ./internal/transaction -run TestAnvil -count=1 -v`: PASS, fresh/restart.
- `go vet ./internal/transaction`: PASS.
- Scoped golangci-lint: PASS, 0 issues.
- `go test ./internal/providers/cowswap`: PASS.

No new live API, external-chain, deployment or credential evidence. Repository-wide final gates remain parent-owned after integration.
