# Transaction sending

`internal/transaction.Driver` implements `runtime.Driver.Execute(ctx, Intent)`.
It is a single in-process sender, with no disk journal, replacement engine,
rebroadcast loop, persistence callback or restart recovery API.

The sender reads the write endpoint's mined nonce, estimates gas on the read
endpoint, adds configured headroom and signs one EIP-1559 transaction. Maximum fee,
priority fee, headroom, confirmation depth and I/O timings are explicit YAML under
`transaction`; defaults are in `internal/config/defaults.yaml`. It rejects a fee
ceiling below the observed base fee plus priority fee. Before broadcasting it
checks the complete signed payload and sender, and rechecks the mined nonce.

Broadcast is attempted once. A send error can mean the endpoint accepted the
transaction, so the sender still observes the same hash. Canonical confirmation
proves ancestry, descendant depth and log identity using fresh witnesses. A valid
receipt releases the in-process lane; a revert halts only its strategy. Errors
before broadcast permit fresh preparation. Missing receipts, consumed nonces and
post-broadcast cancellation never prove a safe new submission. There is no fee
escalation: a transaction can remain pending indefinitely at its configured fees.

The runtime owns the worker context. Tick cancellation stops waiting without
starting another sender. Shutdown cancels and drains the worker before closing
RPC. A stopped ambiguous operation remains fenced if the same sender instance is
reused. Runtime outcome metrics and logs omit signed bytes and remote payloads.

All ownership and execution evidence lives in RAM. Process exit loses it, including
unknown pending transactions and external orders. Restart begins new discovery
without reconciling previous work. Use one process per dedicated signer; there is
no file lock or coordination between independent processes.
