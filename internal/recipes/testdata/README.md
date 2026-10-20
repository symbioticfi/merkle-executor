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

`pplus-frontend.json` is the `bundle.metadata` object extracted from the
`compilePermissions` and `freezeTreeBundle` output of curator-frontend commit
`964898559f06551a3d66fbac108f8edd1354de89`, using the deterministic facts in
`src/shared/lib/merkle/operations/authoring/catalog/types/pplus.test.ts`.
Permission type versions are 1, as emitted by that compiler. Protocol and venue
group IDs are `ember`; PPLUS remains the native vault and token label. The full
bundle fixture and its provenance are retained in curator-backend
`test/fixtures/tree-storage/pplus-frontend.json` and its `SOURCE.md`. Only the
`bundle.metadata` object was extracted with Python JSON serialization.

SHA256: `dd737ca273d82c42fd4781aca6156eae7a5f0665403d9a87023afe83fbe72154`.

`TestPplusFrontendNativeCallsUseGenericAssembly` validates all actual tree proofs
and assembles USDC deposit and self-approved PPLUS `redeemShares` requests using
existing generic Approve/Manage steps and numeric inputs. It preserves account
receiver, token/spender, data masks and proofs, with approval cleanup, and rejects
receiver, mask, proof and spender changes. No protocol provider, signed external
input or keeper claim action is needed. These are deterministic ABI/permission
tests; deployed PPLUS eligibility and queued settlement are covered by separate
adapters native-fork and NAV historical-read evidence, not by the local Anvil
executor fixture.
