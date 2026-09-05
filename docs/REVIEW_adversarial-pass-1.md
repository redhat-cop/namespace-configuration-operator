# Review — adversarial pass 1 on the merged work of 2026-09-04/05

Adversarial second-opinion passes, 2026-09-05. First pass: 9 claims over `git diff e8c92ea..2cf0e19` of this
repository (branch feature/finalizer-fixes-template-filtering-tests) and 5 claims over commit 84aa264 of the
operator-utils fork. Second pass: 6 claims over the operator fixes (`git diff 2cf0e19..205b33a`) and 5 over the
library fixes. Reviewers: Codex (gpt-5.6-sol, xhigh) and Cursor (Grok 4.6 high fast, ask mode). Which of them
could measure varied by run and is stated per section; a verdict without an artefact was treated as a hint.
Every verdict was re-measured here before a decision. Briefs and raw outputs: session scratchpad
`adv/review_brief_*.md`, `adv/review_{codex,cursor}_*.txt`.

## Verdicts, operator brief

| Claim | Codex | Cursor | Decision |
|---|---|---|---|
| C1 render failure → error, no partial batch | CONFIRMED | CONFIRMED | — |
| C2 cleanup deletes only what the CR owns | REFUTED | REFUTED | **Accepted on the facts; both snippets rejected** (see below) |
| C3 predicate filters nothing a template can read | REFUTED | REFUTED | **Accepted; scoped fix, snippets rejected** |
| C4 (a) no stale-generation success (b) churn still gets status (c) clearing ReconcileError masks nothing | (a) ok (b) REFUTED (c) ok | (a) ok (b) PLAUSIBLE (c) PLAUSIBLE | **(b) accepted, minimal fix; (c) rejected** |
| C5 filter ≡ renderer | REFUTED | REFUTED | **Accepted on the wording and on one measured divergence; "remove the static path" rejected** |
| C6 malformed selector skips only its CR | CONFIRMED | CONFIRMED | — |
| C7 hack scripts do what their headers say | REFUTED | REFUTED | **Accepted in part** (trailing slash, `--platforms`); podman claim **refuted by measurement** |
| C8 renders carry the flags and the fork image | REFUTED | CONFIRMED | **Claim wording was wrong; no change** |
| C9 guard valid, startup failure has another cause | CONFIRMED (wrong cause) | CONFIRMED (right cause) | **Codex's cause refuted; permissions fix already applied** |

## C2 — cleanup ownership

**Finding (both).** `OwnedResources` recomputes "what the current spec renders for the currently selected
objects" and cleanup deletes that; it is not "what this CR created". Two CRs rendering the same object: deleting
one deletes it. A spec edited since creation: the old names are orphaned. A malformed selector at deletion:
finalizer removed with objects possibly present.

**Re-check.** All three are true of the new code. They are also true of the enforcer this replaced:
`Terminate(instance, true)` deletes every locked resource the manager was started with, with no ownership
check, and a spec change while the operator was down orphaned the old names before this work too. The new
path deletes the same set a started enforcer would have deleted, and additionally works when no enforcer was
started (the bug #2 fixed). The malformed-selector case was chosen deliberately (#3): a finalizer that can
never clear is worse than a documented orphan.

**Decision.** Accepted on the facts, recorded here. Codex's fix (ownerReferences + garbage collection) and
Cursor's fix (a last-applied-keys annotation plus cross-CR arbitration at delete time) are both design changes
to the ownership model; rejected under the operator's "no complications" rule and because ownership is being
addressed at the right layer, server-side-apply field management in operator-utils (issue #16, upstream PR
redhat-cop/operator-utils#103). One part accepted from Cursor's "not asked": the UserConfig cleanup did not
validate `identityExtraFieldSelector`; `ValidateSelectors` is now variadic and names the failing selector.

## C3 — the watch predicate

**Finding (both).** `SelectedObjectChangedPredicate` compares only labels and annotations; `Group.users`,
`User.identities`, `Namespace.spec` changes are dropped. A GroupConfig template may read `.Users`.

**Re-check.** Measured: `Group.users` change → predicate false; `User.identities` → false; `Namespace.spec` →
false. Live on CRC: adding a user to a synced group produced 0 GroupConfig reconciles on the merged build.

**Decision.** Accepted. The gate stays on the Namespace watch only (a namespace's contract with a
NamespaceConfig is its labels and annotations; measured benefit 5→0 reconciles per status-only update) and is
removed from the Group and User watches. Codex's snippet (remove the predicate everywhere) rejected: it gives
back the measured benefit for nothing. Cursor's snippet (deep-compare everything but status) rejected: it
re-implements event filtering per kind; scoping the gate is one line.

## C4 — status writes

**Finding.** (b) If the generation moves on every reconcile, the helper never writes status. (c) Clearing
`ReconcileError` on the parent can look green while child resources fail.

**Re-check.** (b) true under perpetual churn. (c) the parent condition was never set for child failures before
this work either; child failures live in `lockedResourceStatuses`, unchanged.

**Decision.** (b) accepted: `Requeue: true` when the generation moved (one line). Codex's `ReconcilePending`
condition and Cursor's `hasFailingLockedResources` gate rejected: a new condition type is a complication, and
with `returnOnlyFailingStatuses=true` every fresh child counts as "failing" (Initializing), so the gate would
never clear the error right after creation.

## C5 — filter versus renderer

**Finding.** The brief's oracle ("produces an object or an error") is not the code's contract; and a taken
branch beginning with `---\n---` counts as content for the static evaluator while the renderer, which parses only
the first YAML document, gets `null`.

**Re-check.** Measured: `YAMLToJSON("---\n---\n<doc>")` → `null`; a single leading `---` → the document; `data:`
→ `{"data":null}` (an object, then a Kind error: visible, correct). The contract was stated imprecisely in the
brief and in the test comment; the code was right except for the first-document case.

**Decision.** Accepted: `listHasContent` now judges the first YAML document only; the test comment states the
contract precisely; the reviewers' extra shapes (range, define/template, with, `data:`, `eq` non-string, nil
annotations, leading `---`) are in the property test. Codex's snippet (delete the static evaluator, always render)
rejected: the property test proves equality and the static path is the cheap one.

## C7 — hack scripts

**Finding.** `origin_repo` mishandles a trailing slash; `--platforms` with no value dies under `set -u`;
(Codex) `podman push --digestfile` is unavailable to remote clients such as macOS.

**Re-check.** Trailing slash: true. `--platforms`: true. Podman: **refuted** on this machine (remote podman,
`podman info` remote=true): `podman push --digestfile` succeeded and the file held the registry's
`Docker-Content-Digest` exactly.

**Decision.** Trailing-slash strip and `--platforms` check accepted; Codex's pull-and-inspect replacement rejected.

## C8, C9

C8: the brief claimed `config/default` renders the ephico2real image; it renders upstream's, by design (the
overlay retags). Codex's fix (retag the base) rejected: the base must stay upstream. C9: Codex named
`GO_VERSION` as an undeclared input; the pinned callee declares it (checked programmatically). The measured cause
is permissions: the nested jobs request `id-token: write`, a fork's token is read-only; fixed on PR #37 and proven
(job skipped).

## Verdicts, library brief (first pass, commit 84aa264)

Cursor measured in `/tmp` copies; Codex (via the plugin) had a shell and measured in a copy with a redirected
build cache. The fixes for this pass are commits 5265b70, 3b9a9b3 and 05cfe9e on the library branch.

| Claim | Codex | Cursor | Decision |
|---|---|---|---|
| C1 cache is safe and keyed by config | REFUTED | REFUTED | **Accepted**: 1000 fresh `&rest.Config{}` retained 1000 entries. Keyed by the config's material (5265b70). Codex's text-only key with clone-and-rebind and a 256-entry FIFO rejected: the material key already collapses equivalent configs; growth by distinct template text is upstream's behaviour and bounded by the templates ever written to CRs |
| C2 quoted keys are handled | REFUTED | REFUTED | **Accepted on the fact, recorded, no change**: `FilterOutPaths` removes the literal key correctly (measured for `a.b`, `x/y`, a slashed label), but `NormalizeJSONPaths`' dotted form makes `.data['a.b']` also shield nested `data.a.b` in the null-injection check. That code (the #194 null injection) is not carried to the server-side-apply branch |
| C3 tolerated `invalid index` text | REFUTED | REFUTED | **Accepted**: Codex's table of the 11 json-patch sites shows the text covers negative indexes too. Rejected by name in every spelling (5265b70, 05cfe9e). Codex's `pathExists` preflight and pointer-canonical rewrite of patch.go rejected: a second traversal per path and a new exported pointer API to reach the same outcome as validating the spelling |
| C4 null injection respects excluded paths | REFUTED | PLAUSIBLE | **Accepted on the fact, recorded, no change**: an excluded descendant under an ancestor the template omits skips the whole ancestor (`.spec.template.spec` excluded, live `spec.unrelated` kept; `.rules[0]` excluded, whole array kept). Same as upstream without #194; deleted with SSA. Codex's recursion plus rejection of partial array exclusions would turn every existing `.rules[0]`-style exclusion into a reconcile error |
| C5 no observable change for documented spellings | REFUTED | REFUTED | **Accepted in part**: `spec.replicas` unrooted kept its no-op meaning (5265b70); `.data.` no longer deletes the parent (3b9a9b3). The other changed forms (`.data['a.b']` no-op to removal, `.rules[0]` error to removal) are the bugs the change exists to fix. A `FilterOutPathsV2` rejected |

**Volunteered by Codex, accepted as recorded:** `GetLockedResourcesFromTemplates*` returns `[]LockedResource{},
nil` on parse and execute errors. True, and it is upstream's exported contract; this operator renders through its
own `TemplateFilter.Render` for exactly that reason (see the comment in templatefilter.go). A change to the
exported contract belongs to the upstream conversation, not this branch.

**Found while re-checking, fixed:** `.data.` deleted all of `data` (a trailing-dot trim carried over from the old
`]` handling, introduced at 84aa264) and `.data[` did nothing, silently: both are reported by name (3b9a9b3).
The negative-index check ran on the pointer, so a quoted key `.data['-1']` was rejected as an index: the checks
now run on the spelling with quoted keys removed (05cfe9e). Tests for every case fail before and pass after.

## Second pass, operator (fixes at 205b33a)

Cursor's and Codex's sandboxes both refused every shell call and temp file for this run, so their verdicts are
traced from source; every runtime claim was measured here in a detached worktree of 205b33a.

| Claim | Cursor | Codex | Decision |
|---|---|---|---|
| C1 first-document rule ≡ YAMLToJSON | REFUTED | PLAUSIBLE | **Accepted**: measured over 21 shapes, four diverge (`null`, `~`, an anchor with no node, a bare `--- # note`); three of Cursor's rows were backwards (`...` and a lone `%YAML` are parse errors, so applicable; a comment before the first `---` is the object). Fix from both reviewers: judge the literal text with `rendersAnObject`, delete the line scanner |
| C2 gate on Namespace only; only labels/annotations read | REFUTED | REFUTED | **Accepted on the second half, no predicate change**: tests exercise `.Spec.Finalizers` through the render fallback; examples read `.Name` (immutable). The spec-aware predicate both proposed rejected: `Namespace.spec` holds only `finalizers`, written by the API server; no shipped template reads it; the limit is documented in common.go. FEATURES paragraph corrected (it still said the gate was on all three watches). Codex's finding that the predicate test's "spec only" case set nil to nil accepted: the case now changes a finalizer |
| C3 Requeue cannot loop or double-write | REFUTED | REFUTED | **Accepted; pass-1 decision C4(b) retracted**: measured with client-go's workqueue, a dirty mark plus `AddRateLimited` gives 3 reconciles where the watch alone gives 2, because `Forget` does not cancel the delayed add. The requeue also never achieved its stated purpose: under perpetual churn every cycle skips again. `Requeue` removed. Live on the cluster: two spec edits, two reconciles, ReconcileSuccess at the final generation |
| C4 every selector validated, named as in the CRD | CONFIRMED | CONFIRMED | — |
| C5 every GitHub URL form; `--platforms` value | REFUTED | PLAUSIBLE | **Accepted in part**: `git://` added to the arm (a generalised `*://*` arm tried first accepted `file://` with a bogus result and was caught by the new test). `insteadOf` refuted by measurement: `git remote get-url` expands it (Cursor claimed the opposite). `--platforms=` unchanged: an unadvertised form that fails loudly with "unknown option". Codex's 40-line `origin_repo` with host and owner validation rejected for a helper whose only consumer is `gh -R` |
| C6 two gate runs identical | PLAUSIBLE | PLAUSIBLE | **Measured, confirmed**: two runs in a clean worktree, exit 0 both, identical tracked, untracked and ignored state, logs identical except the first run's controller-gen download. Only `bin/` (ignored, the Makefile's tool cache by design) differs from a fresh clone. Codex's scratch-dir redesign rejected: it re-downloads controller-gen on every run |

**Volunteered by Codex, measured, accepted:** the static path declared any non-blank unguarded text applicable,
so a comment-only template, a `---`, a `null`, and, wider than reported, a header comment above a guard that is
false were all "applicable"; the render then failed the whole reconcile for that namespace with the original
"Object 'Kind' is missing in 'null'". Literal-only templates are now judged by the oracle and literal text outside
a guard sends the template to the render fallback; the shapes are in the property test. The shipped chart has no
header comment above a guard, so it was not affected.

**Volunteered by Cursor:** the stale FEATURES paragraph, accepted; a test that greps controller sources for the
predicate name rejected (it tests no behaviour).

## Second pass, library (fixes at 05cfe9e)

Cursor measured in a `/tmp` copy; Codex's CLI sandbox was read-only (traced). Fixes: commit bcf5ba5.

| Claim | Cursor | Codex | Decision |
|---|---|---|---|
| C1 `validatePath` rejects exactly the malformed spellings | REFUTED | REFUTED | **Accepted**: a bracket glued to text (`.data[0]foo` became `/data/0foo`, a silent no-op), `$.`, a lone `.`/`$`, and pointer inputs with dots or brackets in a segment (rejected by the dotted-syntax checks) are now handled; a pointer input passes through untouched. Escaped same-kind quotes (`.data['it\'s']`) rejected as malformed, on purpose: write the key with the other quote kind. Codex's `strconv.UnquoteChar` escape decoder rejected as grammar nobody asked for |
| C2 removing the trailing-dot trim changed no well-formed path | REFUTED | REFUTED | **Accepted, in the good direction**: `.data['']` (the empty key) was retargeted to the parent by the trim; it is now `/data/`. Test added |
| C3 the config fingerprint covers what `lookup` sees | REFUTED | REFUTED | **Accepted; design replaced**: `lookup` builds clients from the whole config (Password, TLS, impersonation groups, exec/auth providers, proxy, timeout, Transport), several of them functions. Cursor's 30-field fingerprint (pointers of function fields included) rejected: incomplete by construction. Codex's clone-and-rebind accepted: one parsed base per text, never executed; each call gets a clone bound to its own FuncMap. `restConfigCacheKey` deleted |
| C4 concurrent first use stores one parse, no cross-config FuncMap | REFUTED | PLAUSIBLE | **Accepted**: same root cause as C3; measured after the fix with 32 concurrent callers whose transports differ, each renders its own identity, one cached base |
| C5 documented spellings unchanged | CONFIRMED | PLAUSIBLE | Cursor's ten-form corpus equal at both heads |

**Volunteered by Cursor, accepted:** `.['root']` became `//root` and matched nothing; one trim, tested.
**Volunteered by Codex, accepted:** the RFC 6901 pointer input `/data/a.b` was being converted to `/data/a/b`
(true since upstream); pointer inputs now pass through.

Re-validated: `go test -race ./...` for the whole library module, its envtest controllers suite included (with
setup-envtest binaries), and the operator suite and gate on the bumped head.

## Outcome

Operator, first pass: 6 claims refuted or partly refuted, 4 fixes applied (predicate scope, requeue, first-document
rule, selector validation) plus two script fixes; 4 reviewer snippets rejected with reasons above. Second pass: the
requeue reversed, the first-document rule replaced by the renderer's oracle (guarded, unguarded and outside a
guard), the predicate test made to assert something, `git://` accepted, a stale paragraph fixed. Library: three
fix commits; four findings recorded without change because the server-side-apply branch replaces that code.
Re-validated after each pass: `go test -race ./...`, `hack/lib_test.sh`, the gate, a cluster run of the fixed head.
Library, second pass: the config fingerprint replaced by clone-and-rebind, six path spellings corrected, every
change with a failing-then-passing test.
