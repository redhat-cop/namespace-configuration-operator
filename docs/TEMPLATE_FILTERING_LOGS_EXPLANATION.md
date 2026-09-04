# Template Filtering Logs Explanation

## Overview

Before a template is rendered for a selected object (a Namespace, Group or User), the operator decides whether
that template can produce an object for it at all. The decision lives in `controllers/common/templatefilter.go`
(`TemplateFilter`); this document explains the log lines it writes. The regex-based filter that earlier versions
of this document described (`suffixPatterns`, `containsPatterns`, "unrecognized conditional") no longer exists.

## Why the filter exists

A guarded template such as

```
{{- if hasPrefix "team-a-" (index .Labels "example.com/team") }}
...
{{- end }}
```

renders to nothing for an object the guard rejects. The renderer cannot represent "nothing": an empty render
becomes the JSON literal `null` and fails with `Object 'Kind' is missing in 'null'`. The filter skips such
objects before the renderer sees them.

## How a decision is made

1. The template is parsed once (cached by its text, with the renderer's own function map).
2. If its top level is a single `if` / `else if` / `else` chain over `hasPrefix`, `hasSuffix`, `contains`,
   `eq`, `ne`, `and`, `or`, `not`, or the truthiness of `.Name` / `(index .Labels "k")` /
   `(index .Annotations "k")`, it is evaluated **statically** against the object's name, labels and annotations.
3. Anything else (a top-level variable, a pipeline, `range`, `with`, `.Spec` access, an unknown function) is
   decided by **rendering** the template against the same value the renderer receives and checking whether the
   output parses to an object (a comment-only or `---`-only output does not count).
4. A template that does not parse is left to the renderer, so the parse error is reported there and fails the
   reconcile with a `ReconcileError` condition and a Warning event.

## Log lines

| verbosity | line | meaning |
|---|---|---|
| V(1) | `skipping namespace - no NamespaceConfig templates match the namespace pattern` (also `group`, `user`) | every template was rejected for this object; nothing is rendered for it |
| V(1) | `template does not parse, leaving it to the renderer` | the template has a syntax error; the reconcile will fail with the error |
| V(1) | `template applicability could not be decided by rendering, leaving it to the renderer` | executing the template errored (e.g. a `required` value missing); the reconcile will fail with the error |
| V(2) | `template applicability decided statically ... applicable=true|false` | step 2 above |
| V(2) | `template applicability decided by rendering ... applicable=true|false` | step 3 above |

Enable them with `--zap-log-level=1` (or `2`), or `ZAP_LOG_LEVEL=1` / `2`; see
`docs/LOG_LEVEL_CONFIGURATION.md`.

## What you will NOT see any more

An error-level `Error unmarshalling json manifest ... Object 'Kind' is missing in 'null'` followed by
`unable to process template for` for every rejected object. Those lines meant an older build was rendering a
rejected object; if they appear, the cluster runs an image from before the filter was rewritten.

## Performance

A statically decided template costs a map lookup and a few string comparisons per object. A template decided by
rendering costs one extra render per object, including any `lookup` calls in its taken branch; keep guards
inside the static grammar when you can.
