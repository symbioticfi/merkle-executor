# Local RFQ consumer fixtures

Run `python3 internal/providers/liquidlanerfq/testdata/generate.py` with the cached
solc 0.8.37 toolchain. Artifacts record source SHA-256 and compiler version, use
Osaka, optimizer 200 runs, and disable the metadata bytecode hash. Generation uses
a temporary directory and never writes sibling repositories.

`FixtureRFQAccount` reproduces permission proofs, roles and Manage dispatch from
the existing local account fixture, plus the hash program, original-buffer Sign
and ERC1271 behavior from SDK-pinned adapters revision
`bd5e235899de5e175a0001a913d47596f6f4f801`. It retains the `root()` interface.
`FixtureRFQReactor` reproduces Request EIP712, nonce use, strict deadline, atomic
ERC20 input/output transfers and Fill/InvalidateNonce tuples from RFQ revision
`22b04a850ca3680103e797bfae0e1a3b997fac83`. The protocol/filler caller substitutes
for the upstream protocol signature; factory registration, adapter execution,
protocol authentication and production administration are not modeled.
`FixtureRFQToken` is an ordinary exact-transfer test ERC20.

All setup methods deliberately lack production access control. These contracts
must never be deployed as production implementations. The daemon test uses the
real committed RecipeExecutor artifact, a local Multicall3 deployment, backend-shaped
HTTP permissions and RFQ responses, and a randomly generated key funded only on an
ephemeral Anvil. It proves local authorization, token movement and recovery behavior,
not production API availability or live protocol acceptance. See
[wire and contract provenance](../../../../api/schemas/LIQUID_LANE_RFQ.md).
