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

## Durable execution

`runtime.Runtime` owns one atomic v2 `Workflow` per executor. It contains immutable
plan/assignments, stable operation identities, external progress and the complete
transaction reservation and signed variants. Startup validates the whole inventory
and rebuilds account/signer indexes before admitting work. Tick does not rescan
historical storage for each strategy. Pending external reconciliation can progress
while another workflow occupies the signer. Account ownership prevents conflicting
work; ready selection rotates fairly.

The runtime persists a reservation before admission. The process-owned transaction
worker outlives an individual Tick timeout. `internal/transaction.Driver` owns one
signer lane, reads the write endpoint's mined nonce, estimates gas, applies EIP-1559
horizon fees, persists each verified signed attempt before broadcast and observes all
same-nonce variants. Ambiguous submission keeps ownership and may rebroadcast exact
bytes. Replacement preserves intent and nonce. A canonical receipt from any owned
variant can win; timeout, cancellation, missing receipts and foreign nonce consumption
cannot prove failure. Signed restart recovery observes saved hashes without signing
or broadcasting. Workflow progress is durable before lane release.

Providers advance only with canonical confirmed execution evidence and fresh snapshots.
External order IDs, publication state and scan cursors survive retries/restarts. API
status alone cannot prove a CoW or Liquid Lane fill. Issuer reconciliation matches exact
managed calls and payment evidence. Exclusive managed-target claims include completed
operations and reject ambiguous static/interleaved attribution. Request completion is
`requested`, permitting a later independently configured claim recipe; it is not payout.
Protocol support limits and wire provenance remain in `docs/PROVIDERS.md` and `api/schemas`.

Storage uncertainty poisons further admission until restart/reload. Runtime drain must
finish before scheduler, RPC and storage close. A failed drain retains process ownership.
Secrets are environment-variable names in YAML; values and remote payloads never enter
logs. See [transaction policy](TRANSACTIONS.md) and [migration](MIGRATION.md).

## SDK and verification

The independently versioned SDK contains neutral chain I/O, recipes, operations,
planning/coordinator contracts and local state primitives. It creates no host signer
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
