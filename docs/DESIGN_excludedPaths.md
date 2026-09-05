# Design record — where the default `excludedPaths` live, and how clusters leave the `.metadata` exclusion

Decided 2026-09-05 after an adversarial design review (Codex gpt-5.6-sol, Cursor Grok 4.6, and a first-principles
Fable 5.1 reviewer with cluster access), each asked the same six questions over two options. Their raw reports are
in the session scratchpad; the decisions and the measurements they rest on are here.

## The problem

`excludedPaths` on a template are the parts of a created object the enforcer never enforces: set at creation,
then left alone. Upstream the operator unioned its defaults (`.metadata`, `.status`, `.spec.replicas`) into every
template's list and WROTE the CR (`IsInitialized`). Three consequences, all measured:

1. every CR differed from what its author or their Git declared; with a GitOps controller healing the spec back,
   the two rewrote it at each other (the chart's 0.21.1 entry);
2. the chart that deploys this operator started mirroring the operator's defaults to keep Git equal to the cluster,
   a second place that has to track the operator forever;
3. after the server-side-apply enforcer made `.metadata` unnecessary as a default, every live CR still carried it,
   written there by the old build, and nothing removed it: labels stayed set-once (issue #16 unresolved on any
   existing cluster).

## The options reviewed

- **A.** The operator stops writing defaults into the spec and applies them in memory when it builds the locked
  resources. Migration of the stale `.metadata`: A1 the operator removes its own former defaults once (it cannot
  know what the author declared: the spec is the only record and it was mutated); A2 a documented one-off patch
  per CR; A3 the chart declares the new set once as a vehicle; A4 (Cursor) a one-shot prune of exactly the old
  default set, gated on a status marker so it runs once, preserving author extras, and letting Git re-add an
  intentional `.metadata`.
- **B.** The chart declares the operator's default set on every template (the parked draft, chart branch
  `feat/16-metadata-no-longer-excluded`, 0.22.0; proven on the sandbox as release revision 10).

## Verdicts and decision

Both reviewers: **Option A**, reject B as the long-term design. Their reasons, re-checked here: the rewrite loop
needs both sides to write the spec, and A removes the operator's side for good; B couples the chart to the
operator's defaults forever, must be repeated in every values file that ever copied the defaults (the sandbox's
own values did), was incomplete as drafted (the GroupConfig templates were missed), and loops against an older
operator image with GitOps self-heal. Neither reviewer found a way A can be made to loop.

Both also named what neither option says out loud: dropping `.metadata` from a CR's list is a forced ownership
transfer of every rendered label and annotation key on every object that CR already created, and the restart of
every locked-resource reconciler (one dry-run apply through admission per object). That is the point of #16, and
it is what to measure on a cluster before merging (below).

**Implemented (this branch):** `common.EffectiveExcludedPaths` (sorted union of defaults and the author's list)
applied in `TemplateFilter.Render`; `IsInitialized` writes finalizers only; `.metadata.finalizers` added to the
defaults (review of PR #40: a template that renders a finalizer must not make the operator re-add it); README, CSV
and WARP corrected, with a test that fails when code and documents disagree.

**Measured on the sandbox with this build:** a fresh NamespaceConfig declaring no excludedPaths keeps the field
absent, its generation moves once (the finalizer), the operator owns and enforces its ConfigMap's rendered label
and leaves a hand-added label alone; existing CRs' generations do not move; no errors.

## Two corrections from the first-principles reviewer, adopted

1. **The finalizer write also wrote the spec.** `Update(ctx, instance)` on the whole typed object serialised the
   non-pointer selector structs as `annotationSelector: {}` and dropped an author's empty lists (measured on the
   sandbox: `manager/Update` owned `f:spec.f:annotationSelector` on the fresh CR). Finalizers now go as a merge
   patch computed against the pre-mutation copy, so only `metadata.finalizers` crosses the wire; a test asserts the
   patch body has no `spec`.
2. **Observability instead of a status field.** A status field would need a CRD change that OLM owns and the chart
   duplicates; instead the operator emits a Warning event (`MetadataExcluded`) naming each template that still
   excludes `.metadata`, so the CRs that keep set-once labels are visible in `oc get events`.

## Migration of the stale `.metadata`: decided

**No automatic prune.** All three reviewers, for the same measured reason: `spec.templates` is one atomic value to
every writer (the API server records `f:templates` whole, Helm replaces the whole list, ArgoCD compares it index by
index), so no field manager, annotation or history can tell the entry the old operator wrote from one the author
declared; a prune would strip an author's `.metadata` and, against a Git that still declares it, reopen the loop.

**For chart-managed CRs: chart 0.22.0 as drafted, reframed.** The chart declares `excludedPaths: [.status,
.spec.replicas]` on every template as its own declared policy, not as a mirror of the operator's defaults (those
changed twice in one day; the mirror was stale as written). Under Option A that declaration cannot loop or diverge
harmfully: the operator never writes the list, and it unions its own defaults in memory whatever the chart says, so
a stale chart list only means a default the chart does not mention is still applied. The declaration is what makes
the migration deterministic: every cluster that installs any release from 0.22.0 on gets its lists rewritten once by
Helm (measured on the sandbox, revision 8 to 10: every CR `[.status, .spec.replicas]`, 40 objects single-owner,
every CR ReconcileSuccess), reconcilers restart without deleting anything, and the enforcer takes ownership of the
rendered labels and annotations. Ordering: the Option A operator image first on every cluster; with the old operator,
a two-path declaration is the 0.21.1 loop under GitOps self-heal.

**For CRs outside the chart:** the Warning event names them; their authors remove `.metadata` from
`excludedPaths` (Git or `oc patch`).

## Measured before merging (this branch, sandbox)

A fresh NamespaceConfig declaring nothing, created while no operator ran, then reconciled by this build: generation
stays 1, `excludedPaths` and `annotationSelector` absent, the operator's manager entry owns only
`metadata.finalizers` (status through the subresource), its ConfigMap owned for `data.a` and the rendered label, a
tampered rendered label restored and a foreign label kept. A CR declaring `.metadata`: one `MetadataExcluded`
Warning event on the CR (in the default namespace, where cluster-scoped objects' events land; measured before the
once-per-process dedup, when its count grew per reconcile; now one event naming every such template, once per
process per CR while that set is unchanged; a transition of the set, including a return to an earlier set, emits
again), its
rendered label set once. The chart's CRs: generations unchanged by the image alone. No errors.
(An earlier attempt at the same measurement read a written spec and generation 2: the in-cluster operator, still
the previous build, had reconciled the probe in the second before the snapshot; recorded so nobody repeats it.)
