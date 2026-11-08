# Plan 002: Read only Merkl runtime-token candidates that are visited

Planned at `fd35d77`, 2026-11-08. P2; effort M; fix risk medium; no dependencies.

Status: DONE. Composed claims lazily load visited candidates while standalone inventories retain grouped reads. The 128-candidate regression uses four getters for a first-positive claim. Focused race tests and both full verification gates passed.

## Problem and current state

`sdk/providers/merkl/reads.go:42-47` expands a runtime-token claim into every API reward. Lines 81-96 submit all aggregate chunks. `sdk/protocolio/batch.go:71-85` executes every requested chunk eagerly for the daemon's BatchReader. However, `sdk/providers/merkl/automatic.go:213-230` selects only the first positive candidate in composed recipes. With 128 rewards this does 258 getters instead of four when the first candidate is claimable.

## Scope and design

Own only `sdk/providers/merkl` and `docs/PROVIDERS.md`. Do not change generic batching or other providers. Keep pinned reads, shared root/default-recipient results, deduplication, exact selection order, and deferred consumed-getter failures. Standalone `prepareAll` needs the complete inventory and may keep efficient aggregate grouping. For first-positive composed selection, load only needed candidates or small bounded lookahead; prefer lazy exact-needed reads over a new tuning knob.

Match existing table-driven Go 1.27.3 tests in `reads_test.go` and `automatic_test.go`. All remote data remain untrusted; do not change proofs, recipient validation, permission masks or recipe bytes.

## Execution

1. Add a failing composed-recipe test with at least 128 valid reward candidates and a first-positive candidate using a BatchReader, asserting bounded actual getter/request counts and unchanged selected output. Observe its failure before implementation.
2. Implement the smallest reusable lazy read/cache path. Add coverage for later candidates, reused getters, and later failed getters not masking an earlier selected claim. Retain all-inventory dedup/chunk tests.
3. Update docs to accurately describe strict consumed getter validation and grouping. Run `GOTOOLCHAIN=go1.27.3 GOWORK=off go test -race ./providers/merkl` from `sdk`; expected exit 0. Coordinator runs both full gates.

## Done and boundaries

The first-positive regression performs no work proportional to the unused inventory; existing claim and proof tests pass. No dependency or configuration additions. Stop and report if lazy planning cannot preserve selection order or error semantics. Future batching changes must exercise both BatchReader and lazy single-reader backends.
