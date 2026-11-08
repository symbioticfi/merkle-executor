# Verification scope

Run the repository gates against the exact revision being reviewed:

```sh
GOWORK=off make check
GOWORK=off make integration
make generate
python3 sdk/recipes/lens/generate.py --check
python3 cmd/merkle-executor/testdata/issuers/noon/generate.py --check
git diff --check
```

`make check` covers host and SDK builds, race tests with coverage, vet and lint.
`make integration` uses owned ephemeral Anvil processes, generated local credentials
and HTTP fixtures. A passing local gate does not establish hosted CI, production
API availability, deployed permissions, live settlement or deployment readiness.
Report actual gate results and the exact revision separately from this description.

## Generic recipe execution

The local application and SDK suites use the real RecipeExecutor artifact with
permission-enforcing account/token fixtures. They cover event discovery, the complete
permission bundle and proofs, all 12 operations, packed operand bounds, full uint256
Inputs/results, bindings, arithmetic, approvals and cleanup, direct calls and skipped
gates. Independent encoding/hash vectors check commitments, selectors and event
topics. High operand bytes and trailing ABI bytes remain part of the committed and
submitted calldata.

The numerical lens tests cover caller/origin context, prior authenticated effects,
exact constrained Inputs, zero short-circuits, arithmetic errors and original-code
introspection divergence. The original full simulation must reproduce every
prepared numerical dependency before signing. Unknown and unresolved Inputs fail
closed. Explicit owning providers may fill only their declared authorized runtime
fields; generic input resolution has no protocol selector or deployment fallback.

Native issuer fixtures exercise committed claim/read/request recipes, atomic
balance guards, independent request/claim lifecycles and process-local confirmation.
Source and compiler provenance for the Noon handler stay with the test fixture, outside
production ABI generation. [Native validation and limits](NATIVE-ISSUER-TESTING.md).
Canonical confirmation settles generic native execution; these tests do not attest
a provider-specific request, payout or pending issuer obligation.

## Permissions and RAM ownership

Curator fixtures cover pinned account/caller discovery, exact adapter/root/chain
association, full sorted-bundle reproduction, every leaf and proof, bounded HTTP,
redirect rejection, cancellation and canonical closure. Invalid permissions and root
changes prevent signing. Pending signed attempts and external-order tasks retain
their exact identity in RAM without a new curator fetch.

Transaction and keeper tests cover capture before broadcast, ambiguous sends,
identical-byte rebroadcast, bounded fee replacements, funding failures, deadlines,
foreign nonce consumption, cancellation, canonical confirmation, receipt reorgs,
account reservations, terminal revert and private settlement state. CLI tests check partial-failure diagnostics preserve healthy
calldata and strategy order. Runtime state disappears when the process exits;
there is no journal, restart recovery or acknowledgement handshake.

## Isolated providers

CoW, Liquid Lane RFQ, native issuer claims, Merkl, 3F claims and Aave capacity have isolated providers.
Their local HTTP/RPC fixtures validate request framing, numerical policy, identity,
bounds, redaction, cancellation and in-process retries. Independent typed-data/proof vectors
establish local encoding, not live API acceptance.

Native claim unit tests cover discovery, permission preservation and idle/error
classification. [Native issuer fixtures](NATIVE-ISSUER-TESTING.md) exercise host
planning and execution; they do not establish deployment or live settlement.

CoW tests require confirmed account authorization before order publication,
preserve deterministic order identity through ambiguous in-process submission, and
require canonical full-fill or expiry evidence. Merkl tests verify cumulative
amounts and proofs against the effective distributor root and recipient routing;
a claim can feed a later recipe balance read through the shared lens.

RFQ tests preserve the saved Request/quote identity across publication ambiguity,
paginated lookup and in-process retries. They reject API-only filled status, contradictory
cancellation and incomplete or overly dense event evidence. The local Reactor
fixture verifies canonical input/output transfers and nonce usage; its protocol
authentication is simplified. [RFQ fixture limitations](../sdk/providers/liquidlanerfq/testdata/README.md)
and [wire provenance](../sdk/schemas/LIQUID_LANE_RFQ.md) record that boundary.

## Generated contract sources

The core ABI source is adapters `codex/bailsec-normalized` at
`0427c657375512a87158010c323556739fe96d1b`. The account ABI includes
`addCapability(address,bytes32)` and recipe operations remain 0–11. The source pin
in `sdk/abi/source.json` requires the exact clean checkout and validates compiled
source/dependency hashes before copying core ABIs and the local executor fixture.
Protocol ABI generation covers the used provider subsets. Their source pins and
limitations are indexed in [schema provenance](../sdk/schemas/PROVENANCE.md).

The host builds the local SDK in this checkout; SDK-only checks run independently.
No deployment or hosted CI result is implied by generated-source parity or local
Anvil success.

## Dependency refresh

The host and SDK use Go 1.27.3, go-ethereum/abigen 1.17.10 and golangci-lint
2.15.0. CI uses Foundry 1.9.0 and the runtime image uses distroless Debian 13.
Both Go modules refresh their direct and indirect requirements; external consumers
pin an immutable SDK release. Re-run generation and the gates above
after updating either module. SDK simulation helpers and local fixtures use
Solidity 0.8.38. Imported protocol bytecode retains its original compiler and
source revision so the reference contracts remain unchanged.
