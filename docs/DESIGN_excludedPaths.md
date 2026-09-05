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

## Migration of the stale `.metadata`

Open at the time of writing; decided below once the first-principles reviewer's measurement of the CRs' own field
managers (who wrote `spec.templates[].excludedPaths`: the operator or Helm) is in, because that is the one
provenance signal that could make an automatic prune safe.
