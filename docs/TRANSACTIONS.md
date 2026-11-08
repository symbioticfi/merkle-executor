# Transaction lifecycle in RAM

Both services use `sdk/transaction.Manager`. Each process supplies its signer,
RPC backend, YAML configuration and logger, then explicitly calls `Initialize`
and runs `Start`. Each solver process is configured with its own signer; this is
trusted deployment configuration, not a runtime restriction. Constructors start
no workers. All transaction state stays in RAM.

Solvers and providers discover and prioritize orders or claims. The manager
receives the chosen destination and calldata through `Send` or `SendAsync`.
Requests may carry a business deadline, a stricter fee ceiling and an
obsolescence check. Results contain signed hashes, an error and, only after
canonical confirmation, a receipt. `Included`, `Reverted` and `Uncertain` derive
facts from that evidence; an error alone never proves a signed call did not land.

Recipe keepers use `manager.Sender(name)`, a `Submit`/`Poll`/`Busy` view of the
same worker. The keeper retains its immutable plan while the sender ticket is
busy. A canonical receipt authorizes settlement; a canonical revert halts the
selected recipe members. If an older owned call wins the same nonce, the newer
plan is discarded without settlement and must be freshly prepared.

## Signing and fees

Before each signature, the manager checks the business deadline, optional
obsolescence callback, current nonce, gas estimate and sending-endpoint balance.
Funding must cover the final maximum gas cost. The signer must preserve the
exact unsigned payload and configured identity. Every signed original and
replacement is recorded before broadcast, including ambiguous and rejected
responses. Failures after signing never erase that identity.

`Config.MaxGas` defaults to 16,777,216 and is also bounded by the fresh block gas
limit. Hosts give the same budget to planning and signing. Fees use recent market
tips and a cap of twice the base fee plus tip, bounded by `MaxFeeGwei` and any
stricter request cap. `MaxFeePerGas` exposes the same pricing for solver economics.
At `ReplacementInterval`, replacements increase both fee fields by at least
12.5% over accepted fees, with fresh gas and funding checks. Rejected candidates
do not raise the accepted fee floor. Exact rebroadcasts preserve signed bytes.

## Ownership and receipts

One worker owns one unresolved nonce and its signed variants. The caller's
operation deadline bounds the first signature; the request's business deadline
bounds later signing. Native expiry or obsolescence returns an uncertain result
while retaining signed attempts. A freshly validated native request can replace
that nonce. A keeper ticket instead stays busy and observes its signed work
through expiry, without further signing.

Every retained hash is checked. Receipt evidence must match the owned hash,
canonical inclusion block, ancestry and configured confirmation depth, including
for reverts. Missing or reorganized receipts and foreign nonce advancement do
not prove execution. A winning older attempt is never credited to a newer
request. The manager retains at most 256 signed variants at one unresolved nonce
and stops signing at that bound rather than evicting identities.

`PollInterval` defaults to two seconds, `ReplacementInterval` to 30 seconds and
`ShutdownTimeout` to one minute. Shutdown stops admission and drains signed work
within that bound before the host closes RPC dependencies. Hosts own metrics
and tracing; the SDK has no observer callbacks or background telemetry workers.
A result returned before eventual inclusion may therefore have no fill metric.

Restart loses hashes, order identities and reservations while external work may
continue. Settle external work before restarting or upgrading. There is no
serialization or restart recovery.
