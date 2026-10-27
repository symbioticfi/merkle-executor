# Transaction processing

The SDK exposes `txengine` for transaction mechanics and `txmanager` for durable
recipe ownership. Hosts explicitly construct and start these components, supplying
RPC connections, signing credentials and telemetry. Logging/tracing helpers live
in `sdk/telemetry`; process setup stays in `internal/observability`. Transaction
metrics retain the namespace `merkle_executor_txmanager_*` with bounded operation
labels. A host with an existing sender can implement `submission.Transport`
instead; vault-solver uses its shared sender rather than starting another engine.

The engine owns one signed lifecycle at a time. It reads the write endpoint's mined nonce before preparation, retains that nonce through estimation/signing, and ignores pending counts for selection. It keeps a floor for canonically confirmed owned nonces. Fees use the next base fee, its exact EIP-1559 growth bound and recent block fullness. Gas is estimated against the next block; unsupported block overrides use latest-state estimates with larger headroom. New-block validity, congestion or stall evidence triggers same-nonce replacements; unavailable fee windows use bounded fallback bumps. All variants remain eligible for receipt discovery. Ambiguous sends get an exact-byte rebroadcast, with no new nonce. Pending timeout and shutdown drain are bounded. The engine never sends cancellation self-transfers.

Merkle's adapter adds business durability. The keeper persists the job before admission; the signing wrapper validates sender, chain, destination, calldata, value and signature payload, and fsyncs every signed attempt before returning it to the broadcaster. A persistence error blocks further signing and acknowledgement. Caller cancellation returns pending work while the engine continues on its process context. The next keeper tick joins that lifecycle, rather than creating another one. Even unsigned admitted work retains its job identity until a caller joins the engine result. Initial gas estimation has one budget for next-block estimation and its latest-state fallback: the smaller of five seconds and half the replacement interval, further bounded by an earlier request or process deadline. Expiry before signing returns an unsigned failure even when the HTTP attempt timeout allows longer; identity is released only when that outcome is joined. This budget does not bound canonical settlement or authorize retry of signed or ambiguous work.

The journal stores one identity and same-nonce variants. Terminal success/revert is persisted only after the engine's canonical ancestry and descendant-depth proof. The submission layer validates receipt logs before persisting execution evidence; the generic engine validates transaction/block identity and status. The initial inclusion proof has three receipt-read budgets for its three sequential calls, capped by the final abandonment grace. Inclusion uses the receipt block as a numbered witness; confirmation depth uses the descendant at receipt block plus required depth. The engine walks exact parent hashes from that witness, rereads the receipt, and then verifies the same numbered witness again. Normal latest-head growth does not invalidate this proof, and ancestry work depends on required depth rather than receipt age. An older variant's receipt can win. The result carries its exact hash, block and logs; providers use that evidence to reconcile their prepared calls. Keeper progress is persisted before acknowledgement clears the journal. A restored terminal journal also restores the owned-nonce floor.

A generic Merkle recipe can be repeatable and has no universal order ID/idempotency check. Merkle retains its unresolved journal when the engine abandons tracking or observes foreign nonce consumption. Restart checks every owned hash without replaying stored calldata. Neither a missing receipt, an RPC error nor a pending nonce is proof that the recipe failed. This intentional business-layer constraint prevents duplicate recipe execution and unauthorized external settlement.

`txManager.maxFeeGwei` is the only transaction configuration field. It sets a
finite positive fee ceiling in send mode. Timing and fee-horizon settings are
fixed by runtime policy version `1`: 2 confirmations, 2-second receipt polling,
5-second broadcast timeout, 30-second account telemetry and replacement intervals,
5-minute pending timeout, and 1-minute shutdown drain.

The fixed fee horizon spans 6 blocks at 12 seconds each. Tip floors are 0.02 Gwei
normally, 0.1 for a full block, and 0.2 under congestion, capped at 15 Gwei. The
reward window is 3 blocks at percentile 50; escalation starts after 2 full blocks
or 3 stalled blocks. Gas headroom is 500 basis points, or 1000 for the estimate
fallback. The engine checks that these values fit the configured global fee cap.

Both read and write HTTP RPC clients refuse redirects, including transaction
submission redirects, so signed payloads stay on the configured endpoint.

Read RPC handles simulation, fee history, receipts and confirmation headers. Write RPC handles nonce admission and broadcast. RPC error text is redacted but preserves a finite allowlist of nonce, fee/rejection and execution-revert classifications; original typed causes remain available for invalid-params fallback and revert decoding. No endpoint credentials or provider response text appear in lifecycle logs.

The engine has no solver identity or protocol-obsolescence callback. Receipt confirmation and restoring the confirmed nonce floor are ordinary engine APIs. Merkle owns recovery after tracking ends: it polls stored variants directly, without a second passive observer, hash cache, deduplication ledger or receipt callback. Engine gas/fee metrics account receipts found during active tracking; recovered terminal receipts are reported by journal logs and keeper outcomes, without passive-observer cost counters.
