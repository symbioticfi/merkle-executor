# Protocol contract interfaces

Core interfaces and provenance live in `sdk/abi`.

This directory contains only the ABI subsets needed by the CoW, Liquid Lane RFQ, connector discount
and Merkl off-chain providers and their Multicall3 transport. Sources and deliberate
subset limits are documented in [schema provenance](../schemas/PROVENANCE.md).
Native protocol calls and reads are encoded in committed recipes; issuer artifacts
used by local integration tests live under `cmd/merkle-executor/testdata/issuers`.
