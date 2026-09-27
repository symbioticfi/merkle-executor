# Working agreement

Match the conventions researched in `../vault-solver/CLAUDE.md`; the architectural
comparison is in `docs/DESIGN.md`.

- Go 1.27.1, Cobra CLI, strict YAML, go-errors wrapping, logr structured logs,
  context-aware I/O and table-driven tests. Keep configuration values in YAML.
- Generic packages may not branch on protocol names. Register providers at startup;
  protocol details belong under `internal/providers/<name>`.
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
