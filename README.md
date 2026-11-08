# Merkle executor

A Go daemon and independently versioned SDK for executing curator-committed
RecipeExecutor programs. Recipes own protocol calls, reads, amount selection,
permissions, gates and postconditions. The runtime supplies authorized inputs,
simulates the exact transaction, submits it and tracks external settlement.

All runtime state is held in RAM. There are no journals, databases, state
folders or restart recovery. Run one active owner per signing account. Restart
loses pending transaction and order tracking; old transactions and orders may
still execute. Drain work before upgrading or restarting.

## Run

Use Go 1.27.3 and copy [the example configuration](config/example.yaml).

```sh
make build
bin/merkle-executor check-config --config config/example.yaml
bin/merkle-executor plan --config config/example.yaml
bin/merkle-executor run --config config/example.yaml
```

Simulation is the default and needs no key. Send mode requires a finite positive
`txManager.maxFeeGwei` and a private key supplied through `signer.keyEnv` (default
`MERKLE_EXECUTOR_PRIVATE_KEY`). YAML contains environment variable names, never
secret values. Read and write RPC endpoints must be HTTP(S) on the configured
chain. Redirects are refused. Executor accounts and their common caller are
validated at startup; the signing key must match that caller.

`strategies` is an ordered list of executor addresses. Accounts come from the
contracts, and active permission artifacts come from `curator.url`. Duplicate
executors and unknown YAML fields are rejected. `plan` discovers, assembles and
simulates without signing or publishing orders. Partial candidate failures are
logged to stderr in strategy order while healthy calldata remains on stdout.
`run` continuously drives transaction and external settlement work until stopped;
stopping discards RAM tracking when the process exits.

When configured, `/healthz`, `/readyz` and `/metrics` expose process health and
keeper outcomes. Readiness reflects the latest iteration. `version` prints build
metadata. There are no offline checkpoint commands.

## Execution

One discovery cycle groups executor metadata, account roots, event evidence and
canonical checks. Providers can fill authorized inputs or masked calldata only.
Independent getters use bounded JSON-RPC batches and explicitly safe Multicall3
reads; numerical dependencies use caller-correct RPC-only lenses. Stateful recipe
effects always execute in order.

The planner packs same-executor multicalls and, when the caller already supports
it, delegated cross-executor atomic batches. It verifies numerical witnesses,
permissions and the exact final unchanged-code transaction. Oversized execution
lists are packed into a verified prefix; remaining work stays eligible for later
cycles. Native repetition uses bounded growth probes rather than scanning every
prefix length.

Planning and signing share `txManager.maxGas` (default 16,777,216), bounded also
by the fresh block gas limit. Set an explicit YAML budget for other chain limits.
The sender estimates gas, applies bounded EIP-1559 fees and checks a fresh
sending-endpoint balance against the final maximum gas cost before signing.
The sending endpoint must serve `eth_getBalance`; invalid, unavailable or
insufficient funding leaves no signed transaction or active nonce ownership.
Signed work is held in RAM until canonical confirmation. Ambiguous sends retain the nonce
and rebroadcast identical signed bytes. There are no automatic fee replacements.
An unknown consumed nonce retains ownership until its exact receipt is found. After a
canonical successful receipt, native work finishes and optional external tasks
continue with their original order identity. External tasks reserve their account.
A confirmed reverted batch halts every selected executor until process restart.

Built-in integrations cover CoW orders, Liquid Lane consumer RFQ, Merkl proofs
and standalone claims, 3F standalone claims, and isolated Aave capacity probing.
Other native protocols execute entirely through their committed recipes. Liquid
Lane connector acquisition and calldata construction belong to vault-solver.
See [provider contracts and supported shapes](docs/PROVIDERS.md).

## Development

```sh
make check       # both modules: build, race tests, vet, lint, lens artifact checks
make integration # ephemeral loopback Anvil; no external chain or live credentials
make generate    # pinned abigen v2 bindings, wire types and RPC-only lenses
```

This checkout's daemon builds against its local `sdk/` through a module replace,
including container builds. External consumers pin `sdk/vX.Y.Z` releases. Runtime
APIs changed with this RAM rewrite; migrate callers rather than using old journal
interfaces. Remove `stateDir` from existing YAML; strict decoding rejects the
removed field. Existing state files are ignored and are not deleted automatically.

See [architecture](docs/DESIGN.md), [transactions](docs/TRANSACTIONS.md),
[SDK host ownership](docs/MERKLE-SDK.md), and [verification scope](docs/VERIFICATION.md).
