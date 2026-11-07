# RAM-only runtime migration

This runtime deliberately keeps all workflow, transaction and external-order state
in RAM. The earlier durable v2 design is superseded: there is no state directory,
journal, importer or restart recovery. Existing state files are not read, modified
or deleted. A new process cannot resume work described by them.

Before switching an existing deployment, account for outstanding transactions and
orders with the old runtime. Stopping a process does not cancel an on-chain
transaction or revoke an external authorization. The new process may prepare the
same recipe again because it has no knowledge of earlier submissions. Keep one
process per dedicated signer; there is no cross-process file lock or reservation.

## YAML and CLI

Remove `stateDir`. Rename old `txManager.maxFeeGwei` to
`transaction.maxFeeGwei`. Remove `transaction.horizon`, `replacementInterval`,
`pendingTimeout` and transaction-level `shutdownTimeout`. The sender uses a fixed
`priorityFeeGwei`, `gasHeadroomBps`, `confirmations`, `pollInterval`,
`broadcastTimeout`, `readTimeout` and `estimateTimeout`. Send mode requires a
finite positive maximum fee covering the priority fee.

The strict user file overlays [YAML defaults](../internal/config/defaults.yaml).
Removed fields are errors. Runtime shutdown timing remains under `runtime`.
`status` and `reset-strategy` are removed; there is no offline runtime state to
inspect or reset. `check-config`, `plan` and `run` remain. Use observability
endpoints or the service's copied `OnSnapshot` callback for current-process state.

## Go SDK

- `runtime.Runtime` owns RAM maps directly; `Store`, preflight, import/migration,
  durable deployment identity and `sdk/state` are removed.
- `runtime.Config` has no recovery revision/fingerprint, and strategies have no
  legacy configuration identity.
- `runtime.Driver` has only `Execute(context.Context, Intent) TransactionResult`.
  Transaction journals, persistence callbacks, Reserve/Release and Resume are removed.
- `Workflow.Execution` holds canonical evidence directly; there is no serialized
  transaction or reservation field. Snapshot returns independent copies.
- Stable integration operations, assignment validation, recipe authorization and
  shared read scheduling remain. Protocol records are kept in RAM through live
  reconciliation; their Restore helpers validate those records without disk I/O.

Hosts must keep an admitted sender alive across individual request cancellation.
Only conclusive canonical evidence or a definitely pre-broadcast failure releases
its lane during that process. There is no cross-restart guarantee to migrate.
