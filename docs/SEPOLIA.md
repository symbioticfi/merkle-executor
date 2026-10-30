# Sepolia deployment

The [shared Sepolia basis](https://github.com/symbioticfi/adapters/blob/714a3f6eb8aa80b5eb6cbd29bb6087f2a31c9bca/script/deploy/testnet/manifests/sepolia-basis-20261030T194357Z.json) contains factories, registries,
implementations and protocol venues on chain `11155111`. It contains no vault,
MerkleAccount, MerkleAdapter, RecipeExecutor or workflow instance. The executor
service consumes existing RecipeExecutor instances, so there is no daemon config
to start until a curator creates and configures one.

Create the vault and Merkle adapter in the Sepolia Curator UI. The protocol owner
must approve the exact vault/adapter pair in the new AdapterRegistry before the
vault can allocate to it. Create a RecipeExecutor for that account, set its caller,
grant the executor its leaf roles, publish the recipe and accept the permission
root through Curator.

Copy [the ordinary config](../config/example.yaml) to an operator-local file.
Set `chain.chainId` to `11155111`, supply Sepolia read/write RPC URLs, set
`curator.url` to the matching Sepolia backend including `/api/v1`, and put the
actual RecipeExecutor address in `strategies`. Shared factory, implementation,
connector and registry addresses are not strategy addresses. Keep a separate
state directory for this chain and caller.

Start with `mode: simulate`, then use the existing commands:

```sh
bin/merkle-executor check-config --config config/sepolia.local.yaml
bin/merkle-executor plan --config config/sepolia.local.yaml
```

Successful planning verifies the configured instance and simulates its complete
recipe. Sending additionally needs the authorized caller's key environment and a
finite positive `txManager.maxFeeGwei` ceiling. A deployed issuer model is not a
claim that every protocol-specific provider can acquire its runtime inputs;
unsupported inputs still fail preparation.
