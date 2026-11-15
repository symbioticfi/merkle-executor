# Shared SDK and host ownership

`github.com/symbioticfi/merkle-executor/sdk` is an independently versioned module.
External hosts pin immutable `sdk/vX.Y.Z` releases. This repository builds its
daemon against `./sdk` so local, CI and container checks use the same source.
`make check` and `make integration` check both modules; the SDK also works alone.

The host supplies RPC/HTTP transports, signing credentials, provider registration,
logging/metrics and process scheduling. SDK imports and constructors never read environment values or start workers.
Each solver process configures its own signer and `transaction.Manager`, initializes
it and explicitly runs `Start`. Both services reuse the same SDK lifecycle through
`Send`/`SendAsync` or the `manager.Sender(name)` keeper view. The host and its
providers select existing orders or claims; the transaction engine handles their
chosen calls without domain-specific discovery or prioritization.

The host calls `Keeper.Tick(context)` repeatedly and can inspect `Pending()` during shutdown. `keeper.Planner` groups discovery and final validation
across executors. `Prepare` prepares one strategy; `PrepareExecution` selects one
atomic direct, same-executor or delegated envelope. Prepared bytes go unchanged
to sender admission. Supply one shared budget to `Planner.MaxGas` and
`transaction.Config.MaxGas` (zero defaults to 16,777,216); the fresh block gas
limit also caps admission. Configuration must stay fixed for a Keeper's lifetime;
strategy names must be nonempty and unique so settlement ownership is unambiguous.

At the recipe layer, use `recipes.DiscoverBatch` followed by
`recipes.ValidatePreparedBatch` for both one executor and many. One executor uses
one-element slices. The standalone discovery, preparation and validation helpers
are removed, along with `PreparationOptions.VerifySimulation`; supply numerical
witnesses to the shared validator. Both sizes receive the same commitment,
identity, root, leaf-role, simulation and canonical-block checks.

Providers implement `RecipeRequirements` and `PrepareRecipe`. They receive only
an authenticated recipe/tree and snapshot. Outputs contain authorized inputs,
masked actions, witnesses and optionally a process-local `provider.Task`.
Stateless providers return no task. A task implements
`Advance(context.Context, provider.Snapshot) (provider.Progress, error)` and owns
its typed progress in RAM. It runs only after canonical authorization, retaining
exact order identity across in-process retries. Account reservations last until
all tasks finish. Prepared plans are process-local and cannot be serialized into
restart recovery.

Hosts may register `providers/claims.NewAutomatic` for standalone native issuer
claims. [Provider behavior and supported shapes](PROVIDERS.md#native-issuer-claims)
describe this stateless integration; generic admission and full simulation still apply.

`keeper.Planner.PermissionSource` supplies permission artifact bytes, and the SDK
validates every leaf/proof plus chain/account/root identity itself. The built-in
source resolves the pinned account adapter and retrieves the backend certificate.
SDK hosts can instead use a local immutable tree file. Neither source can bypass
closing metadata, role, simulation or canonical guards.

Liquid Lane connector acquisition and calldata construction, and host-specific
3F auction feeds/sizing, remain in vault-solver. Contract validation of external
signed terms remains authoritative. This repository does not change or migrate
other repositories automatically.

Transaction ownership, fee replacement, deadlines, funding and canonical receipts
are implemented once in `sdk/transaction`. Host shutdown stops intake and drains
the shared manager within its configured bound. There is no disk state or restart
recovery. See [transaction ownership](TRANSACTIONS.md).
