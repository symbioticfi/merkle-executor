# Native Claims RAM SDK Implementation Plan

> **For agentic workers:** Use subagent-driven development for the disjoint provider, ABI and host tasks. Steps use checkboxes for tracking.

**Goal:** Preserve PR #36 native claim discovery on the current RAM SDK and update its architecture guidance.

**Architecture:** Keep native discovery in one isolated stateless provider. Return plain authorized action outputs and reuse generic admission, batching and RAM transaction ownership.

**Tech Stack:** Go 1.27.3, abigen v2, local Anvil.

**Spec:** `docs/superpowers/specs/2026-11-08-native-claims-ram.md`.

## Global constraints

- No replay envelopes, restart recovery, SDK workers or live test credentials.
- Never mutate recipe instructions, targets, selectors, masks or proofs.
- Preserve fixed IDs, recipients and full original-code simulation.
- Keep generic packages free of protocol-name branches.

## Review focus

- Claimed, immature, zero-entitlement and zero-capacity entries remain idle.
- Fixed permission bytes cannot be widened by discovery.
- Malformed reads and RPC errors must not become successful empty outputs.
- Synchronous vault operations remain outside claim discovery.
- Multiple ready claims reach generic admission as independent executions.

### Task 1: Port the stateless provider

Files: `sdk/providers/claims/provider.go`, `provider_test.go`.

- [x] Observe inherited API failures and add regressions for plain action outputs with nil tasks.
- [x] Replace removed derived replay helpers and advancement with plain `provider.Output` values.
- [x] Preserve discovery, zero/idle behavior, permission masks and pinned preflight checks.
- [x] Run `GOTOOLCHAIN=go1.27.3 GOWORK=off go test -race ./providers/claims` from `sdk`.

### Task 2: Generate the claim binding

Files: `sdk/abi/NativeClaims.json`, `sdk/abi/native_claims.go`, `sdk/bindings/nativeclaims`, `sdk/hack/generate-bindings.sh`, ABI/schema provenance.

- [x] Verify the selected interface subset and record source revision and limitations.
- [x] Add `NativeClaims:nativeclaims` to pinned abigen v2 generation; remove the raw embedded-ABI shim.
- [x] Supply `nativeclaims.NewNativeClaims().GetABI()` to task 1 and verify generation is reproducible.

### Task 3: Host integration and architecture

Files: `cmd/merkle-executor/run.go`, native claim integration fixture/test, `AGENTS.md`, README/SDK/provider/architecture documentation.

- [x] Register the provider alongside the existing automatic providers.
- [x] Add a local Anvil regression proving ready claim argument selection and execution through the real planner; observe the missing-provider failure first.
- [x] Document native claims as a narrow provider exception while retaining recipe ownership elsewhere.
- [x] Keep the local SDK module replacement and current dependency baseline.

### Task 4: Review, verify and publish

- [x] Review the complete diff against current `stage` and fix actionable findings.
- [x] Run `make check`, `make integration` and `git diff --check`.
Publish the verified merge/port to `codex/native-claim-discovery` and update PR #36's description with the final scope and evidence.

## Verification record

- `make check`: passed for both modules, including race tests, vet, lint and generated lens checks.
- `make integration`: passed against ephemeral local Anvil and API fixtures.
- New host regression observed a missing-provider failure, then selected the native maximum and completed the real claim with the provider registered.
- Review found a synchronous foreign-recipient compatibility issue; its regression failed before the fix and passed afterward. The scoped re-review is clean.
- Claim binding regeneration is reproducible; issuer fixture generation checks and `git diff --check` pass.
- No external-chain execution, live signing credentials or deployment was used.
