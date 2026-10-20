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
persisted numerical dependency before signing. Unknown and unresolved Inputs fail
closed; no protocol selector or deployment catalogue supplies missing native data.

Native issuer fixtures exercise committed claim/read/request recipes, atomic
balance guards, independent request/claim lifecycles and crash recovery. Source and
compiler provenance for the Noon handler stay with the test fixture, outside
production ABI generation. [Native validation and limits](NATIVE-ISSUER-TESTING.md).
Canonical confirmation settles generic native execution; these tests do not attest
a provider-specific request, payout or pending issuer obligation.

## Permissions and recovery

Curator fixtures cover pinned account/caller discovery, exact adapter/root/chain
association, full sorted-bundle reproduction, every leaf and proof, bounded HTTP,
redirect rejection, cancellation and canonical closure. Invalid permissions and root
changes prevent signing. Saved-attempt recovery and external-order advancement use
persisted identity and canonical evidence without a new curator fetch.

Transaction and keeper regressions cover persistence before every broadcast,
ambiguous sends, same-nonce fee replacements, an older owned variant winning,
foreign nonce consumption, cancellation, canonical confirmation, receipt reorgs,
late receipt recovery and configuration fingerprints. An uncertain repeatable
recipe retains its job and nonce ownership. Timeout or restart does not authorize
replay. Durable keeper progress precedes journal acknowledgement.

## Off-chain providers

Only CoW, Liquid Lane RFQ and Merkl have protocol-specific runtime providers.
Their local HTTP/RPC fixtures validate request framing, numerical policy, identity,
bounds, redaction, cancellation and recovery. Independent typed-data/proof vectors
establish local encoding, not live API acceptance.

CoW tests require confirmed account authorization before order publication,
preserve deterministic order identity through ambiguous submission/restart, and
require canonical full-fill or expiry evidence. Merkl tests verify cumulative
amounts and proofs against the effective distributor root and recipient routing;
a claim can feed a later recipe balance read through the shared lens.

RFQ tests preserve the saved Request/quote identity across publication ambiguity,
paginated lookup and restart. They reject API-only filled status, contradictory
cancellation and incomplete or overly dense event evidence. The local Reactor
fixture verifies canonical input/output transfers and nonce usage; its protocol
authentication is simplified. [RFQ fixture limitations](../internal/providers/liquidlanerfq/testdata/README.md)
and [wire provenance](../api/schemas/LIQUID_LANE_RFQ.md) record that boundary.

## Generated contract sources

The core ABI source is adapters `codex/bailsec-normalized` at
`0427c657375512a87158010c323556739fe96d1b`. The account ABI includes
`addCapability(address,bytes32)` and recipe operations remain 0–11. The source pin
in `sdk/abi/source.json` requires the exact clean checkout and validates compiled
source/dependency hashes before copying core ABIs and the local executor fixture.
Protocol ABI generation covers only the three off-chain providers and Multicall3.
Native issuer artifacts are local test fixtures.

The host consumes an immutable published SDK revision from `go.mod`. Verification
must identify that resolved revision and distinguish it from an optional local
workspace build. No deployment or hosted CI result is implied by generated-source
parity or local Anvil success.
