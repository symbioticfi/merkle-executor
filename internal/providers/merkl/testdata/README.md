# Merkl Multicall3 integration fixtures

`Multicall3.sol` is the unmodified MIT-licensed official source from
[mds1/multicall3 at b667d67ecfa5361a81e8f110234ce242613b0012](https://github.com/mds1/multicall3/blob/b667d67ecfa5361a81e8f110234ce242613b0012/src/Multicall3.sol).
Its SHA-256 is `2054218939d3fa0f52f8ce1a33658d570a550671f63197356ee5744f7e188b1e`;
the generation script verifies this before compiling.

`FixtureMerklDistributor.sol` is a local getter fixture. It holds root, claimed
amounts and recipient settings keyed by account and token. Reads require the
configured aggregate caller so accidental direct reads fail. Its setter and forced
revert controls exist only for local tests. It does not implement reward claims.

Regenerate with Foundry and cached solc 0.8.12:

```sh
python3 internal/providers/merkl/testdata/generate.py
make generate
```

The offline compiler uses London EVM, optimizer 200 runs and no metadata bytecode
hash. The script writes both fixture artifacts with full ABIs and deployment
bytecode; it also extracts the `aggregate3`-only production ABI. Abigen v2 then
regenerates the production Go binding. No generated artifact should be hand edited.

`GOWORK=off make integration` selects `TestAnvilMerklMulticallUsesPinnedDistributorState`
through the existing CLI `TestAnvil` target. It deploys these contracts on an
ephemeral localhost Anvil, validates one aggregate call per preparation, cumulative
claim encoding, current deltas, historical block reads, recipient precedence,
minimum-based skips and aggregate reverts. It uses Anvil's local unlocked fixture
account, never external signing credentials or a live chain.
