# Review — defaults applied in memory, finalizers-only patch, MetadataExcluded event (PR #41, issue #16)

Adversarial second-opinion passes, 2026-09-05, on branch `feat/16-defaults-in-memory`. The design itself was reviewed
before any code (`docs/DESIGN_excludedPaths.md`); these passes are on the implementation. Reviewers: Codex
(gpt-5.6-sol, xhigh, read-only sandbox, traced from source and the Go module cache) and Cursor (Grok 4.6 high fast,
ask mode, no shell). Every verdict was re-measured here (`go test -race ./...`, the gate `hack/local-ci.sh`, and the
sandbox cluster where stated). Briefs and raw outputs: session scratchpad `adv/review_brief_optionA*.md`,
`adv/review_{codex,cursor}_optionA*.txt`.

## First pass (head 19d23c0: the implementation)

| Claim | Codex | Cursor | Decision |
|---|---|---|---|
| C1 the finalizer merge patch from `client.MergeFrom(original)` carries only `metadata.finalizers`, cannot remove an author's empty list, and is no worse than the Update it replaced | REFUTED on concurrency: a plain merge patch has no resourceVersion precondition, so a finalizer another controller added between our GET and our PATCH is silently removed; the Update would have conflicted | same split verdict: patch body CONFIRMED, "no worse than Update" REFUTED | **Accepted.** All six sites (Reconcile and deletion path, three controllers) use `client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})`; the patch carries the read resourceVersion (controller-runtime v0.15.2 `pkg/client/patch.go`), a stale write is a 409 and requeues. Test: the generated patch has only metadata, including `resourceVersion` "7". |
| C2 the deletion path patches from a copy still equal to the live object | CONFIRMED | CONFIRMED | — |
| C3 `EffectiveExcludedPaths` sorted union never restarts a reconciler spuriously | CONFIRMED | CONFIRMED | — |
| C4 `.metadata.finalizers` as a default: no template renders one; the release is a no-op for a template that does not | CONFIRMED | CONFIRMED | — |
| C5 the MetadataExcluded event on every reconcile cannot starve other events of the same CR | REFUTED: client-go's spam filter is 25 events per source+object (all reasons together), then one per 300 s; 25 reconciles of one such CR silence `CleanupIncomplete` on that CR | REFUTED, same source (`events_cache.go:44-48`) | **Accepted.** Emitted once per process per CR and message set, and never on the deletion path (the call moved after the `IsBeingDeleted` block in all three controllers). |
| C6 the documentation test runs from the module root and under the gate and fails when the files move | CONFIRMED | CONFIRMED | — |
| C7 an existing CR with the old three-path list is honoured unchanged; the chart's declared list makes labels enforced | CONFIRMED | CONFIRMED, noting the in-memory union changes `GetKey` once (one reconciler restart, one dry-run apply, no object change) | — |

**Volunteered by Codex, deferred to the third pass:** the documentation test matches the exact numbered block for
the current defaults, but a fourth documented item appended after it would pass. Folded into the third-pass changes
(below) rather than moving the tree under the second-pass reviewers.

## Second pass (head cf934ea: the optimistic lock, the once-per-process event)

| Claim | Codex | Cursor | Decision |
|---|---|---|---|
| C1 a 409 on the finalizer patch requeues in both paths and cannot leave a CR stuck | CONFIRMED (only deletion-time NotFound is swallowed, correctly) | CONFIRMED | — |
| C2 the optimistic lock uses the resourceVersion that was read; nothing refreshes `instance` before the deletion-path patch | CONFIRMED (`patch.go:100`) | CONFIRMED | — |
| C3 placement after the deletion block; a changed template set warns again; a re-created CR (new UID) warns again | REFUTED as wording: only a change of the derived message set warns again, not any template change | CONFIRMED | **Wording corrected** in the code comment; no code change. |
| C4 the process-wide map is bounded by the CRs that ever carried `.metadata` | REFUTED: one entry per UID per historical message set (about 179 bytes each), never pruned, unbounded by template churn | CONFIRMED as slightly wider than stated, prune not warranted | **Accepted Codex's finding.** One entry per UID holding the set last emitted (`sync.Map.Swap`), emitted on every change of that set, deleted when the CR stops excluding `.metadata`, and dropped on the deletion path (`ForgetMetadataExcluded`, first statement of each `IsBeingDeleted` branch). Tests: one entry after two different sets, re-emit on returning to an earlier set, empty after the warnings disappear and after Forget. |
| C5 the type assertion cannot panic from the three controllers; a nil object with a live recorder is reachable only through the exported API | CONFIRMED, with the API hardening | CONFIRMED | **Accepted.** The helper takes `client.Object` and returns on nil; test added. |
| C6 no remaining current-tense statement of the old defaults or of the operator writing the list | CONFIRMED (history only) | the design record still said the event count grows per reconcile | **Accepted Cursor's finding.** Sentence corrected; the documentation test fails on the old sentence (measured). FEATURES entry for #17 marked as superseded by #16. |

**Volunteered by Codex, accepted:** `TestMetadataExcludedWarnings` reused one hard-coded UID, so `go test -count=2`
would have hit the process-wide cache on the second iteration; it now uses `uuid.NewUUID()`. (The `controllers`
package as a whole cannot run under `-count=2` at all: Ginkgo refuses repeated suite runs in one process; measured.)

**Gate:** one gate run failed its kustomize check with only "config/default lost --zap-log-level" because the check
discarded kustomize's stderr; the failure did not reproduce on two later runs. The check now fails with kustomize's
own error text.

## Third pass (head aa08842: the one-entry cache, Forget on deletion, nil object)

| Claim | Codex | Cursor | Decision |
|---|---|---|---|
| C1 no interleaving emits twice or zero times for one transition; Forget racing Warn leaves at most one entry | CONFIRMED (MaxConcurrentReconciles defaults to 1; one active reconcile per key) | CONFIRMED | — |
| C2 `sync.Map.Swap` (Go 1.20) is available on every build route | CONFIRMED (go.mod 1.21, Dockerfile Go 1.26, workflows ~1.21) | CONFIRMED | — |
| C3 the set cannot change without a spec change, so the event budget is safe | REFUTED: the set was emitted as one event PER TEMPLATE, so one CR with 25 such templates spent the whole per-object burst by itself; a process restart re-emits the same generation | CONFIRMED that no path changes the set without a spec write; the same precision about restarts | **Accepted Codex's finding.** One event per set, its message the joined per-template messages, or past 1024 bytes a count and a six-byte digest of the set. Tests: 26 templates emit exactly one event naming "26 templates"; a small set names each template. |
| C4 index-keyed messages | CONFIRMED: keep the index (content as key would suppress reorders while old events name stale indices, and would hash or retain manifest text) | CONFIRMED, same reasoning | — |
| C5 Forget placement; the deletion branch never reaches Warn | CONFIRMED | CONFIRMED | — |
| C6 the cache test cannot interfere with other tests; FakeRecorder capacity sufficient | CONFIRMED from source (no `t.Parallel`; only this file touches the map) | CONFIRMED | — |
| C7 the gate's temp file leaks on the failure path | CONFIRMED, trap proposed | CONFIRMED leak, acceptable | **Accepted the trap** (two lines); the proposed grep-the-script assertion in `hack/lib_test.sh` declined: it tests the script's text, not its behaviour. |

**Volunteered by Codex, accepted:** a typed nil pointer inside a non-nil `client.Object` interface passed the nil
check and panicked in `GetUID`; both helpers now treat it as absent (reflect), with a test.

**Volunteered by Cursor and Codex, accepted:** the design record's "once per process per CR and message set" described
the second-pass cache, not the third; it now states the one-event, last-set contract and that a return to an earlier
set emits again, and the documentation test fails on the old sentence (measured).

**Volunteered by Cursor, accepted:** the gate's overlay kustomize still discarded stderr; both renders now write it
to one temp file that the trap removes on any exit.

**Deferred from the first pass, done now:** the README default-paths check requires the numbered block to end where
the prose resumes, so a fourth documented item fails the test (measured with an appended `4. .foo`).

## Fourth pass (head c42ecbf: one event per set, typed nil, the trap)

| Claim | Codex | Cursor | Decision |
|---|---|---|---|
| C1 the API server limits a core/v1 event message to 1024 bytes and the recorder does not truncate | REFUTED: the limit applies only to events with `eventTime` set; the legacy recorder leaves it zero | PLAUSIBLE: no limit in apimachinery or the core/v1 type; the 1 kB figure belongs to `events.k8s.io` `note`; kubernetes validation not in the module cache | **Refuted here from the source** (kubernetes v1.28.2 `pkg/apis/core/validation/events.go`: `NoteLengthLimit` is checked in `legacyValidateEvent`'s `else` branch, entered only when `eventTime` is set). The code comment that claimed a server limit is retracted; 1024 bytes is now stated as this package's display budget. |
| C2 no path emits over 1024 bytes; the summary starts at the eighth template | CONFIRMED (7 templates: 1006 bytes; 8: 1150; indices with four digits can tip seven over) | CONFIRMED (threshold 8; the chart's largest default CR has 3 templates, not 4) | — (boundary test added) |
| C3 the digest is a human fingerprint, not a boundary; indices would serve the reader better | CONFIRMED, indices first, digest as a last fallback | CONFIRMED, indices first, count-only fallback, drop the digest | **Accepted:** indices when they fit (up to about 190 templates), the count alone beyond that; sha256 dropped. Tests: 26 templates name indices 0..25; 8 templates end with "(templates 0, ..., 7)"; 400 fall back to the count. |
| C4 `isNilObject` covers every pointer-backed `client.Object`; non-pointer implementations proceed | CONFIRMED | CONFIRMED | — |
| C5 tests order-independent under `-shuffle=on` | CONFIRMED | CONFIRMED | — (measured: `go test -shuffle=on ./controllers/common/` passes) |
| C6 the single EXIT trap runs on every `fail` path and is cleared after success | CONFIRMED | CONFIRMED | — |

**Volunteered by Codex, accepted:** the design-record assertion checked only "return to an earlier set"; it now
requires the complete last-set sentence.

## Fifth pass, confirmation (head 74d5e83: the indexed summary)

Both reviewers confirmed every claim and reported nothing meeting the fix-plus-test bar. Codex cited kubernetes
v1.28.2 `events.go` lines 143-157 and 184-186 for the `eventTime` branch; Cursor confirmed from client-go v0.28.2
`makeEvent` (no `EventTime`) and controller-runtime v0.15.2's recorder provider (legacy `record` broadcaster). Both
measured the indexed summary's exact ceiling at 192 consecutive templates and the count-only fallback at under 200
bytes, and both showed each of the three message tiers is locked by a test that fails when that tier is removed.
Cursor's advice on the message function's signature, accepted: keep it; the only production caller derives both
arguments from the same templates. Gate green on 74d5e83. Clean.

## Note on commit identifiers

On 2026-09-05, after the merge, the five commits from the second-review commit to the #41 merge were rewritten to
remove an attribution trailer from their messages (trees unchanged, verified byte-identical). Heads cited above by
their old identifiers map as follows: aa08842 → c421233, c42ecbf → 7d0bfe4, 74d5e83 → 80fdb33, 22c3797 → d221313, 9615e47 → be6b5ab. The image built from the merge before the rewrite reports
`v1.2.6-140-g9615e47`; its content equals `be6b5ab`.
