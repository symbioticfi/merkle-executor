# Native claim ABI subset

`abi/NativeClaims.json` is a manually selected consumer ABI, not a complete issuer
interface or deployment catalogue. `hack/generate-bindings.sh` generates
`bindings/nativeclaims/NativeClaims.go` with go-ethereum 1.17.10 `abigen --v2`.
The selected JSON SHA-256 is
`993e6575bc806806cb46eb9a7542b2389a7c39ff7d9553757b2521b4f8513f98`.
The core executor revision in `abi/source.json` is unchanged.

## Sources

The following source interfaces were checked on 2026-11-08:

- Adapters [`9fb87aaed7f3dc890f41789c3595b67e5de9ffc3`](https://github.com/symbioticfi/adapters/tree/9fb87aaed7f3dc890f41789c3595b67e5de9ffc3):
  `src/interfaces/ll-adapter/noon/INoonWithdrawalHandler.sol` supplies
  `claimWithdrawal(uint256)`, `getUserNextRequestId(address)`,
  `getWithdrawalRequest(address,uint256)` and `withdrawPeriod()`.
  The withdrawal tuple is `(uint256 amount,uint256 timestamp,bool claimed)`.
- The same adapters revision's Pareto interfaces under
  `src/interfaces/ll-adapter/pareto/`: `IParetoClaimState.sol`, `IParetoCDO.sol`,
  `IParetoCreditVault.sol`, `IParetoDepositQueue.sol` and
  `IParetoWithdrawalQueue.sol` supply the owner-ledger, strategy, epoch, price,
  pending and prefunding getters, plus direct and epoch withdrawal claims.
- The `claimDepositRequest(uint256)` queue selector and the matching authoring
  call signatures are recorded in the frontend's
  [`src/shared/abis/protocols/rwa.ts`](https://github.com/symbioticfi/curator-frontend/blob/da1febeaca05b42494cc814d7fb9cc8d6933b488/src/shared/abis/protocols/rwa.ts)
  at `da1febeaca05b42494cc814d7fb9cc8d6933b488`. This is a consumer signature
  declaration, not a full Pareto implementation ABI.
- OpenEden HYBOND
  [`contracts/extension/Express.sol`](https://github.com/OpenEdenHQ/openeden.hybond.audit/blob/db60d6741d9e92ff2793616f178fcea58d0333f7/contracts/extension/Express.sol)
  at `db60d6741d9e92ff2793616f178fcea58d0333f7` supplies
  `claimDepositEscrow(address,address)`, `depositEscrowBalance(address,address)`,
  `claimRedeemEscrow(address)` and `redeemEscrowBalance(address)`.
- Centrifuge protocol
  [`72a8acfdbb2510da72aba6045c054f6b9a8afe7e`](https://github.com/centrifuge/protocol/tree/72a8acfdbb2510da72aba6045c054f6b9a8afe7e):
  `src/misc/interfaces/IERC7540.sol` supplies claimable deposit/redeem and
  cancellation getters and the two cancellation claim methods.
  `src/misc/interfaces/IERC7575.sol` supplies the four `max*` getters,
  two-argument `deposit`/`mint` and three-argument `withdraw`/`redeem`.
  Cancellation claim return values follow this implementation's uint256-returning
  interface; the provider does not decode native claim return data.

## Supported discovery

The isolated `providers/claims` provider recognizes exactly one unconditional
managed action with no recipe queries or bindings. It returns one plain action
output per ready authorized claim, with no settlement task or serialized envelope.

| Claim shape | Discovery/readiness |
| --- | --- |
| Noon `claimWithdrawal(uint256)` | Scan owner IDs from zero to the next ID; require positive, mature, unclaimed requests. |
| Pareto epoch deposit/withdrawal claim | Scan owner epochs through the next epoch; require an owner entitlement, fixed price and completed pending/prefund processing. |
| Direct Pareto normal/instant withdrawal | Require a positive native owner entitlement. |
| OpenEden escrow refund | Read the matching owner/asset escrow ledger. |
| ERC-7540 deposit/mint/withdraw/redeem claim | Require positive claimable entitlement for request ID zero; select the corresponding native `max*` amount. |
| ERC-7540 cancellation claim | Require positive entitlement for the fixed request ID and account controller. |

Only permission-authorized calldata bits may change. Targets, selectors,
recipients/controllers, masks, proofs and recipe instructions stay fixed. Fixed
IDs and amounts are never widened. Preflight calls use the account as sender at
the pinned block, and generic admission still simulates the complete execution.

## Deliberate limits and evidence

- No nested or multi-action native claim discovery; ordinary synchronous vault
  operations and other native financial policy remain outside this provider.
- The amount-based async claim subset uses aggregate request ID zero and the
  two-argument deposit/mint entry points. It does not enumerate arbitrary
  ERC-7540 request IDs or support three-argument deposit/mint claims.
- A claim revert means unavailable; transport or malformed-response failures
  remain errors. Native request/distributor contracts enforce actual validity.
- Scans honor the host operation context and retain no restart cursor or state.
- These source pins establish the selected interface shapes. Local unit and
  Anvil tests do not establish current proxy implementations, hosted availability,
  live signing, deployed permissions or successful issuer settlement.
