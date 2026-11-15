# External execution simulation implementation plan

**Goal:** Remove RFQ's unused nested-return probe while retaining complete shared admission and unchanged outer settlement simulation.

**Architecture:** The shared recipe admission core authenticates immutable recipe/call identities, fresh metadata and roles, and original/fresh block pins. Generic execution keeps grouped simulation plus numerical witness and readiness validation. An explicit external-simulation API requires a host callback to simulate the exact selected calls inside their complete external transaction at the supplied fresh block and gas budget; that host owns external usefulness and settlement checks. Neither path fabricates return values or publishes cache evidence before successful admission.

**Tech stack:** Go 1.27.3, existing SDK and host chain readers, local Anvil 1.9.0.

**Constraints:** RAM only; no protocol branch or skip-validation flag in the SDK. No contract, permission, calldata, amount-binding or settlement change. Keep all generic numerical/result checks. RFQ uses ordinary unchanged-code `eth_call` from the real signer; code overrides are unnecessary for this integration.

**Review focus:** Empty outer returns must succeed without fake inner results. Invalid roles/identity or original/fresh reorgs must fail. External callback errors must prevent cache publication. RFQ must preserve exact envelope, signer, caller, gas and block. Generic paths must still reject invalid witnesses and inactive execution.

- [x] Add the required callback/request API through the existing admission core; cover callback binding, failure, reorgs and generic validation regressions.
- [x] Migrate vault-solver RFQ to one complete outer simulation. Cover ordinary readers without code overrides, unchanged call context, empty returns and every canonical contract rejection fixture.
- [x] Delete the obsolete probe, generated artifact, generator, probe-only test machinery and verification target. Update SDK/host architecture and operator documentation.
- [x] Run SDK/daemon `make check` and `make integration` with Anvil 1.9.0 and independent source review. Focused host RFQ unit/race and all seven canonical-contract cases pass.

Publication uses existing PR #39. The vault-solver plan tracks its final SDK pin, repository-wide host gates and PR #277 update.

The source audit confirms RFQ supplies no numerical witness. Its probe was required only by generic result-based readiness, not an RFQ numerical comparison. Full RFQ settlement remains the external execution success condition.
