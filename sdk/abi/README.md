# Contract interfaces

These ABI arrays are copied from the sibling adapters Foundry artifacts at
`bd5e235899de5e175a0001a913d47596f6f4f801` (Solidity 0.8.37):

- `out/RecipeExecutor.sol/RecipeExecutor.json`
- `out/MerkleAccount.sol/MerkleAccount.json`
- `out/IERC20.sol/IERC20.json` (vendored as `ERC20.json`)

Refresh the `.abi` arrays from those artifacts after rebuilding adapters, then
regenerate Go bindings with `make generate`, using abigen 1.17.5 and
its `--v2` generator. The Recipes instruction payloads are declared in
`sdk/recipes/codec.go`, because they are encoded data rather than public
contract methods. The independently encoded fixture and local EVM test verify
those payloads against the current RecipeExecutor.

RecipeExecutor uses opcode-prefixed `bytes[]` steps, packed `uint256` operands
and bindings, and `SetRecipe(bytes32,bytes[],bytes[])`. MerkleAccount exposes
the active permission root through `root()`.

