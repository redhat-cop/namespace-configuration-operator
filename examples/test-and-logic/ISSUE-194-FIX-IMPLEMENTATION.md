# Issue #194 — Fix Implementation (Forked operator-utils)

Repository/Branch/Commit
- Fork: github.com/ephico2real2/operator-utils
- Branch: fix-issue-194-field-removal-zero-value
- Commit: 9569465 — "Fix issue #194: Remove fields with value 0 when conditionals change"

Goal
- Ensure that when a field is missing in the rendered (expected) object but present in the live (actual) object, the patch explicitly removes that field — even when the live value is "zero-like" (e.g., "0").

High‑level approach
- Use JSON Merge Patch semantics to delete fields by setting them to null in the patch.
- Before creating the patch, walk the expected vs. actual maps and add null entries for any keys present in actual but missing in expected. This instructs Kubernetes to remove those fields.

Key code changes (summary)
- Added helper addNullFieldsForMissing(expected, actual, patchMap):
  - Recursively traverses both objects (as map[string]any).
  - For any key missing in expected but present in actual, sets patchMap[key] = nil.
  - For nested maps present in both, recurse to find deeper missing keys.
- Added createPatchWithNullFields(expected, actual):
  - Builds a patch map containing:
    - Differences between expected and actual (as before), and
    - Null entries for "present in actual, missing in expected" keys via addNullFieldsForMissing.
  - Serializes patch map as application/merge-patch+json.
- Updated reconciliation path to use createPatchWithNullFields so removals are included when applying the patch.

Why this fixes the bug
- Previously, when a conditional removed a field from the template, the patch often did not request deletion of the stale field. Kubernetes therefore kept the field (with value "0").
- With the new logic, those missing keys are added as null in the merge patch, which causes Kubernetes to remove them — aligning live state with the rendered template.

Behavioral guarantees
- Field removal works when a condition flips from true → false.
- Field re‑addition continues to work when the condition flips false → true (expected includes the field again, so the normal patch path adds/updates it).
- Works recursively on nested structures (e.g., spec.hard).

Notes and considerations
- This approach relies on JSON Merge Patch behavior: setting a key to null deletes it.
- Excluded paths configured by the operator remain respected (no change to exclusion policy).
- Designed to be generic; not limited to ResourceQuota.

How I wired the forked module (commands)
```bash
# Option A: Track the branch (lightweight)
go get github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value
go mod tidy

# Option B: Pin the exact commit using a pseudo-version
go mod edit -replace \
  github.com/redhat-cop/operator-utils=github.com/ephico2real2/operator-utils@v\
0.0.0-20251208075852-9569465257c1
go mod tidy

# Build and run (local)
./build.sh -o bin/manager main.go
./run-go.sh --skip-build
```

How the pseudo-version was derived (optional)
```bash
# Get the commit hash used for the fix
cd ../operator-utils-fork
git rev-parse HEAD
# 9569465257c18041b4a4483c90aebfc278882387

# Get the UTC timestamp in YYYYMMDDhhmmss
TZ=UTC git show -s --format=%cd --date=format-local:%Y%m%d%H%M%S 9569465257c18041b4a4483c90aebfc278882387
# 20251208075852

# Compose: v0.0.0-<timestamp>-<12-char-commit>
# v0.0.0-20251208075852-9569465257c1
```
