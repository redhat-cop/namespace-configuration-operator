# Review — adversarial pass 1 on the merged work of 2026-09-04/05

Adversarial second-opinion pass, 2026-09-05, on two briefs: 9 claims over `git diff e8c92ea..2cf0e19` of this
repository (branch feature/finalizer-fixes-template-filtering-tests) and 5 claims over commit 84aa264 of the
operator-utils fork. Codex (gpt-5.6-sol, xhigh, via the codex-rescue plugin) had a shell but its sandbox refused
temp files, so it measured with read-only commands only; Cursor (Grok 4.6 high fast, ask mode) traced from source
and in the library brief ran measurements in `/tmp` copies. Every verdict was re-checked here before a decision.
Briefs and raw outputs: session scratchpad `adv/review_brief_*.md`, `adv/review_{codex,cursor}_*.txt`.

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

## Verdicts, library brief

Recorded in the companion section once the Codex run completes; Cursor's measured verdicts: C1 pointer-keyed
cache grows with fresh configs (accepted, key by config fingerprint); C2/C4 `NormalizeJSONPaths` collapses
bracketed dotted keys (accepted on the fact; recorded as a limitation of code the SSA branch deletes); C3 the
tolerated `invalid index` text also covers negative indexes (accepted, reject negative indexes at conversion);
C5 an unprefixed path now gains a root slash (accepted, prefix only when the input began with `.` or `$`);
"not asked": the cache test depends on global map state (accepted, reset in the test).

## Outcome

Operator: 6 claims refuted or partly refuted, 4 fixes applied (predicate scope, requeue, first-document rule,
selector validation) plus two script fixes; 4 reviewer snippets rejected with reasons above. Re-validated:
`go test -race ./...`, `hack/lib_test.sh`, the gate, a CRC run of the fixed head (below). Second pass: pending
on the fixed head.
