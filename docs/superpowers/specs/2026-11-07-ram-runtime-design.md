# RAM runtime

The user approved direct replacement of the daemon and SDK runtime APIs on
2026-11-07 and instructed execution. Keep Go and the existing contract interfaces.

## Intent and constraints

Replace runtime orchestration from zero, remove redundant machinery, and make
execution and integrations consistent. All runtime state lives in typed Go values
in RAM. Configuration and committed contract artifacts remain readable files.
There is no journal, database, file lock, replay format or restart recovery.

Recipe bytes remain the authority for calls, reads, amounts, gates and limits.
Keep generated ABI bindings, strict permission validation, numerical lens,
authenticated provider ownership, payload masks and canonical chain guards.
Generic packages never branch on protocol names. SDK construction starts no
background process and reads no credentials or environment.

## Approaches

Replacing disk with a memory Store retains obsolete serialization and recovery.
Removing every transaction lifecycle loses ownership after ambiguous submission.
The selected approach replaces those layers with one RAM scheduler and one small
sender, retaining ownership only for the process lifetime.

## Runtime

The scheduler owns an immutable in-flight execution, per-executor due times,
terminal failures and optional external settlement tasks. A successful canonical
receipt fans out directly in RAM. Stateless integrations finish immediately.
External tasks retain exact order identities and lookup cursors; their account
remains reserved until settlement. Preparation never publishes external orders.

The standalone sender owns at most one signed transaction. It estimates gas,
uses bounded EIP-1559 pricing, saves the signed bytes in RAM before broadcast,
rebroadcasts those exact bytes, and returns canonical receipt evidence once.
No adaptive fee horizon, replacement ladder, persistence callback or acknowledge
handshake remains. An ambiguous submission, missing receipt or consumed nonce
without an owned receipt retains ownership. Existing pending work prevents fresh
admission. Hosts can inject an existing sender through the same small interface.

## Preparation and batching

Open one pinned discovery cycle for candidate executors. Group independent
metadata, roots, events, event headers and final role/metadata reads across the
cycle. Deduplicate identical permission acquisition within the exact account/root
scope. Provider requirements are authenticated before acquiring runtime data.
Resolve numerical dependencies through the existing caller-correct prefix lens.

Preserve ordered same-executor multicalls and delegated cross-executor atomic
batches. Result probes preserve sequential effects. Final admission always runs
the exact unchanged-code envelope, validates numerical witnesses and closes
canonical guards. Failed transport batches never fall back to individual calls.
Gas-bounded packing defers claims that do not fit. Repeat selection must avoid
replaying prefix lengths 1 through N.

## Migration and acceptance

Remove stateDir, state/checkpoint commands, fingerprints, journal types and old
transaction packages. Point this repository's daemon at its local SDK, including
container builds; SDK consumers still import the independently versioned module.
Update architecture and operation documentation rather than retaining obsolete
recovery claims. Do not change other repositories or delete existing state files.

Restart loses pending transaction/order tracking, reservations, timers and
terminal halts. A transaction or external order can continue after process exit;
there is no restart exactly-once guarantee. One active owner per signer is an
operating requirement. Migration starts after old work has settled.

Verify RAM lifecycle, ambiguity, reorg/depth, immutable identities, account
reservations, atomic failure, batching request counts and all existing recipe
operations. Run make check and make integration with ephemeral local Anvil only.
Report local/API-fixture evidence separately from deployment evidence.
