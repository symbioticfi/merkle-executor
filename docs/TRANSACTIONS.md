# Transactions in RAM

`transaction.Manager` owns one signed EIP-1559 transaction. It starts no worker
and requires no store. The host calls `Submit`, schedules `Poll`, and checks
`Busy`. A host with a shared sender implements `keeper.Sender` and must serialize
that signer across all its jobs.

Submission validates the destination, estimates gas and reads mined/pending
nonces from the write endpoint immediately before signing. Existing pending work,
a stale mined nonce below the confirmed floor, or inconsistent counts prevent
admission. The signer must preserve the exact chain, nonce, destination, calldata,
value, gas and fee payload. Signed bytes enter RAM before broadcast.

Fees use a one-block median priority reward (minimum one wei), with a fee cap of
`min(2*baseFee+tip, txManager.maxFeeGwei)`. If base fee plus tip exceeds the ceiling,
no transaction is signed. Gas receives ten percent headroom, bounded by the block
gas limit. The fee ceiling must be finite and positive. There is no fee-horizon
model, replacement ladder, cancellation transfer or abandonment timer.

A send error can follow a successful node acceptance. `Busy` remains true after
signing even on timeout or cancellation. `Poll` checks the owned hash, verifies
canonical ancestry and configured confirmation depth, and returns its exact
receipt identity and logs once. Missing receipts trigger identical signed-byte
rebroadcast. A higher mined nonce without an owned receipt retains ownership;
it does not prove whether the recipe ran. RPC errors and unconfirmed/reorganized
receipts never become terminal success.

The keeper retains the matching immutable envelope until terminal evidence.
Canonical success creates optional external settlement tasks. A canonical revert
halts every selected executor for this process; an atomic batch is never partially
resent. There is no acknowledgement handshake or second receipt journal.

The daemon polls explicitly and continues running until its context ends.
Cancellation can leave live work outside the process. No background drain, disk
checkpoint or recovery occurs.

Restart loses hashes, order identities and reservations. An old transaction can
still mine, and an authorized external order can still fill. Fresh nonce reads
do not reconstruct those workflows. Operate one active process per signing
account and settle old work before restarting or upgrading. Old journal files are
ignored, never migrated or automatically deleted.
