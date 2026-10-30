# Claim preparation

The `claims` automatic SDK provider handles unconditional, unbound native claim
calls in authenticated recipes. It discovers owner entitlements at the same
pinned block as recipe and permission discovery, prepares calldata and probes
issuer eligibility with the account as caller. Transport or malformed response
failures are errors; an issuer revert means that candidate is not currently ready.
The original executor call or complete multicall is then simulated with its real
caller and current permissions before signing.

Supported calls:

- Noon withdrawal request IDs, including ID zero.
- Pareto epoch deposit/withdrawal claims and direct normal/instant claims.
- ERC-7540 aggregate mint/deposit/redeem/withdraw and cancellation refunds.
- OpenEden deposit/redemption escrow refunds.
- 3F request rights, discovered from the factory attested by the claim capability.
  Enumeration includes older requests and transferred rights. The reviewed factory
  creates requests at CREATE nonce `7 + 3 * index`; `isRequest` attests every bound.
- Merkl cumulative claims use the existing Merkl provider and proof verification.

Only masked bytes change. The provider does not infer permission from a balance,
redirect a recipient, choose financial trade amounts or rewrite a recipe. A recipe
must have one claim operation, with no runtime amount/ID binding in that operation.
Its account leaf retains any necessary factory check, such as 3F `isRequest`.
Claims leave proceeds in the account. A separate repayment automation applies its
own reserve, cap and target-health checks.

Numbered discovery scans at most 4096 entries; overflow fails visibly. Eligible
requests are processed in ascending order, up to 16 per transaction. Further runs
rescan the owner ledger. `Output.Repeats` holds subsequent calldata variants of the
same committed recipe. The planner uses `RecipeExecutor.multicall` and retains the
variants in the durable plan. No external numerical result witness is shared
between invocations. One failed invocation reverts the batch.

No reviewed supported claim erases an older entitlement merely by claiming a newer
request. Noon marks one ID; Pareto queue clears one epoch; ERC-7540 and OpenEden
consume aggregate or escrow rights; each 3F request owns its own token balances.
Merkl's cumulative amount includes previously unclaimed rewards. Issuer-side FIFO
settlement is distinct from holder claim ordering. See the adapters claim document
for the reviewed sources. These tests establish encoding and local execution;
they do not establish live issuer funding or account eligibility.
