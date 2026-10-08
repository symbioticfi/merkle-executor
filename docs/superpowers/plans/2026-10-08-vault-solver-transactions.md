# Vault-solver transaction processing for Merkle executor

Goal: use the transaction engine from vault-solver stage cc75494298aa3c9b4a17021564d569b811a8fd42, targeting Merkle executor main.

Architecture: vendor the integration-neutral engine and its regression suite, with import-path substitutions and canonical gofmt import ordering only. Keep a small Merkle adapter for durable recipe identity, persist-before-broadcast signed attempts, canonical execution evidence, and acknowledgement after durable keeper progress. The engine owns fee selection, mined nonce admission, gas estimation, same-nonce replacements, exact uncertain rebroadcasts, receipt scheduling, bounded tracking and graceful drain. The adapter retains uncertain recipe work after engine abandonment; a generic repeatable recipe has no order-level idempotency proof, so neither timeout nor restart permits automatic replay.

Tech stack: Go 1.27.1, geth 1.17.5, strict YAML, existing atomic state store, Prometheus, OpenTelemetry API.

Spec: user request to make merkle-executor use the same transaction processing as vault-solver; user selected main as PR base.

Global constraints: preserve the dirty primary checkout; no protocol branches in the engine; no secret values in config/logs; no transaction may reach the network before persistence; preserve provider execution logs and job-before-ack ordering; no cancellation self-transfers; use ephemeral local chains for execution tests.

Review focus: engine source parity, concurrent cancellation and persistence, ambiguous first/replacement broadcasts, older-attempt receipts, restart recovery, fee bounds, RPC error classification/redaction and keeper settlement evidence.

- [x] Vendor the engine, record immutable provenance and verify its source parity; run upstream regression tests.
- [x] Integrate durable signing and asynchronous ownership with keeper acknowledgement; add regression tests for replacements, cancellation and restart.
- [x] Adopt horizon fee configuration and RPC next-block estimation; test configuration and endpoint/error boundaries.
- [x] Update operator/design docs, run make check and make integration, and obtain a fresh review.

Delivery: commit and push the validated feature branch, then open and attach a main-targeted PR. No merge or deployment is included.

Ruling: use the latest remote stage rather than the stale local vault-solver checkout; current stage intentionally removed cancellation transactions and durable transaction storage. Merkle retains its existing business journal because its recipes can be repeatable and provider settlement requires receipt evidence.


Verification: baseline scoped tests passed. The new restart regression first failed because recovery rebroadcast saved calldata; it now passes. RPC classification/next-block estimation tests first failed before integration and now pass. Review findings were reproduced and fixed: refreshed canonical receipt evidence, redacted balance telemetry, finite fee-history percentile and gofmt-compatible provenance. Unsigned-admission acknowledgement and error-on-earlier-hash recovery have regressions. Final make check (build, race/coverage suite, vet, lint) and make integration passed. The primary checkout's tracked patch and untracked hashes were verified unchanged.
