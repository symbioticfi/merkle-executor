# Runtime throughput and isolation implementation plan

**Goal:** Isolate failed executor candidates, preserve useful batch capacity, and continue confirmed work without waiting for the next strategy timer.

**Design:** Keep the common pinned, grouped discovery core. The planner receives per-candidate failures and returns healthy plans with explicit diagnostics; strict discovery callers retain all-or-error semantics. After envelope rejection, validate descending configured-order prefixes so a valid intermediate size is considered without assuming arbitrary EVM success is monotone. Keeper drains immediately confirmed envelopes while submitting each executor and advancing each settlement at most once per tick. Idle candidates may be prepared again after earlier confirmed effects. An optional host sender notification wakes explicit host loops on canonical completion; pending work and errors yield to normal polling.

**Constraints:** RAM only; preserve every permission, identity, role, amount, witness, receipt and canonical-block check. Keep signed ownership after any attempted broadcast. No background SDK workers, new protocol branches, additional batching limits, or configuration knobs.

- [x] Add failing/healthy candidate and largest-prefix regressions; implement partial discovery, diagnostics and batch fitting.
- [x] Add fast-confirmation, pending, fairness and cancellation regressions; implement bounded keeper drain and optional wakeup access.
- [x] Notify after host result publication; wire generic, RFQ and 3F loops without retry spins. Test callback ordering and RFQ continuation before its cadence timer.
- [x] Update architecture/operator documentation and run SDK/daemon `make check` plus `make integration`.

Publish this SDK revision through PR #39 and pin it in vault-solver PR #277, whose host follow-up tracks repository-wide validation. Independent review resolved the exhausted-fit diagnostics case. Local Anvil also exposed an existing duplicate-broadcast response variant: `already imported` now retains its bounded classification through RPC redaction and follows the pending-owned path. CI's Anvil 1.9 exposed inclusion between a missing receipt/old nonce read and rebroadcast. A nonce-conflict response triggers another exact-owned-hash receipt check; only existing canonical confirmation can release ownership. Unproven failures still return errors and preserve ownership.

Retain signed ownership after an RPC attempt; a fresh pre-sign funding check rejects insufficient maximum-cost coverage before signing. Only unsigned failures release automatically; no new signed-attempt release or cancellation workflow is introduced. RFQ uses the explicit external-simulation admission API and unchanged complete outer settlement. It supplies no numerical witness and consumes no inner return data; the generic numerical and result-based readiness checks remain in the normal admission path.

## Funding admission follow-up

- [x] Add a fresh sender-balance check against final unsigned maximum cost before each signature. Keep existing ownership after every attempted broadcast.
- [x] Cover exact coverage, gas headroom, malformed/unavailable balances and no-sign/no-send/no-busy failures. Host checks also include transaction value and replacements without losing prior ownership.
- [x] Run SDK/daemon `make check` and `make integration` with Anvil 1.9.0; independent source review found no production blockers. The real host bridge funding/retry regression also passes ten race-enabled Anvil runs.

Publish the SDK through existing PR #39; the host plan tracks its final pin, repository-wide gates and PR #277 update. No auto cancellation or additional runtime state.
