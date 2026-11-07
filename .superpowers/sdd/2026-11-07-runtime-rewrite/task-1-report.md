# Task 1 report: neutral chain I/O and shared scheduler

Completed in the isolated checkout `/Users/andreikorokhov/.codex/worktrees/merkle-runtime-rewrite/merkle-executor`. Original checkout was not edited. No live APIs, chain endpoints, keys, signing, pushes or deployments were used.

## Implementation

- Added independently consumable `sdk/chainio`: neutral `Reader`, `CodeOverrideReader`, `LogReader`, `HeaderReader`, `CanonicalReader`, `Snapshot`, `Execution`, `PreparationPin`, `ReadAccessor`, heterogeneous reads/results, bounded RPC array transport and aggregate codec interfaces.
- Moved the existing read batching and strict bounded JSON-RPC parsing into `chainio`. `protocolio` temporarily aliases neutral types/transport and delegates canonical evidence helpers; protocol HTTP helpers remain there. Host `internal/chain` delegates array parsing and exact override calls to that same transport, removing its duplicate parser.
- `ReadRequest.CodeOverride` is part of exact isolated `eth_call` arguments and immutable request identity; overrides never aggregate. Complete call contexts retain sender, target, fees, value, gas, access lists, blobs and authorizations. Header-by-hash requests/facade support dependent ancestry checks.
- Added explicit `Limits` with strict camelCase YAML tags: max members, encoded request/response bytes, calldata, gas, output, concurrent workers, outstanding request count/bytes and collection window. `DefaultCallGas` and `DefaultOutputBytes` are mandatory host policy rather than hidden runtime defaults; `ExpectedGas`/`ExpectedOutputBytes` declare per-read estimates without changing call semantics. Explicit call gas cannot be undercharged by an estimate.
- Scheduler fairly interleaves currently ready owners and alternates existing/newly ready waves. A configured short collection window collects available stages; slow providers need not finish before unrelated work proceeds. Worker count is bounded. Accepted outstanding work charges request context overhead plus reserved result bytes, and cancellation does not cancel another owner's shared read. New owners progress behind a large in-flight stage.
- Deduplication is collection-wave scoped, includes chain identity, exact pin/tag, complete call/filter, override bytes and execution class, and never caches across waves. `latest`/`pending` requests and mutable receipts are not deduplicated. Results are deeply cloned per owner. Header/log/read arrays and returned bytes do not share mutable state across owners.
- Partitions enforce member, exact isolated wire/conservative aggregate ABI wire, calldata, gas and expected-output budgets before dispatch. Actual responses/output and per-owner reserved outputs are checked before consumers receive results. Malformed/failed groups produce no single-read fallback.
- Added one bounded canonical ancestry/witness implementation shared by `VerifyReceipt`, `ConfirmedSnapshot`, and `VerifyConfirmedSnapshot`. Receipt verification preserves the captured inclusion+depth witness, exact hash-linked parents, fresh receipt identity/log validation, and fresh numbered witness closure. It does not chase a moving head. `ReceiptExecution`, `ConfirmReceipt`, `Canonical`, and `VerifyExecution` expose neutral evidence helpers.
- Removed all `recipes` imports of `provider` and `protocolio`, including internal-package tests, so `provider -> recipes` is acyclic. `recipes.PreparationOptions.Pin *types.Header` accepts a cloned common opening head while keeping latest presend observations and closing guards fresh. Recipe cache lookup/replacement/discard now use a map index, preserving FIFO links, frozen version checks, exact bytes, conservative byte charges and publication after closing guards.

## API coordination

Root and integration agent were notified immediately when neutral types and dependency direction were available. Driver agent uses `chainio.VerifyReceipt(ctx, ReceiptReader, *types.Receipt, uint64) (*types.Receipt,error)` and can use `ReceiptExecution` for successful or reverted receipts. Nil verified receipt means not sufficiently deep/currently canonical.

Root planner delegates to:

```go
chainio.ConfirmedSnapshot(ctx, chainio.SnapshotReader, chainio.Snapshot, depth) (chainio.Snapshot, error)
chainio.VerifyConfirmedSnapshot(ctx, chainio.SnapshotReader, chainio.Snapshot, depth) error
```

`SnapshotReader` embeds `Reader` and `CanonicalReader`; `ReceiptReader` embeds `CanonicalReader` and `TransactionReceipt`.

Scheduler API: `NewScheduler(BatchReader, Limits) (*Scheduler,error)`, `Run(ctx) error`, `Close() error`, `BatchRead`, and exact call/header/hash/log/receipt/override facades. Root supplies every limit from YAML. Suggested initial host defaults communicated: default call gas 1,000,000, output 65,536 bytes, aggregate gas 30,000,000; these are not baked into scheduler production code.

## Test-first evidence

The scheduler test suite was written before implementation and first failed on missing types/scheduler. Additional tests were observed failing for starvation of a newly ready owner, unbounded actual output, hexadecimal override wire size undercharging, and outputs exceeding reservations; implementations were then corrected. Canonical receipt and snapshot tests first failed on missing exported helpers. A later shared-pin fixture initially returned invalid one-byte calldata; it was corrected to the existing valid execution fixture, after which the regression passed.

Tests cover 1/16/64/256/1024 concurrent owners, numbered-pin dedup, caller/override/chain inequality, no latest/pending dedup, cloned results, byte/count/calldata/gas/output partitions, queue admission/release, cancellation retaining another owner, malformed result cardinality without fallback, fresh closing observations and new-owner fairness. Existing RPC malformed/oversized/duplicate/missing-ID and member-error tests moved with the transport. The override fixture checks complete context against the common encoder. Existing recipe cache/reorg/permission/simulation regression coverage remains.

## Verification

Passed locally:

- `go -C sdk test ./chainio ./recipes ./protocolio`
- `go -C sdk test -race ./chainio ./recipes ./protocolio`
- `go test -race ./internal/chain` (root `replace .../sdk => ./sdk` is active)
- `go -C sdk test ./...`
- `go -C sdk vet ./chainio ./recipes ./protocolio`
- `go -C sdk run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run --config .golangci.yml ./chainio/... ./recipes/... ./protocolio/...` — 0 issues after fixing error identity comparisons.
- `git diff --check`

One full root `go test ./...` during the concurrent rewrite failed only to build `cmd/merkle-executor`: `run.go:95` still expected legacy `Provider` automatic constructors while the integration agent had changed them to `Integration`. Other root packages shown in that run passed, including `internal/chain`, provider packages and the new transaction package. Root was notified; CLI/service porting belongs to root. Do not treat this intermediate root run as a completed final gate.

These are local unit/race and HTTP API-fixture tests. No Anvil integration or deployment evidence is claimed here. Root owns final whole-rewrite `make check`, `make integration`, all-module/lens/container gates and automatic daemon benchmarks.

## Limits and remaining integration work

- Resource budgets are explicit declarations/host conservative estimates. The scheduler preserves zero gas exactly rather than changing EVM execution context; gas consumed by a zero-gas `eth_call` cannot be inferred from returned bytes. Default estimates must remain conservative in host YAML.
- Queue admission returns a bounded explicit error when full; callers may retry a future eligible wave. Canceled owners keep their reservations until the bounded in-flight group resolves, so shared owners are unaffected.
- The scheduler's fairness and owner-count fixtures prove its local boundary. They do not by themselves prove the final automatic-provider daemon path, cold/warm protocol throughput, deployment or live behavior.
- Temporary protocolio/provider aliases and old submission/keeper implementations remain for root's final caller/reachability cleanup after porting. This task did not modify root runtime, service/config or provider integration files.

## Independent-review remediation (follow-up)

Addressed all three findings in `task-1-independent-review.md` after writing focused regression tests and observing each fail on the reviewed implementation:

1. **Abandoned/stalled partition cancellation.** Each dispatched partition now owns a context with mandatory host-configured `Limits.ReadTimeout` (`yaml:"readTimeout"`). Its context remains alive while any assigned owner remains live and is canceled when the last live owner cancels. Canceled pending partitions do not call the backend. Context watchers are stopped when transport returns; in-flight completion releases the existing queue reservation. This preserves another owner's read and also bounds deadline-free owners. Root added explicit `reads.readTimeout: 30s` in host YAML; no scheduler default is hidden in code.
2. **Raw response/request limits.** Introduced an explicit `ReadBounds` backend-context contract. Scheduler partition contexts carry configured request, raw response and calldata caps; backends preserve that context through transport. The shared HTTP reader now reads at most configured raw response bytes plus the one-byte overflow sentinel, under the existing hard 32-MiB ceiling. It no longer substitutes `ReadResult` base64 remarshal size for hexadecimal RPC wire size. Exact encoded request bodies are checked both before codec dispatch and in HTTP. Decoded output/per-owner reservations retain their separate checks. Partition estimates charge hexadecimal expansion, JSON envelope and conservative aggregate return framing, and dedup retains the largest owner's wire reservation.
3. **Aggregate calldata limits.** Scheduler partitions conservatively charge aggregate ABI tuple/header/padding for eligible safe calls. `RPCBatchReader` independently measures actual post-codec input and override code bytes before calling transport; this includes the generated host `aggregate3` payload. No failed-group single fallback was added.

New local tests prove: all canceled owners cancel a stalled backend and release capacity; pending canceled partitions never reach the backend; a finite configured I/O deadline also bounds a live deadline-free owner; oversized raw RPC error bodies fail before consumption; actual generated host Multicall calldata is rejected before HTTP dispatch; codec payloads are checked against exact calldata caps; and larger deduplicated response reservations still split partitions correctly. The earlier shared-live-owner cancellation test remains green. These are local context/HTTP/ABI fixtures only.

Follow-up verification passed:

- `go -C sdk test -race ./chainio ./recipes ./protocolio`
- `go test -race ./internal/chain`
- `go -C sdk test ./...`
- `go -C sdk run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run --config .golangci.yml ./chainio/...` — 0 issues.
- `git diff --check`

The backend contract is explicit: custom `BatchReader` implementations must preserve and honor `ReadBounds` in their wire transport. The provided RPC/HTTP implementation and host chain adapter enforce it. Conservative admission may reject an aggregate-eligible read under an exceptionally small budget even where an isolated call would fit; it never silently changes read class or retries after a failed group.

A follow-up full root `go test ./...` still failed on in-progress CLI test migration (`config.DefaultStrategyInterval`, `config.DefaultKeyEnv`, `config.TxManager` and `bindState` references). It also exposed the redemptions scheduler fixture missing new `ReadTimeout`; root authorized updating that explicit test literal, and its focused test passed afterward. These root CLI failures are outside the scheduler fix and were reported to root.
