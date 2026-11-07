# Transaction processing

`internal/transaction.Driver` implements the SDK `runtime.Driver` contract. One
runtime-owned worker and one durable workflow replace the former keeper, submission
journal, transaction manager and transaction engine lifecycle layers.

The driver reserves one signer lane. It reads the write endpoint's mined nonce,
ignores pending counts for allocation and retains a floor for canonically confirmed
owned nonces. Read RPC supplies fee history, estimation, receipts and headers; write
RPC supplies nonce admission and broadcast. Both HTTP clients refuse redirects.

Before broadcasting, the driver validates the signer result against the complete
quoted transaction, including chain, nonce, destination, value, calldata, gas and fees,
and calls the workflow persistence callback with the exact signed wire bytes. An
uncertain write blocks further admission and release until restart/reload. Signed
originals and replacements remain candidates for confirmation. An older variant may
win. Ambiguous sends can rebroadcast identical bytes; they never allocate a new nonce.

Fee policy uses exact EIP-1559 base-fee growth, recent block fullness and bounded tips.
Gas estimation targets the next block; unsupported overrides use latest-state
estimation with explicit fallback headroom. Congestion, new-block validity and stall
evidence can trigger same-nonce replacements. The driver never sends cancellation
self-transfers. All limits, timings, horizon and fee values live in YAML under
`transaction`; defaults are in `internal/config/defaults.yaml`. Send mode requires a
finite positive `transaction.maxFeeGwei` ceiling.

Receipt confirmation proves canonical ancestry and descendant depth, closes with
fresh numbered witnesses and validates log identity. Terminal evidence is persisted
before the runtime durably changes phase and releases ownership. External settlement
uses that exact transaction evidence. A reverted recipe halts its workflow for
inspection. Missing receipts, foreign nonce consumption, timeouts and cancellation
retain the reservation because a generic recipe may execute more than once.

Restored signed workflows are observation-only: no signing, rebroadcast or replay of
saved calldata. Definitely unsigned admission may refresh only after its safe state
is persisted and the lane released. Process-owned observation survives a caller Tick
timeout. Shutdown cancels/drains this worker before releasing RPC or state resources;
a drain failure leaves ownership intact.

RPC errors preserve typed causes for cancellation/revert handling but redact endpoint
credentials and unbounded remote text from logs. Runtime outcome metrics use bounded
strategy/outcome labels. No signing credential is used by local tests.
