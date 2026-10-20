# Working agreement

The service is independently maintained; its architecture is in `docs/DESIGN.md`.

- Go 1.27.1, Cobra CLI, strict YAML, go-errors wrapping, logr structured logs,
  context-aware I/O and table-driven tests. Keep configuration values in YAML.
- Generic packages may not branch on protocol names. Native protocol calls, reads,
  gates, amount selection, claims, authorization and balance guards belong in the
  committed recipe. The isolated `sdk/providers/aave` provider may supply one
  authenticated borrow-capacity Input through its eth_call-only prefix probe;
  the recipe still caps the actual amount and enforces its health floor. Other
  protocol providers serve CoW orders, Liquid Lane HTTP RFQ and Merkl HTTP proofs;
  `keeper.NewRecipeInputs()` handles exact recipe constraints.
  Liquid Lane connector acquisition and calldata construction belong to vault-solver's
  Merkle extension. External signed terms come from RFQ/backend integrations and
  are validated by contracts; the daemon does not size connector swaps or verify
  their signatures.
- Never store secret values in configuration or logs. Use environment variable names.
- Contract interfaces are vendored under `api/abi` and generated with abigen v2.
  Regenerate artifacts instead of editing generated bindings. Protocol wire schema
  provenance and deliberate subset limitations belong in `api/schemas`.
- Runtime providers may fill authorized data only. Do not mutate recipe instructions,
  permission masks, targets or proofs to make a transaction succeed.
- A failed/ambiguous submission retains nonce ownership. Persist before network
  mutations; preserve external order identity through retries and restarts.
- Verify with `make check` and `make integration`; the latter uses an ephemeral
  local Anvil and no external chain. Report local, API-fixture and deployment
  evidence separately. Never use live signing credentials for tests.
