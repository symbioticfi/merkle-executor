# Offline recovery diagnostics

Design spike 007, 2026-10-16. Source reference: 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e.
Reviewed against local source d6690d0daded2cb6ab71f035b4124f22c141f438.
This document specifies a future command. It does not add a runtime command or API.

## Evidence and consistency

Choose a separate `merkle-executor diagnose --config PATH` command with Cobra
NoArgs. Keep existing status JSON and reset-strategy behavior byte-compatible.
No executor selector in v1: the single signer journal must be inspected alongside
all saved historical and configured strategy identities so an orphan owner cannot
be hidden by filtering. Do not normalize legacy strategy/job IDs into addresses.

The command loads and validates YAML but does not dial RPC, construct a signer,
load a signing key, construct submission.Manager, call state.Open, acquire the
writer lock, call any provider, acknowledge/reset/replay or write state. It uses
read-only atomic-file snapshots, so it works while the daemon holds its lock.

Map every reported field to a durable source:

| Report field | Authoritative local source | Limit |
|---|---|---|
| deployment.chainId/caller | deployment.v1, same checks as checkStateIdentity | No chain verification; mismatch blocks interpretation |
| strategy identifier/configured | union of strategies.v1 and configured executors | Legacy identifiers are retained exactly |
| jobId/phase | keeper.Job.ID/Phase in strategy.<identifier>.v1 | Validate known phase/nonempty identity; no inferred engine phase |
| finishedAt | Job.FinishedAt, only if valid and nonzero terminal timestamp | Active jobs have no creation time; report null, never infer age from mtime |
| journal owns job | exact match between validated record.ID and Job.ID | No lowercase/prefix identity substitution |
| attempts/hash count/nonce/chain/sender | validated submission.record Raw + Attempts | Derive hashes only after all attempt identities validate |
| saved terminal and winning hash/block | validated record.Terminal/Execution | Saved evidence can be reorganized; not a fresh canonical claim |
| saved job receipt hash/block | validated Job.Result.Execution/Hash | Keep distinct from journal evidence; mismatch is a blocker |
| progress | Job.Progress Done flags and safe outcome classification | Persisted provider claims, no live payment proof |
| live phase/current confirmations | no offline source | Always unknown/null |

Use a new pure SDK API rather than a host copy of the journal decoder. Proposed
signature in sdk/submission/inspection.go, not implemented in this spike:

~~~go
func InspectRecord(raw json.RawMessage, chainID *big.Int, sender common.Address) (JournalInspection, error)
~~~

JournalInspection exposes only validated ID, chain/sender, nonce, ordered attempt
hashes, required confirmation depth, saved terminal kind and sanitized terminal
block/hash/winning hash. It contains no raw bytes, arbitrary errors, Execution
object, destination/calldata or signatures. Refactor Manager.decode's exact current
checks into a private decodeRecord shared by Manager and InspectRecord. Keep
version=1, bounded confirmations, sender and chain identity, dynamic-fee type,
zero value, nonnil destination, same nonce/target/calldata across variants, unique
hashes and execution validation/terminal membership. Do not instantiate a manager
or call Transport.Hold merely to decode. SDK imports no host package. Diagnostic
errors are a finite typed code; unsafe underlying decoder text is never serialized.

The host reads raw journal JSON with state.Read into json.RawMessage, then calls
InspectRecord. Before shipping, add an additive state.ReadBounded helper with the
same atomic no-writer-lock semantics and a bounded LimitReader; use that in the
new command. Its record byte budget belongs in a strict YAML diagnostics section,
validated offline. Choose diagnostics.maxRecordBytes (int64): default 16 MiB
(16777216), valid range 1..64 MiB (67108864). Include it in config/example.yaml.
Measure representative existing records as an implementation verification gate;
if those cannot fit safely, replan explicitly rather than silently removing bounds. This configuration
addition is part of the future scope, not permission to change current config now.
An oversized record yields record_too_large, no decoder attempt and no raw output.

Read deployment, inventory, each referenced job and the journal into one pass;
reread the same files once and compare presence plus internal byte digests. If a
file changed, do not retry indefinitely: report consistency=mixed and blocker
mixed_snapshot. Stable reads give consistency=observed_stable, explicitly not a
transactional snapshot; ABA changes and races immediately afterward remain
possible. No inode/mtime-based safety claim. A disappeared deployment or mismatch
between records blocks correlated conclusions even if individual reads succeeded.
The total command is bounded by existing OperationTimeout; no background polling.

PendingID can come from an in-memory admitted unsigned request with no signed
record. Even observed_stable files and no journal do not prove that the running
sender is available. Always report unsignedAdmission=unknown and retrySafety=unknown.
No offline record permits a recommendation to delete/retry/clear ownership.

## Report schema and redaction

Choose schema `merkle.recovery.v1`, with deployment, consistency,
unsignedAdmission, retrySafety, journal, jobs and blockers. Emit deterministic
strategy ordering and attempt ordering from the validated journal. Decimal chain,
block and nonce values are strings; addresses/hashes use full hex. Unknown or
unavailable values are null. Preserve known legacy job IDs exactly; bound textual
identities to valid printable UTF-8 and 256 bytes, otherwise emit null with
invalid_identity. Job phase is a closed set: prepared/authorized/settled/failed.
Provider progress is an array of {index, done, outcome}; outcome is only
pending/done/unknown derived from validated fields. Do not serialize provider's
arbitrary Outcome string, State, Data or plugin error.

Required terminalEvidence fields: kind, transactionHash, blockNumber, blockHash
and freshness=saved_only. No live confirmation count is computed. Confirmation
depth is requiredDepth, not achievedDepth. Journal.attempts is a list of hashes;
winning hash can be an older attempt and must be in that list. Bad signed bytes or
changed replacement identity make the whole journal unknown: no partially trusted
hashes, nonce, owner ID or terminal metadata leak into the report.

No raw signed transactions, calldata, signatures, proofs, complete Execution,
receipt logs, provider payloads, arbitrary saved errors or config/RPC/HTTP URLs
are printed. stderr is bounded and uses codes, not wrapped payload text. Never
serialize a raw Job or record wholesale. Input parsing failure cannot dump input.

A valid complete diagnosis exits 0 even when it describes signed-unknown or
external-pending work. Invalid config/flags, missing or mismatched deployment,
unsupported/corrupt records, mixed snapshot, cancellation and stdout failure exit
1. After offline config validation, errors may emit a sanitized partial report;
all missing/invalid correlations stay unknown. No record changes occur on either
exit path. No new flag or behavior is added to reset-strategy.

## Blockers and operator guidance

Use finite codes and fixed operator text, not protocol errors or guessed next
transactions. Every message is inspect/resume guidance, never permission to retry.
Precedence is deployment_invalid, mixed_snapshot, corrupt/unsupported identity,
orphan/mismatched ownership, then ordinary durable lifecycle evidence.

| Code | Evidence | Bounded guidance |
|---|---|---|
| signed_unknown | Valid signed attempts with no terminal evidence | Resume normal recovery with the same configuration; retain ownership |
| authorization_evidence_saved | Saved confirmed recipe evidence and journal still occupied | Resume normal recovery to validate/acknowledge; do not reset |
| external_pending | Authorized job, acknowledged/no matching journal, unfinished progress | Resume provider reconciliation; offline report cannot prove payment |
| confirmed_revert_review | Saved reverted terminal or failed job | Operator review of the saved failure and normal canonical recovery required |
| orphan_owner | Valid journal ID has no matching saved job at all | Restore the owning workflow/configuration; retain journal |
| owner_removed | A matching saved job exists but its strategy is absent from config | Use compatible recovery tooling and original configuration; retain journal and IDs |
| owner_mismatch | Job result/identity conflicts with journal evidence | Stop manual intervention and investigate; no release inference |
| corrupt_record / unsupported_version / invalid_identity | Structural/typed evidence invalid | Inspect supported tooling/backups; no replay/reset advice |
| mixed_snapshot | Files changed between the bounded two passes | Rerun inspection later; current result is not a consistent correlation |
| record_too_large | Read exceeds configured finite byte budget | Review configured diagnostics budget offline; do not print or truncate payload |
| no_signed_evidence | No journal observed | Unsigned admission and retry safety remain unknown |

Examples are synthetic. None establishes live canonical state or safe replay.

~~~json
{
  "schema": "merkle.recovery.v1",
  "deployment": {
    "chainId": "1",
    "caller": "0x3333333333333333333333333333333333333333",
    "source": "saved_only"
  },
  "consistency": "observed_stable",
  "unsignedAdmission": "unknown",
  "retrySafety": "unknown",
  "journal": {
    "state": "validated",
    "ownerId": "legacy-job-1",
    "chainId": "1",
    "sender": "0x3333333333333333333333333333333333333333",
    "nonce": "7",
    "attempts": [
      "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    ],
    "requiredDepth": 2,
    "terminalEvidence": null
  },
  "jobs": [
    {
      "identifier": "0x1111111111111111111111111111111111111111",
      "configured": true,
      "jobId": "legacy-job-1",
      "phase": "prepared",
      "finishedAt": null,
      "journalOwnsJob": true,
      "savedReceipt": null,
      "progress": []
    }
  ],
  "blockers": [
    {
      "code": "signed_unknown",
      "jobId": "legacy-job-1",
      "guidance": "Resume normal recovery with the same configuration; retain ownership"
    }
  ]
}
~~~

An acknowledged authorization can still await external settlement.

~~~json
{
  "schema": "merkle.recovery.v1",
  "deployment": {
    "chainId": "1",
    "caller": "0x3333333333333333333333333333333333333333",
    "source": "saved_only"
  },
  "consistency": "observed_stable",
  "unsignedAdmission": "unknown",
  "retrySafety": "unknown",
  "journal": null,
  "jobs": [
    {
      "identifier": "0x1111111111111111111111111111111111111111",
      "configured": true,
      "jobId": "job-2",
      "phase": "authorized",
      "finishedAt": null,
      "journalOwnsJob": false,
      "savedReceipt": {
        "transactionHash": "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
        "blockNumber": "100",
        "blockHash": "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
        "freshness": "saved_only"
      },
      "progress": [
        {
          "index": 0,
          "done": false,
          "outcome": "pending"
        }
      ]
    }
  ],
  "blockers": [
    {
      "code": "external_pending",
      "jobId": "job-2",
      "guidance": "Resume provider reconciliation; offline report cannot prove payment"
    }
  ]
}
~~~

A changed/corrupt record does not expose its partial signed data.

~~~json
{
  "schema": "merkle.recovery.v1",
  "deployment": {
    "chainId": "1",
    "caller": "0x3333333333333333333333333333333333333333",
    "source": "saved_only"
  },
  "consistency": "mixed",
  "unsignedAdmission": "unknown",
  "retrySafety": "unknown",
  "journal": {
    "state": "unknown",
    "ownerId": null,
    "chainId": null,
    "sender": null,
    "nonce": null,
    "attempts": [],
    "requiredDepth": null,
    "terminalEvidence": null
  },
  "jobs": [],
  "blockers": [
    {
      "code": "mixed_snapshot",
      "jobId": null,
      "guidance": "Rerun inspection later; current result is not a consistent correlation"
    },
    {
      "code": "corrupt_record",
      "jobId": null,
      "guidance": "Inspect supported tooling or backups; retain ownership"
    }
  ]
}
~~~

## Acceptance matrix

All names below are proposed new tests. Reuse current cmd root_test.go fixtures:
TestStatusWhileDaemonOwnsState, TestStateCommandsRejectOtherDeployment,
TestStatusWithoutIdentityDoesNotCreateState, TestOfflineStateCommandsUseSavedCallerWhenAssertionOmitted,
TestStatusRetainsLegacyIdentifiers and TestResetStrategyRequiresTerminalAcknowledgedWork.
Reuse submission manager_test.go signed variants, changed replacement, restart and
terminal evidence fixtures, plus state/store_test.go atomic read fixtures.
Every row asserts zero RPC/HTTP/provider calls, zero signing-key/env-value loading, zero
writer-lock acquisition and zero file changes. Assert snapshot bytes and reset
semantics unchanged. Synthetic sensitive markers must be absent from stdout,
stderr, returned DTO and exposed error causes.

| Proposed test | Input and expected result | Additional assertion |
|---|---|---|
| TestDiagnoseWhileDaemonOwnsState | Writer lock held: observed_stable report | No second writer lock; existing status still passes |
| TestDiagnoseMissingIdentity | No deployment.v1: deployment_invalid, exit 1 | Does not create directory or lock |
| TestDiagnoseOtherDeployment | Wrong saved chain/caller: deployment_invalid | No journal decoding against invented identity |
| TestInspectOriginalReplacement | Same nonce/payload signed variants | Both hashes in persisted order; exact owner and nonce |
| TestInspectOlderAttemptWins | Terminal evidence references first variant | Winning hash first, never last-by-default |
| TestDiagnoseOrphanOwner | Journal without matching job: orphan_owner | No inferred job or deletion guidance |
| TestDiagnoseRemovedOwner | Owner absent from config: owner_removed | Historical inventory still inspected |
| TestDiagnoseLegacyIdentity | Known legacy name and job ID | Exact strings retained, no address normalization |
| TestInspectUnsupportedVersion | Unsupported record version | No partial owner/hash fields, unsupported_version |
| TestInspectCorruptAttempt | Invalid signed bytes | Unknown journal and corrupt_record; no raw error |
| TestInspectChangedReplacement | Target/data/nonce/sender/chain variants | Entire journal rejected using shared decoder |
| TestDiagnoseMixedSnapshot | A file changes/disappears between passes | Exactly two passes, mixed_snapshot and exit 1 |
| TestDiagnoseUnsignedAdmissionUnknown | Prepared job with no journal | unsignedAdmission/retrySafety unknown; no safe-to-retry statement |
| TestDiagnoseTerminalOccupied | Settled/failed job still owns journal | Report saved evidence/review, no reset recommendation |
| TestDiagnoseRedactsProviderMetadata | Synthetic marker in State/Data/Outcome/errors | Marker absent everywhere; bounded enum only |
| TestDiagnoseRecordBudget | Record exceeds YAML bound; invalid byte-limit config | Bounded read, no truncation decode or raw output |
| TestDiagnoseCancellation | Context canceled between passes | Exit 1, finite termination, no writes |

Job/record parsing also rejects unknown phase, empty/malformed identities,
invalid execution status and terminal winning hash outside owned attempts. Compare
job Result to its Execution without trusting a standalone hash as canonical proof.
If a malformed job prevents safe ownership matching, classify the correlation
unknown rather than manufacture an orphan conclusion from incomplete evidence.

## Trade-offs and next implementation

A separate command avoids changing consumers of the existing status array. A pure
SDK inspector avoids drift from Manager.decode and can be used by other hosts.
An observed-stable two-pass snapshot is useful without the false promise of a
cross-file transaction. It cannot inspect unsigned process memory or replace
normal recovery. Expected implementation effort is 2-3 days including validation,
redaction/resource tests, docs and the SDK consumer-release gate.

Minimum future candidate scope, to reconcile before runtime implementation:

- sdk/submission/inspection.go and inspection_test.go (new safe DTO/typed codes).
- sdk/submission/manager.go and manager_test.go (shared private decoder, unchanged
  runtime semantics and wire record).
- sdk/state/store.go and store_test.go (additive bounded atomic read helper).
- cmd/merkle-executor/diagnose.go and diagnose_test.go (new offline report).
- cmd/merkle-executor/main.go and root_test.go (Cobra registration and regressions).
- cmd/merkle-executor/state.go only to reuse identity validation without weakening
  status/reset guards; keep existing output unchanged.
- internal/config/config.go, internal/config/config_test.go and config/example.yaml
  (diagnostics.maxRecordBytes, finite limits and strict offline validation).
- README.md, sdk/README.md and docs/TRANSACTIONS.md (saved-evidence contract).
- Root go.mod/go.sum only after authorized immutable SDK publication.

Ordered handoff: add shared decoder rejection/redaction tests; extract exact
validator without changing Manager behavior; add bounded reader and pure DTO;
write offline CLI tests with network/key/state spies; implement diagnose; run all
old status/reset tests plus new matrix; document finite schema and resource bounds.
The schema can evolve additively; unknown future journal versions remain explicit.
Do not export record or change persisted journal format.

Commands for that future implementation, each from checkout root unless cd:
`cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test -race ./state ./submission ./keeper`;
`GOTOOLCHAIN=go1.27.1 go test -race ./cmd/merkle-executor ./internal/config`;
then `make check`, `make integration`, temporary local-SDK workspace tests and the
published-consumer checks from plans/README.md. Existing baseline passes do not
prove a diagnose command exists. Refresh the exact file scopes in a self-contained implementation plan before
code changes.

Optional online canonical checks are deferred. They need separately labelled RPC
freshness, explicit network semantics and tests, never an implicit status side
effect. Reset/release/replay automation is excluded from this direction entirely.
