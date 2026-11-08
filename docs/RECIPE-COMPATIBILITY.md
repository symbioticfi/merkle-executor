# Recipe execution boundaries

Recipes own native state reads, eligibility, financial policy and account-context
calls. A numerical cap is policy; it does not select an amount. Native amount
selection uses committed expressions or fixed literals, except for authorized
standalone claims filled by the isolated native claim provider. Off-chain witnesses may
supply signed economics or proofs when the owning provider recognizes and
validates their immutable permission shape. The complete original execution
simulation remains mandatory before signing.

## Authoring templates

| Template | Selection and enforcement |
| --- | --- |
| Lending | Available collateral up to its policy cap; fresh capacity after supply and total-debt headroom select borrowing. Exact spend, supply/debt rounding and issuer output checks remain in the recipe. |
| Claims | One unconditional authorized native call. The owning claim provider discovers ready requests or rewards and fills permitted arguments; native contracts enforce claim validity. Each claim is a separate execution. Fixed IDs remain fixed, and ID zero is valid. |
| Health recovery / health repay | Recipe reads select available liquidity, balances, debt and repayment; exact effect and health checks remain authoritative. |
| LL executing | Connector acquisition and calldata construction belong to vault-solver's Merkle extension. The RFQ backend supplies awarded trade amounts through `/orders` and signed terms for contract validation. The committed recipe enforces payment, recipient, RWA delta and approval-reset checks; the SDK validates permissions and simulates the complete execution. |
| 3F executing | The native selector reads a whole current authorization and checks its funding asset, actual collateral underlying, remaining policy budget and yield. The guarded mint rechecks authorization and policy. The library also supports externally selected terms, but the normal daemon requires an owning witness provider or native reads. |
| Signing | Existing CoW/RFQ providers acquire external quotes for recognized, pinned authorization shapes. Browser authoring requires verified matching read descriptors. A generic arbitrary signature buffer does not create a provider. |
| Custom | Unchanged masked preimages come from the trusted curator artifact. Runtime Inputs need an exact recipe value or an owning provider. |

## Claim discovery and batching

The native claim provider recognizes one unconditional managed call with no
recipe queries or bindings. It discovers ready Noon IDs and Pareto queue epochs,
checks direct owner entitlements, and fills ERC-7540 claim amounts from native
maximum getters at the pinned block. It preserves selectors, targets, recipients,
masks and proofs. It neither inserts recipe checks nor changes claim instructions.

The SDK admits and batches the separate ready-claim outputs through its common
validator. Gas-limited prefixes leave remaining claims eligible for later runs.
A successful transaction establishes execution of the selected claims, not that
an unconfigured issuer or reward token has been exhausted. Rich native recipes
with committed reads, exact bindings and guards remain supported by the generic
executor and are not rewritten or matched by this standalone provider.

Configured 3F `burnAll` groups use native withdrawal availability and full PT/YT
conversion as their asset payout gate/floor. Each target needs a corresponding
burn permission. Native mint selection does not authorize future burn targets or
prove a complete inventory of them; zero-value conversions are idle.

Native claim discovery scans owner ledgers within the host operation deadline;
required getter failures and cancellation fail preparation instead of silently
truncating the scan. No separate native claim-selection helper is required.
3F request discovery retains its factory scope and native request validity checks.

Merkl uses off-chain allocation/proof witnesses plus pinned-block distributor roots and
claimed ledgers. It supports multiple configured guarded claims and independent
zero gates, so idle rewards do not prevent a ready native claim. Tokens, array
shape, proof depths, recipient guards and root-stability checks remain committed.
The frontend guarded claim captures its live nonzero root and verifies that it stays unchanged
through the claim; later valid roots do not require permission refresh when their
proof layout still fits. The provider fills claim witnesses without requiring this exact guard program. New tokens, changed proof depths or changed recipient guards
require a refreshed permission artifact; the keeper cannot expand the tree from unused permissions.

## Liquid Lane connector host boundary

Connector acquisition and calldata construction belong to vault-solver's Merkle
extension. External signed terms come from RFQ/backend integrations and are
validated by contracts. The daemon has no connector provider, sizing logic or
connector signature verifier. External swap data remains subject to the committed
permission layout and contract guards, generic SDK runtime validation, full
execution simulation and canonical receipt checks.

The generic bounded-signing library does not itself acquire a quote. Its
range-only financial Inputs cannot execute through the native input resolver.
CoW/RFQ acquisition still requires a recognized provider shape with an authorized
recipe-selected budget, including fixed amounts; arbitrary bounded sign buffers remain rejected. Browser
signing authoring also requires verified matching price/clock facts and exact
permission membership.

Local tests use generated credentials, pinned API fixtures and ephemeral Anvil.
They establish recipe/provider execution behavior, not hosted service support,
contract deployment, permissions installation or a mainnet transaction.
