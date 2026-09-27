# Recipe fixtures

`merkle-tree.json` is copied from the adapters contract test fixture. All 33 stored
leaves and proofs are checked using the Go codec. `core-abi.json` was independently
encoded with Foundry `cast abi-encode`/`cast calldata`; its recipe includes Read,
Add and Call (operation 11), and its hash was obtained with `cast keccak`.

`anvil/RecipesExecutor.json` vendors the deployment bytecode and ABI from the
current adapters Foundry artifact. `anvil/Fixture.sol` contains local-only account
and token test doubles. The account fixture verifies leaf hashes, sorted proofs
and executor roles. `anvil/sources-sha256.json` records the relevant source snapshot.
Regenerate these artifacts with `python3 internal/recipes/testdata/anvil/generate.py`
after rebuilding adapters; this requires cached Solidity 0.8.37 and Foundry.

Run the independent local chain test with:

```sh
MERKLE_ANVIL_TEST=1 go test ./internal/recipes -run TestAnvilFullExecute -count=1
```

The test launches its own ephemeral loopback Anvil, discovers a committed recipe,
assembles permissions, simulates and executes twice, checks balances and allowance
reset behavior, and verifies Call and zero-gated runtime slots. It never uses an
external RPC endpoint or a real signing key.
