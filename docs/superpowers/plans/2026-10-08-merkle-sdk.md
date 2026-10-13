# Merkle SDK and vault-solver host integration

> Execute inline using executing-plans. The user's approved architecture and this document are the spec.

Goal: one reusable Merkle execution implementation, independently versioned under sdk/go.mod, consumed by the standalone daemon and vault-solver. Services retain their own configuration, protocol providers, telemetry and deployment. Vault-solver retains its one shared transaction manager.

Constraints: SDK imports no daemon package and has no signer creation or fee/nonce engine. Hosts submit signed transactions only after SDK journal persistence. Uncertain work retains an exclusive host lane through restart; acknowledgement follows durable business progress. Never rebroadcast recovered non-idempotent recipes. Tree/proof/role checks, exact prepared calldata, simulation and canonical receipt evidence are shared. No duplicated protocol providers, legacy dependencies, or live signing tests. Existing LL/3F auction integrations remain host business logic; extraction must not claim new auction coverage.

## Task 1: Public module extraction
Move recipes, core generated bindings and their ABI provenance, runtime provider contracts, protocol IO, durable store and generic keeper/planner/tests into sdk. Replace daemon config with SDK Workflow configuration carrying a host revision fingerprint. Update daemon imports and construction. Add an isolation gate and explicit SDK build/test/vet/lint/integration targets. Verify SDK builds independently and standalone tests still pass.

## Task 2: Sender-independent durable execution
Move journal implementation into sdk/submission; inject a Transport with SendAsync, Hold, Release and RestoreConfirmedNonce. Outcomes expose only signed-attempt identity and definite rejection; fees and nonce logic stay in hosts. Persist signed originals/replacements through a callback before transport broadcasts. SDK verifies canonical ancestry itself. Keep standalone adapter thin, preserving its existing engine integration tests. Add journal tests for persistence errors, changed payload, restart and late receipt recovery.

## Task 3: Vault-solver consumption
Add generic request BeforeBroadcast and Owner hooks and exclusive lane reservation to its existing manager, including concurrency/replacement/failure tests. Add the Merkle solver integration using SDK planning, keeper, journal and store; configure bounded inputs and run send or simulation modes. Startup recovery reserves the shared lane before any solver starts. Shutdown drains the manager before closing SDK storage. Document host provider extension points and the LL/3F business integration boundary. Pin the SDK to its published immutable commit, with no local replace directive.

## Task 4: Verification and delivery
Run standalone and SDK make check/integration; vault-solver format/build/race tests/lint and transaction Anvil tests. One fresh-context whole-diff review, fix substantive findings and rerun relevant gates. Update existing Merkle PR to main, create vault-solver PR to stage and attach it. Report local and hosted evidence separately.

Review focus: single signer lane, reservation admission race, every replacement persisted before RPC, immutable request identity, restart without replay, terminal evidence reorg validation, storage ownership and shutdown ordering, SDK dependency isolation and clean consumer build.
