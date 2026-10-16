# Generic recipe execution

The user approved moving on-chain readiness, amount selection, authorized calldata and completion checks into authenticated recipes, and requested coordinated pull requests to adapters, the frontend, merkle-executor and, if needed, vault-solver.

## Execution boundary

Keep all twelve existing opcodes and the deployed executor/account ABI unchanged. The executor discovers the exact committed queries/steps, verifies current permission bytes and caller, derives only authorized numerical inputs, assembles masked runtime calldata, and simulates the full original execute call. A successful canonical transaction proves its committed assertions; it does not prove later off-chain settlement. Requests and later claims remain separate programs.

Add generic work classification from the successful original execute return values, never from arbitrary argument values or function selectors. An active nonempty Manage group is work. Every unconditional Call and Approve is work, including Approve(0). A recipe containing only reads/checks or zero-gated Manage groups is idle. Validate structure, result-vector length and operand references before classification. Failed simulations, missing reads, invalid economics and malformed runtime remain errors. Check work after full canonical preparation, and again before new unsigned submission. Exact signed recovery must not be replanned or dropped because current readiness changed.

The daemon stops registering automatic redemptions and securitize providers. Its providers remain for external material (CoW orders, Merkl proofs, Liquid Lane quotes/signatures) plus generic recipe inputs. Existing issuer-specific packages may remain as explicit-host and fixture code, but are not imported into normal daemon startup. Use the single pre-production runtime policy address-only-v1 in both hosts. Do not add version compatibility or migration paths. Unresolved journals continue to fence ownership; never rewrite saved jobs.

## Conditional claim authoring

Claims have one conditional implementation. Every per-active-group cap, exact readiness debit, fresh output and minimum-rate assertion remains, while all groups may be zero-gated. Remove the final overall progress assertion and unused overall progress aggregation. The generic keeper idles after successful simulation. A manual on-chain invocation of an all-empty program can succeed without useful work. Never translate CheckFailed into idle.

Adapters: ClaimRecipe.build(account, claims) is the single entry point. There is no buildConditional method or idleWhenEmpty JSON flag. Regenerate every affected claim artifact and manifest; no pre-production bytes require preservation.

Curator: claims() uses the same single implementation without a compatibility flag. Every recipe template uses version 1; no byte-version catalogue, older compiler selection or migration behavior is supported before production. User-facing copy explains automatic waiting. Test actual current contracts for all-empty success, one ready group, required read failures, exact debit, fresh output and rollback. No new low-level opcode UI.

## SDK delivery and vault-solver

Publish the reviewed SDK source as a commit on the feature branch, then pin its immutable Go pseudo-version in the daemon and vault-solver companion branch. No production replace directive or implicit go.work. The vault host keeps keeper.NewRecipeInputs and its shared sender; add unknown-protocol action and idle fixtures at the consuming boundary. Keep its provider count and durable nonce ownership. Its sole runtime policy is address-only-v1, with no pre-production migration support.

## Repository boundaries and validation

PRs stack on existing open Merkle branches: merkle-executor codex/improve-all, adapters codex/bailsec-normalized, curator-frontend NO-TASK-merkle-adapter, vault-solver codex/merkle-sdk-integration. Primary dirty work is preserved. RFQ frontend has no Merkle recipe consumer, so no cosmetic PR is planned.

Run make check and make integration for both executor modules; verify lens/generated artifacts where changed. Run relevant Foundry recipe suites, manifests, format and unchanged executor/capability ABI checks in adapters. Curator must pass its exact-branch Node/pnpm full check and bundle budgets. Vault must pass format, full race/coverage, lint, build and local Anvil with GOWORK=off. Report local/fixture, hosted CI and deployment evidence separately. No live signing, merge or deployment.

Implementation choices above are engineering decisions within the user's approved direction. Preserve permission masks, proofs, targets, economic checks, nonce ownership, durable external identity, cancellation behavior and zero-valued nonfinancial identities.
