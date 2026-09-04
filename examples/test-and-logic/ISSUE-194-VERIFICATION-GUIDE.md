# Issue #194 — Verification & Local Test Guide

What this verifies
- That fields removed by conditional rendering are actually deleted from live resources when the condition turns false, and re‑added when it becomes true again.

Prerequisites
- oc or kubectl access to a test cluster
- namespace-configuration-operator repo (this project)
- Forked operator-utils with the fix: github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value

Wire the fixed dependency
Option A (recommended): let Go resolve the branch and record a pseudo‑version
```bash
# In this repo
go get github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value
go mod tidy
```
Option B: pin the exact pseudo‑version (already validated)
```go
// go.mod replace (example)
replace github.com/redhat-cop/operator-utils => github.com/ephico2real2/operator-utils v0.0.0-20251208075852-9569465257c1
```

Build & Run locally
```bash
# Build with version metadata
./build.sh -o bin/manager main.go

# Run the operator (foreground)
./run-go.sh --skip-build
```

Test configuration
- NamespaceConfig: examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml
- It targets namespaces with label test-issue-194=true and conditionally renders:
  - spec.hard.persistentvolumeclaims: "0" if annotation allow-pvc != "true"

End‑to‑end test steps
1) Initial state — field present
```bash
oc create namespace test-issue-194-ns || true
oc label namespace test-issue-194-ns test-issue-194=true --overwrite
oc apply -f examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml
# Verify field is present
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}' && echo
# Expected: 0
```

2) Condition turns false — field should be removed
```bash
oc annotate namespace test-issue-194-ns allow-pvc=true --overwrite
# Give the operator a few seconds to reconcile (or watch logs)
sleep 8
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}' && echo
# Expected with fix: (empty)
```

3) Condition back to true — field should be added
```bash
oc annotate namespace test-issue-194-ns allow-pvc-  # remove annotation
sleep 8
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}' && echo
# Expected: 0
```

Real‑time proof from the cluster (timestamps + full YAML)
- Use server‑side apply so the API server records a timestamp in managedFields.time.

A) Apply the NamespaceConfig with server‑side apply and capture server time
```bash
# Apply the test manifest via server‑side apply (records managedFields.time)
oc apply -f examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml \
  --server-side --field-manager=issue-194-test

# Show the API server recorded time for the NamespaceConfig
oc get namespaceconfig test-issue-194-field-removal -o json | \
  jq -r '.metadata.managedFields | sort_by(.time) | last | .time'
```

B) Show before/after YAML snapshots directly from the cluster
```bash
# BEFORE (annotation true → field should be removed) — capture full YAML
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml > /tmp/rq-before.yaml

# Apply the toggle
oc annotate namespace test-issue-194-ns allow-pvc=true --overwrite
sleep 8

# AFTER — capture full YAML
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml > /tmp/rq-after.yaml

# Quick diff to visualize field removal
diff -u /tmp/rq-before.yaml /tmp/rq-after.yaml | sed -n '1,200p'
```

C) Extract server-recorded timestamps on the live objects (optional)
```bash
# Namespace server time for the last change
oc get namespace test-issue-194-ns -o json | \
  jq -r '.metadata.managedFields | sort_by(.time) | last | .time'

# ResourceQuota server time for the last change
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o json | \
  jq -r '.metadata.managedFields | sort_by(.time) | last | .time'
```

Sample live YAML (after fix — annotation allow-pvc=true)
```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  annotations:
    rbac.ocp.io/created-by: namespace-configuration-operator
    rbac.ocp.io/source-namespaceconfig: test-issue-194-field-removal
    rbac.ocp.io/test-description: Field removal with value 0 in conditionals
    rbac.ocp.io/test-issue: "194"
  creationTimestamp: "2025-12-08T08:01:31Z"
  labels:
    app.kubernetes.io/managed-by: namespace-configuration-operator
    app.kubernetes.io/version: 0.1.0
    rbac.ocp.io/policy-version: 0.1.0
    rbac.ocp.io/test-scenario: issue-194-field-removal
  name: test-issue-194-quota
  namespace: test-issue-194-ns
  resourceVersion: "14472405"
  uid: 999cfb22-20c6-4406-bca3-367f4ab830d7
spec:
  hard:
    limits.cpu: "2"
    limits.memory: 2Gi
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
status:
  hard:
    limits.cpu: "2"
    limits.memory: 2Gi
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
  used:
    limits.cpu: "0"
    limits.memory: "0"
    pods: "0"
    requests.cpu: "0"
    requests.memory: "0"
```

Original template snippet (for comparison)
```yaml
# examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml
spec:
  hard:
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
    {{- if ne (index .Annotations "allow-pvc") "true" }}
    persistentvolumeclaims: "0"
    {{- end }}
    limits.cpu: "2"
    limits.memory: 2Gi
```

Cleanup
```bash
oc delete namespaceconfig test-issue-194-field-removal --ignore-not-found
oc delete namespace test-issue-194-ns --ignore-not-found
```

Expected outcomes (pass criteria)
- Step 1: value is 0 (field present)
- Step 2: value is empty (field removed)
- Step 3: value is 0 again (field re‑added)

Extra checks (optional)
```bash
# Confirm the controlling annotation state
oc get namespace test-issue-194-ns -o jsonpath='{.metadata.annotations.allow-pvc}' && echo

# Inspect a slice of the YAML to ensure the field is really gone/present
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml | grep -A 10 "spec:" | head -15
```

Notes
- If you change the test namespace name, update the jsonpath commands accordingly.
- You can tail operator logs while running ./run-go.sh to observe reconciliations in real time.
- If you prefer not to pin a pseudo-version, using the branch via `go get ...@fix-issue-194-field-removal-zero-value` is sufficient; run `go mod tidy` afterwards.
