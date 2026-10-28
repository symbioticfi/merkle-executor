# Aave V3 capacity input

Both the standalone daemon and vault-solver register this SDK provider through
`aave.New(reader)`. The host supplies the pinned RPC transport; no host implements
Aave sizing. The provider recognizes one authenticated, account-targeted
`RecipeCapability.execute(bytes[],bytes[],uint256[])` containing a variable-rate
`borrow(asset, amount, 2, 0, account)` followed by the committed
`getHealthFactor(pool, account)` floor. Pool, asset, account, helper, floor and all
ABI framing are pinned. Only the borrow amount binding may vary.

Input 0 supplies borrowing **capacity**, in raw debt-asset units. The committed
recipe chooses the actual amount with its `Min` chain and policy debt headroom.
Input 0 must occur once as a direct operand of that chain; other Inputs still
require generic exact recipe sources. Multiple capacity groups, effects between
sizing and borrowing, and unresolved prefix
inputs fail before network I/O. There is no amount or deployment configuration.

The provider replays the authenticated supply prefix in one same-executor
`eth_call` code override at the pinned block, then probes Aave's resulting state.
Executor identity, account permissions and original instruction order are
preserved. Prefix numerical witnesses are persisted and checked against full
original execution; the final original-code simulation and the on-chain health
floor remain authoritative. No probe contract is deployed or granted permission.

`lens/AaveBorrowProbe.sol` ports the `maxBorrow` calculation and borrowing flags
from adapters [`AaveV3Liquidity.sol`](https://github.com/symbioticfi/adapters/blob/d506348870eb2d379ddae69407bc8b63422291d6/src/contracts/merkle-adapter/automations/liquidity/AaveV3Liquidity.sol)
at commit `d506348870eb2d379ddae69407bc8b63422291d6` on
`codex/bailsec-normalized`. That source has SHA-256
`140af89383d56e9d5d95b6ed812ce82193641b041aaa50054b0d72cc2b62a547`.
The percentage multiplication is checked and floored; full-precision floor
`mulDiv` follows the adapters vendored OpenZeppelin Math implementation (MIT,
Remco Bloemen/Uniswap Labs). It preserves global debt, effective eMode threshold,
LTV availability, reserve flags, isolation/silo restrictions, borrowable bitmap,
raw/virtual cash, reserve debt cap and one-unit rounding margins. Zero base LTV
collateral is permitted when native effective eMode accounting permits borrowing.
Required getter failures fail preparation; no fallback is used.

Regenerate with `python3 sdk/providers/aave/lens/generate.py`; `make check`
checks the artifact against a fresh pinned-compiler build. `FixtureAave.sol` is an
Anvil test double only. `testdata/frontend-lending.json` projects the generated
frontend lending recipe and referenced leaves, retaining their original proofs.
It models topology and proves byte-level compatibility, without live-chain evidence.

`Advance` validates the immutable persisted envelope and confirmed execution
identity before completing the native phase. Canonical receipt checks remain in
the keeper; no issuer event or later settlement is inferred.

Adding this provider to a host changes its durable provider roster. Finish or
reconcile all active jobs with the existing version before upgrading. Older
pending journals fail the provider-count check; do not clear them to bypass it.
