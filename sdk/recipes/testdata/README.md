# Recipe fixtures

`merkle-tree.json` is copied from the adapters contract test fixture. All 33 stored
leaves and proofs are checked using the Go codec. `core-abi.json` was independently
encoded with Foundry `cast abi-encode`/`cast calldata`; its recipe includes Read,
Add and Call (operation 11), and its hash was obtained with `cast keccak`.
Regenerate it with `python3 internal/recipes/testdata/generate.py`. The vectors
cover opcode-prefixed steps, packed input/result bindings, both call selectors
and the SetRecipe event topic independently of the Go ABI codec.

`anvil/RecipeExecutor.json` vendors the deployment bytecode and ABI from the
current adapters Foundry artifact. `anvil/Fixture.sol` contains local-only account
and token test doubles. The account fixture verifies leaf hashes, sorted proofs
and executor roles. `anvil/sources-sha256.json` records the relevant source snapshot.
Regenerate these artifacts with `python3 internal/recipes/testdata/anvil/generate.py`
after rebuilding adapters; this requires cached Solidity 0.8.37 and Foundry.

Run the independent local chain tests with:

```sh
make integration
```

The tests launch ephemeral loopback Anvil nodes, discover a committed recipe,
assemble permissions, simulate and execute twice, check balances and allowance
reset behavior, and verify Call and zero-gated runtime slots. Packed-value tests
exercise the maximum `uint216` literal and full `uint256` inputs, numerical
results and bound writes. The service test covers plan/send/restart. These tests
use no external RPC endpoint or live signing credentials.
