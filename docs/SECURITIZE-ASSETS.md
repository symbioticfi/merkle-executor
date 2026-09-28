# Securitize asset routes

The `securitize` provider supports the published EVM redemption routes in the
[reviewed catalog](../config/assets/securitize.yaml). The catalog is a snapshot of
Securitize's [public asset feed](https://public-feed.securitize.io/asset-info), read
on **2026-09-28**. It contains ten symbols and 26 EVM deployments. Every address is
chain-specific. There are 25 wallet routes and four atomic offramps; VBILL on BNB
has no published redemption wallet or offramp and is explicitly unavailable.

| Symbol | EVM deployments in the feed | Published atomic offramps |
| --- | --- | --- |
| BUIDL | Ethereum, Optimism, Polygon, Avalanche, Arbitrum, BNB, Tempo | Ethereum: USDC, USDtb, RLUSD |
| ACRED | Ethereum, Polygon, Avalanche, Sei, Ink | None |
| VBILL | Ethereum, Avalanche, BNB | Ethereum: RLUSD |
| STAC | Ethereum | None |
| HLSCOPE | Ethereum, Polygon, Optimism | None |
| CURR | Ethereum | None |
| SECZ | Avalanche | None |
| BRSRV | Ethereum, Tempo | None |
| HINC | Ethereum, Avalanche | None |
| ARKVX | Ethereum | None |

The feed also contains eleven non-EVM entries on Solana, Aptos, Sui and TRON.
They are not EVM RecipeExecutor deployments and are excluded from this catalog.
Listing an EVM asset does not establish a deployed MerkleAccount, compatible chain
transaction environment, issuer enrollment, connector oracle/budget, or accepted
recipe permissions. Those remain deployment prerequisites. Feed display names
are not used as authorization: some current names are empty or placeholders.

## Configure a route

Use one provider entry per request or atomic redemption action. Amounts and
minimums are decimal strings in the corresponding token's base units. The example
below is an HINC request; its action index must match the actual committed recipe.

```yaml
- name: securitize
  config:
    catalog: assets/securitize.yaml
    symbol: HINC
    chainId: 1
    mode: wallet
    token: "0x015C477A4E2881C4B3bB8999f59ee4A04e812344"
    target: "0xbb543C77436645C8b95B64eEc39E3C0d48D4842b"
    amount: "1000000"
    action: "3:0"
    amountInput: 0
```

`catalog` resolves relative to the main configuration file. `catalog` and `symbol`
must be supplied together. The provider verifies the explicit chain, token, route
kind, target and payment token against that reviewed file; it does not fetch the
feed during execution. Updating discovery data cannot substitute a target or
recipient in an approved recipe. Operators can omit both fields for a separately
verified deployment, but must still explicitly pin every address and the chain.
The selected route must have the documented semantics; arbitrary contracts that
merely share selectors are not supported.

| Mode | Account action | Extra configuration | Completion outcome |
| --- | --- | --- | --- |
| `wallet` | Token `transfer(target, amount)` | No payment token or minimum | `requested` |
| `modern` | Offramp `redeem(amount, minOutput)` | `paymentToken`, positive `minOutput` | `redeemed` |
| `legacy` | Offramp `redeem(amount)` | `paymentToken`, positive `minOutput` preparation bound | `redeemed` |

For example, VBILL's Ethereum modern route uses token
`0x2255718832bC9fD3bE1CaF75084F4803DA14FF01`, target
`0x1eD617529d80AE87E6611f11D8de8532ECED42bC` and RLUSD payment token
`0x8292Bb45bf1Ee4d140127049757C2E0fF06317eD`. BUIDL's USDC and USDtb routes are
**legacy**, not modern: they implement `redeem(uint256)`, not the two-argument
method shown in Securitize's general dApp sample.

`amountInput` and `minOutputInput` are optional numerical recipe slots. Duplicate
assignments are rejected. The provider replaces a single action's calldata; it
does not add actions, change targets, modify permission masks, or create approvals.
The recipe must contain exact permitted token approvals for atomic offramps and
any approval reset or subsequent asset transfer. A wallet action targets the token
contract, while an atomic action targets the offramp. The complete recipe is
simulated, including upstream Liquid Lane inventory delivery and issuer controls.

## Receipt and settlement semantics

Preparation checks the pinned chain, token decimals, atomic offramp asset/payment
topology, pause state, current quote and available liquidity. Modern routes carry
`minOutput` into the onchain call. Legacy USDC returns equal base units; legacy
USDtb normalizes by `10^(outputDecimals-inputDecimals)`. The verified legacy code
requires the redeemer's balance to increase by exactly that normalized amount.
Underflowing decimal differences and uint256 overflows are rejected.

Confirmation uses the canonical execution receipt and the account's successful
`Manage` events to prove the exact configured call executed. A skipped action or
later unrelated token balance cannot complete the workflow. Wallet mode also
requires exact token delivery from this account to the pinned redemption wallet,
net of any return in the same receipt. Modern mode requires a matching
`RedemptionCompleted` event and the corresponding payment-token delivery. Legacy
mode requires the exact verified redemption call and normalized payment delivery.
Payment transfers are checked in the execution receipt, so later movement of
redeemed funds by the same recipe does not hide successful redemption.

`requested` means the tokens were delivered to the issuer's redemption wallet.
It **does not mean cash settled** and does not promise USDC, a payout address or a
settlement date. Securitize's general wallet-flow documentation describes separate
offchain processing; actual cash instructions depend on investor enrollment and
the fund. The public feed does not provide a request identifier, an authenticated
settlement-status endpoint or a claim method. This provider does not invent one,
poll arbitrary balance increases as settlement, or repeat a request to claim it.
Any later sweep/claim needs its own reviewed recipe and an isolated settlement
recipient or request-correlated issuer evidence.

Issuer investor/wallet enrollment, transfer locks, country restrictions, fund
terms and liquidity still apply. Existing balances or ERC-20 method compatibility
do not grant that permission. The executor simulates the exact approved amount;
it does not silently reduce a rounded or otherwise failing full-balance transfer,
retry with a different amount, or widen permissions.

## Sources and validation boundary

- Asset addresses, redemption wallets and advertised ramps come from the
  [issuer feed](https://public-feed.securitize.io/asset-info), including the
  `?symbol=HINC` and other symbol queries. The checked-in YAML is the reviewable
  snapshot; all 26 EVM rows and ten symbols are checked by catalog tests.
- [Securitize's sample integration](https://github.com/securitize-io/securitize-dapp-sample/blob/main/docs/SECURITIZE_SKILL.md)
  documents asset discovery, compliant wallet enrollment, atomic offramps and
  asynchronous wallet requests. The deployed contracts override generic sample
  assumptions, especially the legacy BUIDL selectors.
- `SecuritizeOffRamp.json` is a selected ABI subset of the verified implementation
  [0x76ffdD3628A370a05Ea7271D1A0690BcC936779F](https://sourcify.dev/server/v2/contract/1/0x76ffdD3628A370a05Ea7271D1A0690BcC936779F?fields=abi).
  Both published Ethereum RLUSD ramps resolved to this implementation when read.
- `SecuritizeLegacyOffRamp.json` comes from the verified nonproxy
  [BUIDL/USDC redemption contract](https://sourcify.dev/server/v2/contract/1/0x31D3F59Ad4aAC0eeE2247c65EBE8Bf6E9E470a53?fields=abi,sources).
  The matching [BUIDL/USDtb source](https://sourcify.dev/server/v2/contract/1/0x4580D3c5fFF079BDe60Af9e98B62d4FB031eE0a0?fields=abi,sources)
  establishes decimal normalization; its immutable bytecode identifies USDtb as
  `0xC139190F447e929f090Edeb554D95AbB8b18aC1C`.
- `SecuritizeToken.json` selects `decimals()` from the verified
  [ARKVX DSToken implementation](https://sourcify.dev/server/v2/contract/1/0xD57499B239700DDB99bb5B85596DDAb3745bC24d?fields=abi).
  `SecuritizeTopology.json` selects `liquidityToken()` from the verified modern
  `ILiquidityProvider` and `availableLiquidity()` from the verified legacy
  `ISettlement`. Bindings are generated using abigen 1.17.5 `--v2`.

Local tests exercise exact calldata, all catalog routes, decimal normalization,
minimum output, configuration mismatch, unavailable routes, paused/illiquid
ramps, malformed reads, canonical receipt identity, partial payments, mismatched
events and restart reconciliation. They are ABI and receipt fixtures, not live
issuer transactions or evidence that a particular account is eligible. No live
signing key, token transfer, investor registration or redemption was used.
