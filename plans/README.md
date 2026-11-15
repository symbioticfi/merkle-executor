# PR 39 follow-up implementation

Planned at `fd35d773278b14a774487f84fc03c83f3e920bd9`, 2026-11-08.
The user requested implementation of all three findings. The optional status and
foreground-drain features were subsequently removed at the user's request.

| Plan | Scope | Priority | Depends on | Status |
|---|---|---|---|---|
| 001 | Shared planning/signing gas budget | P1 | none | DONE |
| 002 | Lazy Merkl runtime-token reads | P2 | none | DONE |
| 003 | CLI planning diagnostics | P2 | 001 API wiring | DONE |

Implemented with parallel executors and reviewed as a combined diff.

## Verification

- `make check` passed: host and SDK builds, race tests, vet, generated lens checks and lint.
- `make integration` passed against ephemeral local Anvil and API fixtures, including a transaction whose 10% gas headroom formerly exceeded the cap.
- Focused race tests cover lazy Merkl selection and partial CLI planning diagnostics.
- Independent review found no actionable introduced issue. `git diff --check` passed.
- No external-chain execution, live signing credentials or deployment was used.

## Design

- One YAML `txManager.maxGas` budget feeds planning and signing. Use a conservative default of 16,777,216; an explicit deployment value permits other chain rules. Bound by fresh block gas too. Never release ownership after signing to compensate for bad admission.
- Keep candidate failures visible in `plan` while retaining useful healthy calldata.
- Retain Merkl getter deduplication, but avoid reading unvisited runtime-token candidates. Standalone all-reward inventories may retain grouped reads.

## Considered and rejected

- RAM-only restart loss, missing automatic replacement and unknown-nonce ownership retention are documented design choices.
- A rare idle prefix during fitting can defer a later executor one iteration; no persistent starvation was established.
- Broad protocol abstraction or new persistence would violate the requested minimal scope.
