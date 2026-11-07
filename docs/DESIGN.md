# Runtime architecture

The executor runs curator-committed RecipeExecutor programs. The committed recipe
owns decisions, funding priorities, postconditions and asset limits. Integrations
supply authorized numerical inputs and masked calldata fields. They cannot change
instructions, targets, permission masks, proofs, native value or caller identity.

`internal/service` composes one process: strict YAML policy, read/write RPC clients,
one bounded read scheduler, registered integrations, one planner, one runtime and
one transaction driver. `cmd/merkle-executor` only adapts CLI arguments and output.
Protocol selection belongs to startup registration; generic SDK packages do not
branch on protocol names.

## Preparation

`provider.Integration.Compile` receives a copied authenticated program and emits
independent `Operation`s with stable IDs and complete numerical/action/exclusive
ownership requirements. Compilation is local and runs once per attempt. Registration
order does not define identity. Each issuer route owns its operation; linked request
and consume actions can share one operation.

`runtime.Planner.PrepareAll` shares an initial head across eligible executors and
uses bounded workers. Each attempt validates executor metadata, committed recipe,
permission-tree leaves/proofs, chain/account/root and execution roles. Operations
without numerical dependencies prepare first. The planner combines their effects
with intrinsic recipe-input requirements, resolves a union of numerical dependencies
through the lens, then prepares dependent operations. Final assembly retains exact
committed query/step bytes. Full caller-bound simulation verifies result witnesses,
postconditions and executable work, followed by fresh canonical closing guards.
Presend validation repeats live metadata, role, simulation and canonical checks.

All integration readers and the planner use the same `chainio.Scheduler`. Independent
ready read groups from executors and routes share bounded partitions. Deduplication
requires an immutable numbered block and identical complete execution context.
Latest/pending reads are fresh. Caller-sensitive calls, full simulations and code
overrides retain sender, value, gas, block and override semantics; they never gain
Multicall authority. Only explicitly safe getters use Multicall3. Actual batch failures
never silently retry as individual calls. Member failures stay lazy until consumed.
Count, request/response bytes, calldata, output, gas, queue and concurrency budgets
come from YAML. Cancellation must preserve live owners while releasing dead work.

The recipe cache is a bounded indexed FIFO of immutable event evidence keyed by
chain/executor. A hit still authenticates fresh commitment, event block/hash and
structure. All mutable metadata, roles, roots and simulations remain fresh. Only
successfully closed attempts enter the cache; callbacks receive independent copies.

## RAM-only execution

`runtime.Runtime` owns process-local workflow and account maps plus one sender worker.
There is no storage interface, state directory, journal, importer or startup recovery.
Startup validates only the current configuration and initializes empty maps. Snapshots
are independent copies for in-process observers. Ready selection rotates fairly;
external reconciliation can progress while an unrelated transaction is pending.

The runtime puts an immutable plan and ownership into RAM before starting its worker.
The worker outlives an individual Tick timeout. `internal/transaction.Driver` reads the
write endpoint's mined nonce, estimates gas with YAML headroom, verifies exact signer
output, broadcasts one EIP-1559 attempt and observes its canonical receipt. YAML fixes
the maximum fee and priority fee. There are no replacements, rebroadcasts or restored
signed attempts. An ambiguous submission retains the signer lane while the process
lives. Missing receipts or a consumed nonce cannot authorize a second transaction.
A canonical revert halts that strategy and frees the lane for unrelated work.

Confirmed execution evidence and protocol progress remain in the workflow's RAM.
Providers advance with fresh canonical snapshots and exact call/payment evidence.
API status alone cannot prove a CoW or Liquid Lane fill. The prepared external order
identity is reused through in-process lookup/POST retries; completed operations keep
exclusive attribution claims. Issuer requests finish as `requested`, allowing a
separately configured claim recipe when the issuer is ready.

Process exit discards all transaction, order and workflow knowledge. A new process
prepares from the current chain/API state and may repeat previously authorized work;
there is no restart idempotency or cross-process signer/account lock. Keep one process
per dedicated signer. Shutdown cancels and drains its worker before closing RPC and
the shared scheduler. Protocol limits and provenance remain in `docs/PROVIDERS.md`
and `api/schemas`. See [transaction policy](TRANSACTIONS.md) and [migration](MIGRATION.md).

## SDK and verification

The independently versioned SDK contains neutral chain I/O, recipes, operations,
planning and RAM coordinator contracts. It creates no host signer
or fee engine. The daemon uses `replace .../sdk => ./sdk`, so local checks and container
builds use the checked-out source. Published consumers pin immutable SDK releases;
this rewrite does not alter sibling repositories.

Recipe operands/commitments remain contract-derived: queries/steps are `bytes[]`,
commitment is `keccak256(abi.encode(queries,steps))`, and execute accepts those bytes,
`uint256[]` inputs and `bytes[]` runtime slots. All twelve recipe operations and exact
trailing ABI data are preserved. Bindings are generated with abigen v2 from vendored
interfaces; protocol schema provenance documents deliberate subsets.

Run `make check` and `make integration`. Integration uses ephemeral local Anvil and
fixture APIs with generated test credentials. Local, API-fixture and deployment
evidence are separate; local success does not establish deployed protocol readiness.
