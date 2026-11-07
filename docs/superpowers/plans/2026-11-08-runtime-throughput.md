# Runtime throughput and isolation implementation plan

**Goal:** Isolate failed executor candidates, preserve useful batch capacity, and continue confirmed work without waiting for the next strategy timer.

**Design:** Keep the common pinned, grouped discovery core. The planner receives per-candidate failures and returns healthy plans with explicit diagnostics; strict discovery callers retain all-or-error semantics. After envelope rejection, validate descending configured-order prefixes so a valid intermediate size is considered without assuming arbitrary EVM success is monotone. Keeper drains immediately confirmed envelopes while submitting each executor and advancing each settlement at most once per tick. Idle candidates may be prepared again after earlier confirmed effects. An optional host sender notification wakes explicit host loops on canonical completion; pending work and errors yield to normal polling.

**Constraints:** RAM only; preserve every permission, identity, role, amount, witness, receipt and canonical-block check. Keep signed ownership after any attempted broadcast. No background SDK workers, new protocol branches, additional batching limits, or configuration knobs.

- [x] Add failing/healthy candidate and largest-prefix regressions; implement partial discovery, diagnostics and batch fitting.
- [x] Add fast-confirmation, pending, fairness and cancellation regressions; implement bounded keeper drain and optional wakeup access.
- [x] Notify after host result publication; wire generic, RFQ and 3F loops without retry spins. Test callback ordering and RFQ continuation before its cadence timer.
- [x] Update architecture/operator documentation and run SDK/daemon `make check` plus `make integration`.

Publish this SDK revision through PR #39 and pin it in vault-solver PR #277, whose host follow-up tracks repository-wide validation. Independent review resolved the exhausted-fit diagnostics case. Local Anvil also exposed an existing duplicate-broadcast response variant: `already imported` now retains its bounded classification through RPC redaction and follows the pending-owned path. Non-duplicate failures still return errors and preserve ownership.

Rejection alternatives and the RFQ probe are explanations in this follow-up: retain signed ownership after an RPC attempt; pre-sign funding admission and a proven-never-broadcast boundary are separate possible improvements. Existing RFQ contracts discard inner results, so unchanged outer settlement and authenticated inner-result probing both remain required.
