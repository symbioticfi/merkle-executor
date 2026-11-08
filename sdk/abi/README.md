# Contract interfaces

These ABI arrays are generated from adapters `codex/bailsec-normalized` at
`0427c657375512a87158010c323556739fe96d1b` (Solidity 0.8.37):

- `out/RecipeExecutor.sol/RecipeExecutor.json`
- `out/MerkleAccount.sol/MerkleAccount.json`
- `out/IERC20.sol/IERC20.json` (vendored as `ERC20.json`)

The source pin is `source.json`. From the SDK directory, after rebuilding that
exact clean checkout, run
`MERKLE_ADAPTERS_REPO=/path/to/adapters python3 hack/sync-adapters.py`, then
regenerate Go bindings with `make generate`, using abigen 1.17.10 and
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

`DelegatedAccount.json` is the ERC-7579 `execute(bytes32,bytes)` and
`supportsExecutionMode(bytes32)` subset used by MetaMask's
[EIP7702DeleGatorCore v1.3.0](https://github.com/MetaMask/delegation-framework/blob/v1.3.0/src/EIP7702/EIP7702DeleGatorCore.sol).
Batch mode is `0x01` followed by 31 zero bytes: ordered CALLs, default atomic
execution. The local Anvil fixture `recipes/testdata/anvil/MetaMaskDelegator.json`
records that deployment's runtime, address, capture block and Keccak hash.
Native protocol calls and reads are normally encoded in committed recipes; issuer artifacts
used by local integration tests live under `cmd/merkle-executor/testdata/issuers`.

`NativeClaims.json` is the selected native issuer claim/discovery subset, generated
as `bindings/nativeclaims` by the same abigen v2 script. Its pinned interface
sources and deliberate limits are recorded in
[native claim provenance](../schemas/NATIVE_CLAIMS.md).
