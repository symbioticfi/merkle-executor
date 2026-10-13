# Liquid Lane issuer coverage

This catalogue covers issuer requests and claims. Public RFQ consumer redemptions
use the separate [liquidlane-rfq provider](PROVIDERS.md#liquid-lane-rfq-consumer-redemptions);
signed connector discount production remains vault-solver.

The [asset catalogue](../config/assets/liquid-lane.yaml) covers all **15 asset
factories** listed under Liquid Lane on the [official addresses page](https://docs.symbiotic.fi/get-started/resources/addresses),
checked on 2026-09-28, plus the additional **sUSN** native issuer route from the
adapters source. The separate AccountRegistry entry is not an asset. The
catalogue is embedded discovery data. An authenticated action leaf still supplies
the authority to execute; the catalogue supplies no account, signer, approval,
permission tree or amount.

Issuer calls originate from the MerkleAccount. The published LL account factories
identify the documented asset inventory; the provider does not transfer assets to
those account wrappers or invoke their `sync()` methods. Those wrappers keep their
own balances, request state and adapter allowance, which cannot substitute for a
MerkleAccount redemption.

| Family | Assets | Provider modes | Proceeds |
| --- | --- | --- | --- |
| Midas | mF-ONE, mROX, mHYPER, CarryTradeUSDTRYLeverage, StockMarketTRBasisTrade, mM1-USD, mGLOBAL | `request` | Issuer pushes the requested output asset after approval. |
| Centrifuge ERC-7540 | JAAA, JTRSY, HYB, deJAAA, deJTRSY | `request`, `claim` | Account claims fulfilled shares from its asset-specific vault. |
| OpenEden | HYBOND | `request` | Issuer processes its queue and pushes USDC. |
| Figure/Hastra | PRIME | `request` | PRIME is redeemed to wYLDS; the issuer later pushes USDC for the wYLDS request. |
| Pareto | AA_FalconXUSDC | `queue-request`, `direct-request`, `queue-claim`, `direct-claim`, `instant-claim` | Account claims an eligible queue epoch or CDO receipt. |
| Noon | sUSN | `request`, `claim` | Account claims the fixed USN entitlement after the current withdrawal period. |

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
preceding equality or upper-bound checks against recipe results. Equality selects
the exact value; otherwise the executor selects the smallest upper bound, after
checking every lower bound. Literal caps alone cannot determine an amount.
Centrifuge claims keep that exact share budget and wait if the available allowance
is smaller. Pareto and Noon complete-request claims have no amount field. Their
epoch or request ID must be a committed literal or exact deterministic Result.
An Input needs a preceding exact equality; a range-bound arbitrary index is invalid.

Figure requires an ordered PRIME `redeem` and wYLDS `requestRedeem`, with the
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
`d29862ea24a7968ca7161780d272f70cfe314418c603839fb884985aaebd95f2`.
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
- **Pareto:** validate the queue's `tranche()`, `underlying()`, `idleCDOEpoch()`
  and `strategy()`. During a running epoch use queue `requestWithdraw(amount)`;
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

Factory addresses come from the official page. Token and issuer addresses come
from adapters revision `bd5e235899de5e175a0001a913d47596f6f4f801`, principally
`src/interfaces/ll-adapter/tokens-to-redeem/`,
`script/deploy/adapters/DeployLiquidLane2.s.sol` and
`test/adapters/merkle-adapter/RecipeRedemptionsMainnet.t.sol`.

The five Centrifuge USDC vault addresses, Pareto strategy address and wYLDS output
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
