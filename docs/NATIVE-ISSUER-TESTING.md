# Native issuer execution validation

A read-derived Centrifuge or Midas amount belongs to the committed recipe. The
daemon validates its catalogue identity, account, amount provenance and permission
before allowing native execution. It does not prepare issuer inputs or take
transaction-wide receipt ownership for these calls. This permits claim, fresh
read and request steps against the same issuer in one transaction. Input-backed
routes still require exact selection, readiness and exclusive receipt ownership.

## Portable regression tests

`make check` and `make integration` cover the root module and nested SDK. The root
uses its published SDK module version with `GOWORK=off`; SDK checks also run
independently. Integration tests start owned, unforked Anvil processes and use
local fixtures, including code installed at catalogue addresses. They do not
establish mainnet issuer behavior.

- `TestAutomaticNativeCentrifugeComposition` and
  `TestAutomaticNativeMidasComposition` exercise repeated native calls without
  provider reads or receipt attribution.
- The companion rejection tests preserve account/receiver identity, read-derived
  budget validation and Input-backed exclusive ownership.
- `TestAnvilNativeRedemptionsAtomicityAndCrashRecovery` runs the actual daemon
  entry point in a separate process. It claims, observes the changed issuer
  allowance, requests again and skips a zero Midas fallback. A failing final
  guard rejects preparation; a deliberately mined failure rolls back balances,
  allowance, requests and logs. The test kills the daemon after its signed
  pending transaction is durable, removes a mined receipt, reincludes the same
  bytes and recovers the same job/hash from a different canonical block while
  the curator is unavailable. It also checks zero native gates become idle.
- Permission tests check scope mismatches, malformed and unused calls, complete
  bundle leaves, proofs, exact uint256 values, bounded HTTP responses, redirects,
  cancellation and adapter read failures. Their HTTP handlers require the
  compact `/merkle/{adapter}/{root}` route.

The native daemon test fails with `issuer receipt target or action is repeated`
when the native dispatch fix is removed. It passed three race-enabled repetitions;
the complete daemon Anvil suite also passed with the race detector.

## Additional mainnet-fork exercise

On 2026-10-20, a separate descendant of the populated local mainnet fork was
anchored at block 26211639, hash
`0xd3f69c6d98f25e63511bb8644b451e2c57f559faded734997a8fe04c0f2d022c`.
The saved stack and keeper continued running on the original fork.

The real keeper handled two withdrawals, creating Midas redemption requests 15
and 16 for increased demand. Native settlement of request 15 pushed
9,239.019948 USDC to the account while request 16 remained pending. After a
90-second chain-time delay, the keeper repaid debt and returned collateral;
request 16 continued reserving its proceeds and an unchanged preparation was
idle. Health factor remained above 1.3. Issuer settlement used genuine forked
operator roles and issuer treasury funds, without changing prices or balances.

This exercise uses the saved runtime's long backend route and the previously
built daemon carrying the same native dispatch fix. The portable suite tests
the published compact-route SDK. The saved UI, indexer and NAV snapshots were
not upgraded by these tests. These additional withdrawals were verified through
adapter return; they are not evidence of a new accepted NAV report or final
recipient queue payment. Positive earned Merkl rewards and portable cold stack
restore also remain unverified.

The 60/40 RWA carry policy is an externally authored recipe. It is not a built-in
daemon strategy or frontend template. Native issuer minimum lots and full-exit
rounding must be tested in that policy independently of executor correctness.
