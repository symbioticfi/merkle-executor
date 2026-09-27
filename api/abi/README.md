# Contract interfaces

These ABI arrays are copied from the current sibling adapters Foundry artifacts:

- `out/RecipesExecutor.sol/RecipesExecutor.json`
- `out/MerkleAccount.sol/MerkleAccount.json`
- `out/IERC20.sol/IERC20.json` (vendored as `ERC20.json`)

Regenerate Go bindings with `hack/generate-bindings.sh`, using abigen 1.17.5 and
its `--v2` generator. The internal Recipes instruction payloads are declared in
`internal/recipes/codec.go`, because they are encoded data rather than public
contract methods. The independently encoded fixture and local EVM test verify
those payloads against the current RecipesExecutor.
