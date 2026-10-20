# Liquid Lane issuer coverage

This catalogue covers issuer requests and claims. Public RFQ consumer redemptions
use the separate [liquidlane-rfq provider](PROVIDERS.md#liquid-lane-rfq-consumer-redemptions);
signed connector discount production remains vault-solver.

The [asset catalogue](../internal/providers/redemptions/catalogue/liquid-lane.yaml) now contains **54 routes
over 38 tokens**. It retains the 15 published Liquid Lane assets and Noon routes,
adds the current frontend's 24 Midas products and additional redemption venues,
six Centrifuge products across nine asset-specific vaults, independent Hastra
unwraps/requests, and FalconX BB direct requests. The 2026-10-19 refresh compares
local frontend/NAV sources; it is not a new live topology capture. Source hashes
are retained in [the catalogue provenance](../internal/providers/redemptions/catalogue/SOURCE.md).
The authenticated recipe and permission proof grant execution authority; the
catalogue supplies no account, signer, approval, permission tree or amount.

This is the automatic issuer-preparation scope, rather than a copy of the NAV
inventory. Other committed actions, including Aave V4, Morpho loan-token supply,
TBILL and utility configuration calls, use the existing generic recipe VM and
its result-derived inputs. Valuation coverage does not create an automatic
provider for external request selection, signatures or unconstrained inputs.

Issuer calls originate from the MerkleAccount. The published LL account factories
identify the documented asset inventory; the provider does not transfer assets to
those account wrappers or invoke their `sync()` methods. Those wrappers keep their
own balances, request state and adapter allowance, which cannot substitute for a
MerkleAccount redemption.

| Family | Assets | Provider modes | Proceeds |
| --- | --- | --- | --- |
| Midas | 24 frontend products, including additional venues and BTC/WETH settlement assets | `request` | Issuer pushes the requested output asset after approval. |
| Centrifuge ERC-7540 | JAAA, JTRSY, HYB, deJAAA, deJTRSY, ACRDX | `request`, `claim` | Account claims fulfilled shares from its asset-specific vault. |
| OpenEden | HYBOND | `request` | Issuer processes its queue and pushes USDC. |
| Figure/Hastra | PRIME, AUTO, wYLDS | `unwrap`, `request` | Separate unwraps settle in wYLDS; a wYLDS request later pays USDC. The ordered combined wrapper/request flow is retained. |
| Pareto | AA_FalconXUSDC, BB_FalconXUSDC | `queue-request`, `direct-request`, `queue-claim`, `direct-claim`, `instant-claim` | Account claims an eligible queue epoch or CDO receipt. |
| Noon | sUSN | `request`, `claim` | Account claims the fixed USN entitlement after the current withdrawal period. |
| Noon | USN | `redeem-usn` | Separately whitelisted account burns USN and receives USDC atomically. |

The symbols mF-ONE and mM1-USD correspond to `mFONEAccountFactory` and
`mM1USDAccountFactory` on the addresses page. HYB and HYBOND have different issuers
and redemption interfaces.

## Automatic route and recipe requirements

Startup registers issuer redemption support automatically. Operators do not supply
provider entries, asset addresses, modes, action mappings, amounts or catalogue
paths. The executor derives a unique known route from the chain, target, exact ABI,
token pair and recipients committed in the authenticated recipe and permission
leaves. Masked or runtime-bound token pairs and recipients are invalid. Unknown
contracts sharing a selector remain ordinary committed recipe actions.

Amounts must bind directly to a read-dependent recipe Result, or use an Input with
an exact preceding equality to a read-dependent result. Use explicit arithmetic
such as `min(balance, cap)` to select a capped amount. Upper/lower bounds validate
the selected value; bounds alone cannot determine it.
Centrifuge request/claim and Midas request calls bound directly to a read-dependent Result use native
recipe execution. They can follow mint or claim effects and use fresh in-transaction
reads, make no issuer preparation reads, and acquire no receipt-provider ownership.
Zero amounts follow the recipe's manage gate and full execution simulation. An
Input-backed Centrifuge or Midas route keeps its exact budget, readiness checks and
exclusive receipt ownership. Pareto and Noon complete-request claims have no amount field. Their
epoch or request ID must be a committed literal or exact deterministic Result.
An Input needs a preceding exact equality; a range-bound arbitrary index is invalid.

Independent Hastra unwraps use PRIME/AUTO `redeem` and settle in wYLDS. A separate
wYLDS `requestRedeem` uses the account's selected read-derived wYLDS balance and
waits while an existing request is pending. The combined Figure flow retains an
ordered wrapper `redeem` and wYLDS `requestRedeem`, with the
second amount linked to the first through the committed `previewRedeem` query.
The original recipe retains its output and balance guards. A read depending on
an earlier effect is unsupported until its actual result can be resolved safely;
preparation does not substitute a preview for an actual post-effect balance.

The recipe owns approvals, ordering and balance/output conditions. Providers fill
only existing authorized fields. Requests and claims run independently: a
confirmed request establishes issuer submission, while later settlement or a
claim belongs to a separate approved recipe.

The checked-in catalogue is copied exactly into
`internal/providers/redemptions/catalogue/liquid-lane.yaml`, SHA-256
`14f5c8979da891a85b913244d9829d93f53d507263413d990d73026ff9139fe6`.
Tests enforce byte-for-byte equality with the reviewed source. A catalogue update
requires deliberate review, copy and policy-version update. Recovery validates
stored configuration, account, action bytes and numerical inputs against this
embedded policy before I/O; it does not discover a new route or amount.

## Issuer calls and readiness

- **Midas:** check `mToken()`, `tokensConfig(outputAsset)` and `minAmount()`;
  approve the redemption vault, then call `redeemRequest(outputAsset, amount)`.
  Requests are readable through `redeemRequests(id)`. The returned request amount
  can exclude a fee. Issuer greenlists, vault/function pauses and feed health still
  apply. The LL wrapper's mGLOBAL cutoff scheduling is not an issuer method and
  is not recreated by the direct provider.
- **Centrifuge:** resolve `token.vault(outputAsset)` and require the configured
  target and its `asset()` to match. Submit `requestRedeem(amount, account,
  account)`. Claim from the aggregate `maxRedeem(account)` allowance using
  `redeem(shares, account, account)`. A claim needs no historical request IDs.
  `pendingRedeemRequest` and `claimableRedeemRequest` expose issuer request state;
  membership and transfer restrictions remain issuer-controlled.
- **OpenEden:** verify `token()` and `redeemAsset()`, and observe `redeemMinimum()`
  before `requestRedeem(account, amount)`. Pending and final queue balances are
  `pendingRedeemInfo(account)` and `redeemInfo(account)`. The Express contract
  pays the receiver during issuer processing; it has no user-side claim call.
- **Figure:** verify `PRIME.asset() == wYLDS` and `wYLDS.asset() == outputAsset`.
  Execute PRIME `redeem(amount, account, account)` followed by wYLDS
  `requestRedeem(wYLDSAmount)`. `pendingRedemptions(account)` returns shares,
  assets and timestamp. There can be only one outstanding request per account.
  Preparation validates the exact recipe-derived `previewRedeem` amount.
  Reconciliation verifies the PRIME `Withdraw` sender, receiver, owner, shares and
  assets, then matches the request to those exact assets. An unrelated wYLDS
  balance cannot establish the output link.
- **Noon:** verify `sUSN.asset() == outputAsset`, `handler.usn() == outputAsset`
  and, for new requests, `handler.hasRole(handler.STAKING_VAULT_ROLE(), sUSN)`.
  Existing claims remain available if that role is later revoked. Request with
  `sUSN.redeem(shares, handler, account)`; it transfers USN to the handler and
  records an entitlement for the calling account. A claim reads that account's
  exact `getWithdrawalRequest(account, requestId)`, requires a positive unclaimed
  amount and `timestamp + withdrawPeriod() <= snapshot timestamp`, then calls
  `handler.claimWithdrawal(requestId)`. It pays USN to the account. Shares and
  USN both have 18 decimals. The live withdrawal period applies to existing
  requests. The deployed staking vault stores its handler privately and exposes
  no `WITHDRAWAL_HANDLER()` getter. Complete recipe simulation verifies the
  configured handler receiver and vault pause/blacklist/whitelist conditions.
  Whitelisted direct-withdrawal accounts cannot use this asynchronous receiver
  route. A request can acquire shares earlier in the same recipe; preparation
  therefore does not require those shares to be held before recipe execution.
  The separate `redeem-usn` mode targets Ethereum RedeemHandlerV2
  `0x9F8D0a683871125083170F65430F8A15aFfa0086`, burns 18-decimal USN and pays
  6-decimal USDC from its treasury. It requires that handler's account whitelist,
  allowed collateral, fresh full-amount quote, available block budget, and treasury
  USDC balance plus allowance. It has no request ID or issuer-side claim delay.
  The recipe supplies USN approval/cleanup and an output check; the provider does
  not create privileges. USDT, USN minting, request discovery and ID scheduling
  remain outside this integration.
- **Pareto:** validate the queue's `tranche()`, `underlying()`, `idleCDOEpoch()`
  and `strategy()` for queue operations. Direct requests validate the CDO/strategy
  and the selected AA or BB tranche without consulting the AA queue. BB supports
  only direct requests; claim calls remain account-level and use the canonical AA
  route. During a running epoch use queue `requestWithdraw(amount)`;
  between epochs use CDO `requestWithdraw(amount, token)`. A queue claim requires
  positive `userWithdrawalsEpochs(account, epoch)`, a positive
  `epochWithdrawPrice(epoch)` and zero `epochPendingClaims(epoch)`, then calls
  `claimWithdrawRequest(epoch)`. A direct claim uses strategy
  `withdrawsRequests(account)` plus `apr0Users(account)` and the
  elapsed-epoch/closed-vault condition. Completed APR0 epochs also include their
  `apr0RateByEpoch` interest, with the deployed contract's rounding and overflow
  behavior. Closed-pool current-epoch principal accrues no unfinished epoch rate;
  an instant claim uses `instantWithdrawsRequests(account)` and CDO
  `allowInstantWithdraw()`. Full recipe simulation still checks actual liquidity
  and the configured deployed implementation.

Confirmed account `Manage(Action[])` logs can correlate the exact executed issuer
action with the authorizing receipt. A skipped Manage step emits no such log.
Issuer event layouts are deliberately not guessed: Midas revisions use different
indexing, and the vendored interfaces do not define a universal request event.
Generic token-balance changes alone do not identify an issuer request or prove its
settlement.

## Provenance and verification boundary

Historical factory addresses come from the official page. The original token and issuer addresses come
from adapters revision `bd5e235899de5e175a0001a913d47596f6f4f801`, principally
`src/interfaces/ll-adapter/tokens-to-redeem/`,
`script/deploy/adapters/DeployLiquidLane2.s.sol` and
`test/adapters/merkle-adapter/RecipeRedemptionsMainnet.t.sol`.

The original five Centrifuge USDC vault addresses, Pareto strategy address and wYLDS output
asset were also read through public Ethereum RPC at block **26,074,706**, hash
`0xb942e450ee9fa50321f5f54530b9cce7a03a2ef30bf9b8fbfe0cd30e3e1a9bc3`.
These are topology checks; they do not verify any configured account's KYC,
available liquidity, accepted recipe or complete redemption lifecycle. Providers
must validate topology again at their own pinned snapshot.

The sibling fork tests exercise real issuer calls with locally deployed accounts,
funding fixtures and, where required, issuer-admin enrollment on the fork. They
establish calldata compatibility, not permissionless production access. The fork
request tests generally stop before final asynchronous payout.

ABI arrays are consumer subsets extracted from the sibling's compiled interfaces
with Solidity 0.8.37. Go files are generated by go-ethereum abigen 1.17.5 with
`--v2`; generated files must not be edited. The mappings are:

| Vendored ABI / Go package | Compiled interface source |
| --- | --- |
| `MidasRedemption` / `midasredemption` | `IMidasRedemptionVault` plus read-only `IMidasPostVault` methods from the recipe fork test |
| `CentrifugeShare` / `centrifugeshare` | `IERC7575Share` |
| `AsyncRedeemVault` / `asyncredeemvault` | `IAsyncRedeemVault` |
| `FigureYieldVault` / `figureyieldvault` | `IFigureYieldVault` |
| `OpenEdenExpress` / `openedenexpress` | `IOpenEdenExpress` plus `IOpenEdenExpressConfig` from the deployment script |
| `ParetoQueue` / `paretoqueue` | `IParetoWithdrawalQueue` |
| `ParetoCDO` / `paretocdo` | `IParetoCDO` plus issuer ABI views `allowInstantWithdraw` and `epochEndDate` |
| `ParetoStrategy` / `paretostrategy` | `IParetoCreditVault` plus issuer-source views `withdrawsRequests`, `instantWithdrawsRequests`, `lastWithdrawRequest`, `apr0Users` and `apr0RateByEpoch` |
| `NoonWithdrawalHandler` / `noonwithdrawalhandler` | Verified deployed handler read, role and claim methods |
| `NoonRedeemHandlerV2` / `noonredeemhandlerv2` | Reviewed Ethereum native USN→USDC getters, redemption and receipt event |
| `ERC4626` / `erc4626` | `IERC4626`: `asset`, `maxRedeem`, `maxWithdraw`, `previewRedeem`, `redeem`, `convertToAssets`, `Withdraw` |

The additional Pareto views come from
[Idle-Labs/idle-tranches revision 2a62e05](https://github.com/Idle-Labs/idle-tranches/tree/2a62e05b26e2d42fc3535dc30d2741058d24c5e4).
The CDO views were copied from `abi/IdleCDOEpochVariant.json`; the strategy views
are explicit ABI entries for the three public `mapping(address => uint256)`
getters in `contracts/strategies/idle/IdleCreditVault.sol`. This is a limited
read/request/claim interface, not the complete protocol ABI or a claim that every
upstream feature exists on the configured deployment.

APR0 getter ABI entries and claim arithmetic were checked against the deployed
strategy proxy's verified implementation
[0x62568889198F1bAb603E26dA7b6c1808838fE489](https://sourcify.dev/server/v2/contract/1/0x62568889198F1bAb603E26dA7b6c1808838fE489?fields=abi,sources).
That implementation combines normal and APR0 requests and has no separate
`postDefaultRequests` mapping. Newer upstream code adds different default handling;
it must not be substituted without reviewing the actual deployed implementation.
Queue claims require an exact epoch committed or derived in the recipe; this
provider does not discover or roll forward queue epochs.

The [Figure curator guide](https://github.com/provenance-io/hastra-eth-vault/blob/main/docs/CURATOR_INTEGRATION.md)
documents the single-pending-request constraint and admin-driven completion.
The [Pareto integrator guide](https://github.com/pareto-credit/docs/blob/master/developers/integrators/smart-contract.md)
documents queue versus direct requests and CDO claim eligibility.

Source SHA-256 hashes used to extract or define the ABI subsets:

| Source relative to adapters, unless marked upstream | SHA-256 |
| --- | --- |
| `src/interfaces/ll-adapter/midas/IMidasRedemptionVault.sol` | `7124e276aa507a99d8870033503ff295da92ca0645e12273d38ef4607b84c1e0` |
| `test/adapters/merkle-adapter/RecipeRedemptionsMainnet.t.sol` | `d13cea981299aa4cba210d2c60fc2e6642accc92e2729d5c89106d65b361172e` |
| `src/interfaces/ll-adapter/IERC7575Share.sol` | `de4535c350c0a938451104a129b02604c1a8c2484b34c7f0bc19a4a046b47a76` |
| `src/interfaces/ll-adapter/IAsyncRedeemVault.sol` | `2f66acc77bfd1926e7219c5689c9627bd2e7bb3aea33de80dc343ae3a4ef8f19` |
| `src/interfaces/ll-adapter/figure/IFigureYieldVault.sol` | `87751ce3701232150831533e4c0efabf97b0ea0d790edb239b79d8dfcff5ded4` |
| `src/interfaces/ll-adapter/openeden/IOpenEdenExpress.sol` | `85329c9c4862a00a2597ada1eb56b2e2895913f40d20a16a6d7ad37f8c2493ff` |
| `script/deploy/adapters/DeployLiquidLane2.s.sol` | `6613fd86b8a77cbf1031d29fe01cac99569041527bd7c8ba6464a7dce77db189` |
| `src/interfaces/ll-adapter/pareto/IParetoWithdrawalQueue.sol` | `412a5fbd642051739f7883078d483aa65dbebffcf9655b506d508dfb7734cce2` |
| `src/interfaces/ll-adapter/pareto/IParetoCDO.sol` | `6173975a2a05d470e70f0925c31925e2c619a9204d2b616a029203ad70723e86` |
| `src/interfaces/ll-adapter/pareto/IParetoCreditVault.sol` | `50a244a60c4b2983c8f2d31a016bdff662199a4cfda767cf1c8cfd482f0451a1` |
| `lib/openzeppelin-contracts/contracts/interfaces/IERC4626.sol` | `a43d346b74ce2cfa871ce3386aaaf6aec719e205e7c8148efc1c95612fc7b8eb` |
| Upstream `abi/IdleCDOEpochVariant.json` | `18f071bfc784f5aa54777a2419d76193d4ce51b224ef40f05870725afed6c76c` |
| Upstream `contracts/strategies/idle/IdleCreditVault.sol` | `eabd44b467a450b39dde044a1b84de0849ab5f5b570ca0928bb5c09c3a4bf8c5` |

The Noon handler subset was extracted on 2026-09-28 from the verified deployed
[WithdrawalHandler source and ABI](https://sourcify.dev/server/v2/contract/1/0x0DaBc0D9B270c9B0C4C77AaCeAa712b56D0F9178?fields=abi,sources).
The existing ERC4626 binding supplies `asset` and `redeem`. The staking vault
receiver behavior was checked against the
[verified implementation](https://etherscan.io/address/0xebbcbc6672683e1956125e7c5e89e14ceac8cd3d#code);
it uses private namespaced storage for the withdrawal handler. No live Noon
request or claim was submitted. Local tests verify exact calldata, ownership,
maturity boundaries and confirmed managed-call identity; they do not establish
deployed account eligibility or handler funding.

Noon `contracts/WithdrawalHandler.sol` SHA-256: `262670b0ba0c9f49bb81a80f0298eaf3d0009b5916ec44ee3d1c09ab20be62dc`.

Native USN redemption was reviewed on 2026-10-12 against the
[verified RedeemHandlerV2 source and ABI](https://eth.blockscout.com/api/v2/smart-contracts/0x9F8D0a683871125083170F65430F8A15aFfa0086).
`contracts/RedeemHandlerV2.sol` SHA-256:
`095557c6fdedf7647fe76268b3bf7af132507ca9afd9473947f9d23cf30aa9ec`.
The exact 30-source closure, compiler/settings and ABI subset are pinned under
`internal/providers/redemptions/testdata/noon/`. The subset excludes signed-order
redemption, minting, USDT and administration. Source-pinned local execution tests
use the unmodified handler source, existing RecipeExecutor bytecode and a
permission-enforcing account fixture with local burnable token/oracle fixtures.
They prove local burn/payment/cleanup and fail-closed execution; they do not
establish live issuer enrollment, treasury liquidity or production token behavior.
