# Native issuer execution validation

Native issuer semantics belong in the committed recipe: calls and reads, exact
amounts and request IDs, eligibility gates, approval cleanup and output guards.
The daemon has no native issuer provider, selector catalogue or deployment list.
Generic permissions, numerical dependencies, full simulation, atomic execution and
canonical transaction confirmation apply to these recipes as they do to other native
protocol actions. [Input requirements and execution boundary](PROVIDERS.md#native-on-chain-recipes).

## Portable regression tests

Run `make check` and `make integration` for the root module and nested SDK. The
root uses this checkout's SDK through its local module replace; SDK checks also run
independently. Integration tests start owned, unforked Anvil processes with local
fixtures. They use no live signing credentials or external chain writes.

The native application fixtures exercise repeated claim/read/request compositions,
skipped gates, approval cleanup and proceeds guards through the real RecipeExecutor.
A failed final guard must reject preparation, and a mined revert must roll back
balances, allowance, requests and logs. RAM lifecycle tests retain signed identity through ambiguous responses and
canonical receipt re-inclusion. Process exit discards that tracking; there is no
crash recovery or replay guarantee.

Request and claim are independent recipes. A confirmed native transaction settles
generic execution; the daemon does not infer issuer payout or discover the next
request ID. Current-state reads and exact bindings must establish each later claim.
Local asynchronous issuer fixtures live under
`cmd/merkle-executor/testdata/issuers`; they are not production dependencies.

## Source-pinned Noon fixture

`cmd/merkle-executor/testdata/issuers/noon/provenance.json` records the verified
Ethereum RedeemHandlerV2 source closure retrieved on 2026-10-12 from
`0x9F8D0a683871125083170F65430F8A15aFfa0086`, its SHA-256 hashes and original
Solidity 0.8.28/Paris/optimizer-200 compiler settings. The local test executes this
handler through a committed recipe with token, oracle and account fixtures.
It checks full-amount quote rounding, eligibility and treasury failures, atomic
rollback, output guards and generic canonical confirmation.

```sh
python3 cmd/merkle-executor/testdata/issuers/noon/generate.py --check
make check
make integration
```

The source-pinned handler is test evidence. Local token/oracle fixtures do not
establish production token restrictions, account enrollment, treasury liquidity,
future proxy behavior or issuer availability. No native issuer ABI or deployment
catalogue is imported by production Go code.
