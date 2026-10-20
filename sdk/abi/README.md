# Contract interfaces

These ABI arrays are generated from adapters `codex/bailsec-normalized` at
`0427c657375512a87158010c323556739fe96d1b` (Solidity 0.8.37):

- `out/RecipeExecutor.sol/RecipeExecutor.json`
- `out/MerkleAccount.sol/MerkleAccount.json`
- `out/IERC20.sol/IERC20.json` (vendored as `ERC20.json`)

The source pin is `source.json`. From the SDK directory, after rebuilding that
exact clean checkout, run
`MERKLE_ADAPTERS_REPO=/path/to/adapters python3 hack/sync-adapters.py`, then
regenerate Go bindings with `make generate`, using abigen 1.17.5 and
its `--v2` generator. The Recipes instruction payloads are declared in
`recipes/codec.go`, because they are encoded data rather than public
contract methods. The independently encoded fixture and local EVM test verify
those payloads against the current RecipeExecutor.

RecipeExecutor uses opcode-prefixed `bytes[]` steps, packed `uint256` operands
and bindings, and `SetRecipe(bytes32,bytes[],bytes[])`. MerkleAccount exposes
the active permission root through `root()`.

The CoW, Liquid Lane RFQ and Merkl providers also use the protocol ABI subsets in
this directory, including their Multicall3 transport. Sources and deliberate
subset limits are documented in [schema provenance](../schemas/PROVENANCE.md).
Native protocol calls and reads are encoded in committed recipes; issuer artifacts
used by local integration tests live under `cmd/merkle-executor/testdata/issuers`.
