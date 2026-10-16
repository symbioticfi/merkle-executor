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

## PPLUS native-call compatibility

`pplus-frontend.json` is the unmodified metadata tree from the actual portable
curator-frontend compiler bundle generated for Plan064 at base
`b2d1b31fafecd6e7b9a8268cbfafa104b7ebeee3` plus canonical PPLUS support. Its
ERC4626 permission type version is 4. The original bundle fixture and compiler
source hashes are retained in curator-backend
`test/fixtures/tree-storage/pplus-frontend.json` and its `SOURCE.md`. Only the
`bundle.metadata` object was extracted with Python JSON serialization.

SHA256: `9681b65cbf2734de6d4dc7d6e4400b6b28bfd19bd2e9a13c9978bd1fcd40242f`.

`TestPplusFrontendNativeCallsUseGenericAssembly` validates all actual tree proofs
and assembles USDC deposit and self-approved PPLUS `redeemShares` requests using
existing generic Approve/Manage steps and numeric inputs. It preserves account
receiver, token/spender, data masks and proofs, with approval cleanup, and rejects
receiver, mask, proof and spender changes. No protocol provider, signed external
input or keeper claim action is needed. These are deterministic ABI/permission
tests; deployed PPLUS eligibility and queued settlement are covered by separate
adapters native-fork and NAV historical-read evidence, not by the local Anvil
executor fixture.
