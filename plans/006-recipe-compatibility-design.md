# Authenticated recipe compatibility inspection

Design spike 006, 2026-10-16. Source reference: 3184aa390edbeb66d1d6c5d1a62dc337794c4d6e.
Reviewed against local source d6690d0daded2cb6ab71f035b4124f22c141f438.
This document specifies a future command. It does not introduce a runtime API or CLI.

## Authentication and purity

Choose an additive SDK inspection seam. Host orchestration over public discovery
alone would duplicate the tree identity, provider ownership and prefix checks in
keeper.Planner.Prepare. Keep one implementation of those checks and reuse it in
preparation and inspection. Do not reuse Prepare and intercept its simulation:
it already calls PrepareRecipe and can obtain protocol data before simulation.

Existing symbols and the proposed flow:

1. Host config.Load performs the existing strict YAML validation. Construct the
   read client and curator permission source, discoverIdentity and the existing
   NewAutomatic providers plus keeper.NewRecipeInputs. Those automatic constructors
   currently validate dependencies/catalogues without protocol HTTP or key loading.
   Retain the configured caller assertion and the one-caller process rule. Do not
   enter runBot's state/signer setup. A constructor that later needs runtime data
   must supply a pure constructor or produce unknown, never acquire it here.
2. recipes.DiscoverForAccount pins metadata, authenticates the last SetRecipe event
   and its ABI commitment, and closes its initial canonical guard. Check the
   configured recipe hash and caller as Prepare does. The host verifies chain ID
   using chain.Dial; the SDK also requires positive Planner.ChainID for tree identity.
3. Factor Planner.permissionTree plus the existing tree root/chain/account checks
   into a private authenticatedRecipe helper shared with Prepare. recipes.DecodeTree
   validates every leaf/proof, including unused leaves, before any provider callback.
4. Invoke only RecipeProvider.RecipeRequirements on recipes.CloneRecipe/CloneTree.
   Extract the existing ownership, ValidateRecipeComposition,
   recipes.ValidateMutableActions, recipes.AnchoredInputs and recipes.ValidatePrefix
   logic into one private recipePreflight helper shared by both paths. Build the
   same PrefixRuntime ownership/blocked-action maps. Do not resolve values, invoke
   a lens, execute recipe instructions, assemble runtime output or simulate execute.
5. Obtain managed permission hashes with recipes.ManagedActions and call
   recipes.VerifyRoles for every structurally referenced managed leaf. Because no
   runtime skip decision is made, this is deliberately conservative. A role read
   failure makes authorization unknown; missing roles make authentication fail.
   Do not promise structural support for a conditional leaf lacking a role.
6. Before publishing any positive or negative authenticated verdict, call
   recipes.ValidateSnapshot, then recheck the original numbered pin after that
   call. ValidateSnapshot closes the current metadata pin; the extra original
   fence covers the whole inspection window. Any changed root, identity, commitment
   or pin suppresses authenticated verdicts for that executor. Latest-head growth
   alone is allowed. Do not return provisional callbacks as final results.

Proposed public seam in sdk/keeper/inspection.go (these symbols do not exist yet):

~~~go
type RecipeInspection struct {
    Snapshot provider.Snapshot
    Support string
    Diagnostics []RecipeDiagnostic
}
type RecipeDiagnostic struct {
    Provider *int
    Status string
    Code string
    Step *uint
    Call *uint
    Input *uint
}
func (p *Planner) InspectRecipe(context.Context, Strategy) (RecipeInspection, error)
~~~

The DTO has no Calldata, Outputs, signatures, proofs or provider state. The SDK
uses provider indices, with host labels from its fixed startup registrations.
Use constants for finite status/code enums; Provider=nil means SDK/global validation.
Add a typed ValidationDiagnostic error in recipes for selected provider and generic
validation rejection sites; errors.As consumes that safe code and position, never
err.Error substring parsing. Locating it in recipes avoids a keeper/recipes import
cycle. Emit exclusive_target_conflict directly in keeper/composition.go, and emit
ownership codes at the existing shared ownership checks; never infer these codes
from arbitrary text. Prefix/mutable-action rejection codes originate at their
validation boundary as well. Unclassified provider
errors map to unknown/requirements_unclassified. A first version may use the
coarse unsupported_shape code at known rejection sites. Empty requirements mean
unmatched for a protocol provider; recipe-input ownership is still checked by the
shared preflight. An ordinary static transfer can be supported with no matched
protocol provider. Unknown mutable actions fail the aggregate support verdict.

Allowed network calls are read-only discovery/metadata/log/role RPC and curator
GET. Protocol quote/reward/order HTTP, Prepare, PrepareRecipe, Advance, numerical
resolution, signing, state.Open, durable writes and transaction submission are
excluded. Use a separately injected rejecting protocol HTTP transport in tests.
OperationTimeout bounds each executor, including final guards; no retries beyond
existing bounded read policy. No provider method may turn a report into a rewrite.

## Command and schema

Choose `merkle-executor inspect-recipe --config PATH [--executor ADDRESS]` with
Cobra NoArgs. The optional executor must be a full nonzero address already in the
configured set; normalize for lookup and reject an unknown address before network.
No extra targets, manual provider config or runtime overrides. Do not change plan.

Output one JSON object with schema `merkle.compatibility.v1` and results sorted by
canonical executor address. Chain ID and block numbers are decimal strings; hashes
are 32-byte hex and addresses are full hex. Each result has executor, identity
(null if unauthenticated), authentication (verified/failed/unknown), support
(supported/unsupported/unknown), readiness (always unknown), and diagnostics.
Identity contains chainId, blockNumber, blockHash, account, caller, recipeHash and
root. Diagnostics have provider (fixed host label or null), status
(supported/unsupported/unmatched/unknown), bounded code and optional step/call/input.
Absent indices are omitted. No arbitrary message/error field is exposed.

Initial finite codes: structure_supported, no_matching_action, unsupported_shape,
unknown_mutable_action, duplicate_input_owner, duplicate_action_owner,
exclusive_target_conflict, permission_identity_invalid, permission_artifact_invalid,
role_missing, requirements_unclassified, canonical_state_changed,
read_unavailable and cancelled. New codes are additive schema changes; existing
meanings must not be reassigned. Authentication failure overrides all supported
provider statuses to unknown. Unmatched is not an execution readiness statement.

A complete report exits 0 even if it describes unsupported structure: scripts
must inspect support. Invalid flags/config/selector exit 1 with no stdout.
Authentication, transport, timeout or closure failure emits any completed results
plus unknown rows for failed/remaining executors and exits 1. Cancellation stops
further work and emits cancelled unknown rows when stdout remains writable; a
stdout error exits 1 without retry. Error text on stderr is bounded and sanitized.
Do not emit raw wrapped HTTP errors. Partial reports cannot imply every executor
was checked. Preserve one-caller deployment assertions without loading its key.

Examples use synthetic identities. A supported report still has unknown readiness.

~~~json
{
  "schema": "merkle.compatibility.v1",
  "results": [
    {
      "executor": "0x1111111111111111111111111111111111111111",
      "identity": {
        "chainId": "1",
        "blockNumber": "100",
        "blockHash": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "account": "0x2222222222222222222222222222222222222222",
        "caller": "0x3333333333333333333333333333333333333333",
        "recipeHash": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "root": "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
      },
      "authentication": "verified",
      "support": "supported",
      "readiness": "unknown",
      "diagnostics": [
        {
          "provider": null,
          "status": "supported",
          "code": "structure_supported"
        },
        {
          "provider": "redemptions",
          "status": "unmatched",
          "code": "no_matching_action"
        }
      ]
    }
  ]
}
~~~

A mixed report may exit 1 because its second executor could not authenticate.

~~~json
{
  "schema": "merkle.compatibility.v1",
  "results": [
    {
      "executor": "0x1111111111111111111111111111111111111111",
      "identity": {
        "chainId": "1",
        "blockNumber": "100",
        "blockHash": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "account": "0x2222222222222222222222222222222222222222",
        "caller": "0x3333333333333333333333333333333333333333",
        "recipeHash": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "root": "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
      },
      "authentication": "verified",
      "support": "unsupported",
      "readiness": "unknown",
      "diagnostics": [
        {
          "provider": "redemptions",
          "status": "unsupported",
          "code": "unsupported_shape",
          "step": 0,
          "call": 1
        }
      ]
    },
    {
      "executor": "0x4444444444444444444444444444444444444444",
      "identity": null,
      "authentication": "failed",
      "support": "unknown",
      "readiness": "unknown",
      "diagnostics": [
        {
          "provider": null,
          "status": "unknown",
          "code": "permission_identity_invalid"
        }
      ]
    }
  ]
}
~~~

## Acceptance matrix

All test names below are proposed new tests. Reuse permissionReader and
TestPlannerPermissionSourceVerifiesAllBytesBeforeProviders in
sdk/keeper/permission_source_test.go, exclusiveRecipeReader and
TestPlannerExclusiveRecipePreflightBeforeProviderIO in recipe_composition_test.go,
existing automatic provider fixtures and cmd root_test.go flag/config helpers.
For every row assert zero signing, zero order/reward/quote HTTP, zero durable
writes, zero Prepare/PrepareRecipe/Advance and zero execute/lens simulation calls.
Read RPC means only the allowlisted discovery/metadata/log/role methods; curator
GET is at most one per executor. Invalid offline input has zero network calls.

| Proposed test | Case and expected result | Additional call/closure assertion |
|---|---|---|
| TestInspectStaticTransfer | Pure committed transfer: verified/supported; protocols unmatched | Read RPC and one curator GET; all closing fences run |
| TestInspectIssuerRequest | Supported issuer request: verified/supported, readiness unknown | No issuer API or economic balance resolution |
| TestInspectFigureSameStep | Two Figure operations in one Manage: unsupported_shape at second call | Same rejection as selectRoutes; no automatic rewrite |
| TestInspectUnknownMutableAction | Dynamic action with no owner: unsupported/unknown_mutable_action | No preparation callback |
| TestInspectDuplicateOwnership | Duplicate input or action: corresponding owner code | Zero protocol I/O even if another provider matches |
| TestInspectExclusiveTarget | Conflicting exclusive output target: exclusive_target_conflict | Reuse both direct Call and unowned target fixtures |
| TestInspectWrongPermissionIdentity | Wrong chain/root/account: failed/permission_identity_invalid | Zero provider requirement calls |
| TestInspectUnusedInvalidLeaf | Invalid unused proof: failed/permission_artifact_invalid | Every leaf checked; zero requirements calls |
| TestInspectPermissionCancellation | Cancel curator headers/body: unknown/cancelled, exit 1 | errors.Is preserved internally; no continuation |
| TestInspectCanonicalReorg | Original pin or current metadata changes during final reads | unknown/canonical_state_changed; no supported verdict |
| TestInspectMixedReadiness | Two supported recipes, one would currently be idle | Both supported and readiness unknown; zero runtime acquisitions |
| TestInspectPartialFailure | One checked executor, next transport failure, then cancellation | Ordered rows explicit; failures/unchecked unknown, exit 1 |
| TestInspectOfflineSelector | Unknown/malformed executor or unknown YAML field | Exit 1; zero network, keys or state access |

Use both batched and legacy readers for final fences. Include a malicious provider
that mutates its recipe/tree arguments and assert immutable copies and unchanged
subsequent providers. Existing missing-role, wrong-caller and unused-leaf tests
remain passing. Test a non-RecipeProvider as unknown rather than calling Prepare.

## Trade-offs and next implementation

The SDK seam costs a small refactor but avoids a second authority implementation.
Checking all potential managed leaves is conservative; runtime skip/eligibility
support can be designed later without weakening this report. Coarse codes are
stable initially; exact protocol-specific wording stays in provider docs.
Expected implementation effort is 2-3 days including negative cases and SDK release.

Minimum future change scope, to be confirmed by a fresh drift check:

- sdk/keeper/inspection.go and inspection_test.go (new DTO, InspectRecipe, codes).
- sdk/keeper/planner.go (shared private authentication/preflight helpers only).
- sdk/keeper/recipe_provider.go (document inspection purity; existing interface stable).
- sdk/keeper/composition.go, composition_test.go and recipe_composition_test.go
  (typed generic composition rejection codes, including exclusive targets).
- sdk/recipes/diagnostic.go and diagnostic_test.go (new safe typed error and codes),
  prefix.go and derivation.go (typed existing prefix/mutable-action rejections).
- internal/providers/cowswap/automatic.go and automatic_test.go;
  internal/providers/merkl/automatic.go and automatic_test.go;
  internal/providers/liquidlanerfq/automatic.go and automatic_test.go;
  internal/providers/redemptions/automatic.go and automatic_test.go;
  internal/providers/securitize/automatic.go and automatic_test.go, for typed existing
  requirement rejections only; no supported-shape expansion.
- cmd/merkle-executor/inspect.go and inspect_test.go (new), main.go and root_test.go.
- cmd/merkle-executor/run.go only to share pure startup registration; do not share
  signer/state setup or change existing plan output.
- README.md and sdk/README.md for command/API contracts, docs/PROVIDERS.md for codes.
- Root go.mod/go.sum only after an authorized immutable SDK publication.

Ordered implementation: add failing inspection/purity tests; factor shared helpers
with existing preparation tests green; add SDK read-only API; add typed rejection
codes; wire Cobra separately from runBot; close the negative matrix and docs.
Run `cd sdk && GOWORK=off GOTOOLCHAIN=go1.27.1 go test -race ./keeper ./recipes`;
run `GOTOOLCHAIN=go1.27.1 go test -race ./internal/providers/... ./cmd/merkle-executor`;
then `make check` and `make integration`, plus the temporary SDK workspace and
published-consumer gates described in the plan index. Passing baseline tests for
this spike proves no new feature. Before runtime work, reconcile these exact
candidate files into a self-contained implementation plan; do not silently expand
001-005. No new dependency, schema migration or persisted state is required.

Maintenance: positive verdicts are authenticated as of their explicit pin and can
be stale immediately afterward. Keep source checks and output codes together.
Never equate structural support with liquidity, live permission freshness after
closure, successful execution or settled payment.
