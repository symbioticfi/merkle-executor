# Generalized Merkle executor

The service executes curator-committed RecipesExecutor programs. Strategy decisions,
funding priorities, postconditions and asset limits remain in the on-chain recipe.
The service discovers that program, resolves its runtime inputs, verifies the current
permission package, simulates the exact transaction and manages its lifecycle.

## Repository alignment

Studied `../vault-solver` at `c5cae70`: Cobra entrypoints under `cmd`, generic
infrastructure under `internal`, isolated protocol packages, opaque YAML provider
configuration, Go 1.27.1, go-errors, logr/zap, generated abigen v2 bindings, and
context-aware I/O. The new service is independently buildable. It does not import
another module's internal packages or change the solver or adapters checkouts.

## Boundaries

- `recipes`: ABI codec, SetRecipe discovery, tree/role validation and runtime assembly.
- `provider`: integration-neutral registry, snapshots, runtime outputs and asynchronous progress.
- `providers`: protocol-specific adapters; generic static, RPC and HTTP data sources provide
  extension points for issuer claims, lending markets, 3F and future integrations.
- `keeper`: prepare, simulate, persist, send, reconcile external settlement and repeat.
- `state`, `signer`, `txmanager`: exclusive process ownership, durable writes and one nonce lane.
- `config`, `observability`, `cmd`: startup wiring, strict configuration and operational controls.

Provider output may fill numerical inputs or masked calldata fields only. It cannot
replace targets, masks, proofs, recipe instructions or caller identity. External data
does not grant authority. The final full execute simulation validates on-chain bindings
and postconditions; the keeper does not duplicate the Solidity arithmetic interpreter.

LL signed connector swaps execute inside a recipe. CoW authorization and settlement
are separate phases: submit the ERC-1271 order only after its authorization transaction
confirms, persist its deterministic UID and reconcile it before retrying submission.
Merkl reward proofs are protocol data, separate from account permission proofs. Claims
must fit explicitly approved bounded calldata shapes. Asynchronous issuer redemptions
use later claim recipes, configured independently from the request recipe.

## Lifecycle

Read recipe and active account root at a single pinned block. Verify configured chain,
account, caller, recipe hash, leaf hashes, sorted-pair proofs and executor roles. Preserve
all 12 existing recipe operations. Build each Manage/Approve runtime slot exactly once;
an explicitly skipped Manage slot remains empty and must pass full simulation.

Simulation-only operation requires no key. Sending is an explicit deployment mode.
Persist prepared jobs, then signed transaction bytes before broadcasting. Ambiguous
submission, timeouts, restarts and unconfirmed receipts retain nonce ownership. Only
canonical confirmed receipts advance workflows. Durable external phases block another
run of the same strategy until settled. No private keys, API tokens or complete provider
responses enter logs. Failed/unknown external states never become successful completion.

## Verification

Use contract-derived ABI/hash fixtures, generated binding checks, provider httptest
fixtures, crash/restart and nonce/receipt tests, and a local Anvil full-execute test.
Run build, race tests, vet and lint. Local validation does not establish production
readiness for an unconfigured issuer, exchange API or deployed permission tree.
