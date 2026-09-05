# Namespace Configuration Operator

![build status](https://github.com/redhat-cop/namespace-configuration-operator/workflows/push/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/redhat-cop/namespace-configuration-operator)](https://goreportcard.com/report/github.com/redhat-cop/namespace-configuration-operator)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/redhat-cop/namespace-configuration-operator)

## Introduction

The `namespace-configuration-operator` helps keeping configurations related to Users, Groups and Namespaces aligned with one of more policies specified as a CRs. The purpose is to provide the foundational building block to create an end-to-end onboarding process.
By onboarding process we mean all the provisioning steps needed to a developer team working on one or more applications to OpenShift.
This usually involves configuring resources such as: Groups, RoleBindings, Namespaces, ResourceQuotas, NetworkPolicies, EgressNetworkPolicies, etc.... . Depending on the specific environment the list could continue.
Naturally such a process should be as automatic and scalable as possible.

With the namespace-configuration-operator one can create rules that will react to the creation of Users, Groups and Namespace and will create and enforce a set of resources.

Here are some examples of the type of onboarding processes that one could support:

1. [developer sandbox](./examples/user-sandbox/readme.md)
2. [team onboarding](./examples/team-onboarding/readme.md) with support of the entire SDLC in a multitenant environment.

Policies can be expressed with the following CRDs:

| Watched Resource | CRD |
|--|--|
| Groups | [GroupConfig](#GroupConfig) |
| Users | [UserConfig](#UserConfig) |
| Namespace | [NamespaceConfig](#NamespaceConfig) |

These CRDs all share some commonalities:

1. [Templated Resources](#Templated-Resources)
2. [List of ignored json paths](#Excluded-Paths)

### Templated Resources

Each has a parameter called `templatedResources`, which is an array. Each element of the array has two fields `objectTemplate` and `excludedPaths` (see below).

The `objectTemplate` field must contain a [go template](https://golang.org/pkg/text/template/) that resolves to a single API Resource expressed in `yaml`. The template is merged with the object selected by the CR. For example:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: UserConfig
metadata:
  name: test-user-config
spec:
  provider: corp-ldap
  templates:
  - objectTemplate: |
      apiVersion: v1
      kind: Namespace
      metadata:
        name: {{ .Name }}-sandbox
```

This creates a rule in which every time a user from the `corp-ldap` provider is created, a namespace called `<username>-sandbox` is also created.

More advanced templating functions found in the popular k8s management tool [Helm](https://helm.sh/) is also available. These functions are further described in the Helm [templating](https://helm.sh/docs/chart_template_guide/function_list/#kubernetes-and-chart-functions) documentation.

#### Conditional templates

An `objectTemplate` may be wrapped in a guard so that it produces an object only for some of the selected
objects — for example, only for namespaces whose label value belongs to one team family:

```yaml
  templates:
  - objectTemplate: |
      {{- if hasPrefix "team-a-" (index .Labels "example.com/team") }}
      apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: team-a-edit
        namespace: {{ .Name }}
      ...
      {{- end }}
```

Where the guard rejects the object the template renders nothing, and the operator skips it for that object
(logged once per reconcile at `--zap-log-level=1` as "skipping ..."). Guards built from `hasPrefix`, `hasSuffix`,
`contains`, `eq`, `ne`, `and`, `or`, `not`, and the truthiness of `.Name` or of `(index .Labels "key")` /
`(index .Annotations "key")` are evaluated without rendering; any other guard is decided by rendering the
template and checking for empty output, which costs one extra render for that template. Either way the answer
is what the renderer would produce. See `controllers/common/templatefilter.go`.

Two details of that contract: the object is passed BY VALUE, so pointer-receiver methods such as `{{ .GetName }}`
are not available (use `.Name`, `.Labels`, `.Annotations`, `index`); and a guard whose taken branch contains only
YAML comments or a bare `---` counts as rendering nothing, so it is skipped rather than failed.

A template that the guard accepts but that then FAILS to render (a parse error, a `required` value that is
missing, invalid YAML, an output with no `kind`) fails the whole reconcile: the CR gets a `ReconcileError`
condition and a Warning event carrying the object name and the error, and nothing is created, changed or
deleted for that CR until the template is fixed. This is deliberate. The library function the operator used
to call returned an empty list with no error on such failures, and the enforcer then deleted everything it had
previously created for that object while the CR still reported success.

Additionally, there are functions not listed within the Helm documentation that are also available outlined in the table below.

| Function  |  Description |
|---------|---------|
| toYaml | takes an interface, marshals it to yaml, and returns a string. |
| fromYaml | converts a YAML document into a map[string]interface{}. |
| fromYamlArray | converts a YAML array into a []interface{}. |
| toToml  | takes an interface, marshals it to toml, and returns a string.|
| toJson | takes an interface, marshals it to json, and returns a string. |
| fromJson  | converts a JSON document into a map[string]interface{}. |
| fromJsonArray | converts a JSON array into a []interface{}. |

An example below of the `lookup`, `toJson`, `b64env`, `required`, and `lower` functions included in the expanded advanced templating functionality.

```golang
templates:
- objectTemplate: |
    - apiVersion: v1
      kind: Namespace
      metadata:
        annotations:
          parentOperatorCreatedOn: '{{ (lookup "v1" "Namespace" "" "namespace-configuration-operator").metadata.creationTimestamp }}'
          sourceTemplate: "{{ toJson . | b64enc }}"
          url: '{{ required "URL annotation on the Group is required!" .Annotations.url }}'
        name: {{ .Name | lower }}
```

For more examples on templates within Helm charts see this Helm [tips and tricks](https://helm.sh/docs/howto/charts_tips_and_tricks/) templating guide.

### Excluded Paths

The logic of the `namespace-configuration-operator` is to enforce that the resources resolved by processing the templates "stays in place". In other words if those resources are changed and/or deleted they will be reset by the operator.
But there are situations in which at least part of a resource is allowed to change. Common use cases are: annotations and in general the metadata section of a resource can be updated by the various operators watching that resources. The status field is often updated by the main operator managing that resources. Finally, when applicable the `spec.replicas` field should also be allowed to change.

To handle special use case, one can also specify additional *jsonpaths* that should be ignored when comparing the desired resource and the current resource and making a decision on whether that resource should be reset.

Rendered labels and annotations are enforced: the operator applies server-side and owns exactly the fields the template renders, so a rendered label changed by hand is restored and a label or annotation added by another actor is left alone. Add a more specific excluded path when another controller must be allowed to change a rendered field.

The following paths are always included:

1. `.metadata.finalizers`
2. `.status`
3. `.spec.replicas`

They are applied when the objects are built, not written into the CR: `spec.templates[].excludedPaths` stays exactly what its author declared. A CR that still lists `.metadata` (older builds wrote it there) keeps the set-once behaviour for labels and annotations on its objects until that entry is removed.

## NamespaceConfig

The `NamespaceConfig` CR allows specifying one or more objects that will be created in the selected namespaces.

Namespaces can be selected by labels or annotations via a label selector for example:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: small-namespace
spec:
  labelSelector:
    matchLabels:
      size: small  
  templates:
  - objectTemplate: |
      apiVersion: v1
      kind: ResourceQuota
      metadata:
        name: small-size
        namespace: {{ .Name }}
      spec:
        hard:
          requests.cpu: "4"
          requests.memory: "2Gi"
```

Here is a `NamespaceConfig` object using a `matchExpressions` selector:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: tier-config
spec:
  annotationSelector:
    matchExpressions:
      - {key: tier, operator: In, values: [gold,silver]}
```

Although not enforced by the operator the general expectation is that the NamespaceConfig CR will be used to create objects inside the selected namespace.

The `default` namespace and all namespaces starting with either `kube-` or `openshift-` are never considered by this operator by default. This is a safety feature to ensure that this operator does not interfere with the core of the system. You can override this behavior by setting the `ALLOW_SYSTEM_NAMESPACES` environment variable to true.

Examples of NamespaceConfig usages can be found [here](./examples/namespace-config/readme.md)

## GroupConfig

The `GroupConfig` CR allows specifying one or more objects that will be created in the selected Group.
Groups can be selected by labels or annotations via a label selector, similarly to the `NamespaceConfig`.

Often groups are created in OpenShift by a job that synchronizes an Identity Provider with OCP. So the idea is that when new groups are added or deleted the configuration in OpenShift will adapt automatically.

Although not enforced by the operator, GroupConfig are expected to create cluster-scoped resources like Namespaces, ClusterResourceQuotas and potentially some namespaced resources like RoleBindings.

## UserConfig

In OpenShift an external user is defined by two entities: Users and Identities. There is a relationship of one to many between Users and Identities. Given one user, there can be one Identity per authentication mechanism.

The `UserConfig` CR allows specifying one or more objects that will be created in the selected User.
Users can be selected by label or annotation like `NamespaceConfig` and `UserConfig`.
Users can also be selected by provider name (the name of the authentication mechanism) and identity extra field.

Here is an example:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: UserConfig
metadata:
  name: test-user-config
spec:
  providerName: okta-provider
  identityExtraFieldSelector:
    matchLabels:
      sandbox_enabled: "true"
  templates:
  - objectTemplate: |
      apiVersion: v1
      kind: Namespace
      metadata:
        name: {{ .Name }}-sandbox
```

User will be selected by this `UserConfig` only if they login via the *okta-provider* and if the extra field was populate with the label `sandbox_enabled: "true"`. Note that not all authentication provider allow populating the extra fields in the Identity object.

## CR status

The CR status will display the outcome of the last reconcile cycle, plus any error regarding specific resources. Notice that in the past the operator was displaying also successful reconcile statuses for watched resources. Removing the status about successful resources allows for the operator to manage more resources with a single configuration (there is a limit to how big a CR can be).

## Deploying the Operator

This is a cluster-level operator that you can deploy in any namespace, `namespace-configuration-operator` is recommended.

It is recommended to deploy this operator via [`OperatorHub`](https://operatorhub.io/), but you can also deploy it using [`Helm`](https://helm.sh/).

### Deploying from OperatorHub

> **Note**: This operator supports being installed disconnected environments

If you want to utilize the Operator Lifecycle Manager (OLM) to install this operator, you can do so in two ways: from the UI or the CLI.

### Multiarch Support

| Arch  | Support  |
|:-:|:-:|
| amd64  | ✅ |
| arm64  | ✅  |
| ppc64le  | ✅  |
| s390x  | ✅  |

#### Deploying from OperatorHub UI

* If you would like to launch this operator from the UI, you'll need to navigate to the OperatorHub tab in the console.Before starting, make sure you've created the namespace that you want to install this operator to with the following:

```shell
oc new-project namespace-configuration-operator
```

* Once there, you can search for this operator by name: `namespace configuration`. This will then return an item for our operator and you can select it to get started. Once you've arrived here, you'll be presented with an option to install, which will begin the process.
* After clicking the install button, you can then select the namespace that you would like to install this to as well as the installation strategy you would like to proceed with (`Automatic` or `Manual`).
* Once you've made your selection, you can select `Subscribe` and the installation will begin. After a few moments you can go ahead and check your namespace and you should see the operator running.

![Namespace Configuration Operator](./media/namespace-configuration-operator.png)

#### Deploying from OperatorHub using CLI

If you'd like to launch this operator from the command line, you can use the manifests contained in this repository by running the following:

```shell
oc new-project namespace-configuration-operator
oc apply -f config/operatorhub -n namespace-configuration-operator
```

This will create the appropriate OperatorGroup and Subscription and will trigger OLM to launch the operator in the specified namespace.

You can set `ALLOW_SYSTEM_NAMESPACES` environment variable in `Subscription` like this;

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: namespace-configuration-operator
spec:
  channel: alpha
  config:
    env:
    - name: ALLOW_SYSTEM_NAMESPACES
      value: true
  installPlanApproval: Automatic
  name: namespace-configuration-operator
  source: community-operators
  sourceNamespace: openshift-marketplace
```

### Deploying with Helm

Here are the instructions to install the latest release with Helm.

```shell
oc new-project namespace-configuration-operator
helm repo add namespace-configuration-operator https://redhat-cop.github.io/namespace-configuration-operator
helm repo update
helm install namespace-configuration-operator namespace-configuration-operator/namespace-configuration-operator
```

This can later be updated with the following commands:

```shell
helm repo update
helm upgrade namespace-configuration-operator namespace-configuration-operator/namespace-configuration-operator
```

## Metrics

Prometheus compatible metrics are exposed by the Operator and can be integrated into OpenShift's default cluster monitoring. To enable OpenShift cluster monitoring, label the namespace the operator is deployed in with the label `openshift.io/cluster-monitoring="true"`.

```shell
oc label namespace <namespace> openshift.io/cluster-monitoring="true"
```

### Testing metrics

```sh
export operatorNamespace=namespace-configuration-operator-local # or namespace-configuration-operator
oc label namespace ${operatorNamespace} openshift.io/cluster-monitoring="true"
oc rsh -n openshift-monitoring -c prometheus prometheus-k8s-0 /bin/bash
export operatorNamespace=namespace-configuration-operator-local # or namespace-configuration-operator
curl -v -s -k -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://namespace-configuration-operator-controller-manager-metrics.${operatorNamespace}.svc.cluster.local:8443/metrics
exit
```

## Development

### Running the operator locally

```shell
export repo=raffaelespazzoli
docker login quay.io/$repo
oc new-project namespace-configuration-operator
oc project namespace-configuration-operator
envsubst < config/local-development/tilt/env-replace-image.yaml > config/local-development/tilt/replace-image.yaml
tilt up
```

### Test helm chart locally

Define an image and tag. For example...

```shell
export imageRepository="quay.io/redhat-cop/namespace-configuration-operator"
export imageTag="$(git -c 'versionsort.suffix=-' ls-remote --exit-code --refs --sort='version:refname' --tags https://github.com/redhat-cop/namespace-configuration-operator.git '*.*.*' | tail -n 1 | cut -d '/' -f 3)"
```

Deploy chart...

```shell
make helmchart IMG=${imageRepository} VERSION=${imageTag}
helm upgrade -i namespace-configuration-operator-local charts/namespace-configuration-operator -n namespace-configuration-operator-local --create-namespace
```

Delete...

```shell
helm delete namespace-configuration-operator-local -n namespace-configuration-operator-local
kubectl delete -f charts/namespace-configuration-operator/crds/crds.yaml
```

### Building/Pushing the operator image

```shell
export repo=raffaelespazzoli #replace with yours
docker login quay.io/$repo/namespace-configuration-operator
make docker-build IMG=quay.io/$repo/namespace-configuration-operator:latest
make docker-push IMG=quay.io/$repo/namespace-configuration-operator:latest
```

This fork also carries two scripts that keep the local flow out of `.github/workflows/`:

```shell
hack/local-ci.sh                 # gofmt, go vet, go build, go test -race; LOCAL_CI_IMAGE=1 adds a container build
hack/push-quay.sh                # builds linux/amd64 with podman, pushes an immutable tag (git describe)
PUSH_LATEST=1 hack/push-quay.sh  # ...and moves :latest, which is what running clusters pull
```

Both explain their choices in their headers; `local-ci.sh` in particular says why it does not run `make test`.

The same image is also built by CI: `.github/workflows/image.yaml` runs on every push to a `feature/**` or
`fix/**` branch and on manual dispatch, runs `hack/local-ci.sh` as its gate, and pushes
`quay.io/ephico2real/namespace-configuration-operator:<git describe>` (plus `sha-<short>`); a manual run can also
move `:latest`. It needs the `REGISTRY_USERNAME` / `REGISTRY_PASSWORD` repository secrets (a quay robot). From a
laptop:

```shell
hack/ci-image.sh run [--latest]   # dispatch the workflow for the pushed current branch and follow it
hack/ci-image.sh status           # recent runs
hack/ci-image.sh pull [tag]       # pull what CI built for HEAD (or a tag) and print its labels and version stamp
```

### Deploy to OLM via bundle

```shell
make manifests
make bundle IMG=quay.io/$repo/namespace-configuration-operator:latest
operator-sdk bundle validate ./bundle --select-optional name=operatorhub
make bundle-build BUNDLE_IMG=quay.io/$repo/namespace-configuration-operator-bundle:latest
docker login quay.io/$repo/namespace-configuration-operator-bundle
docker push quay.io/$repo/namespace-configuration-operator-bundle:latest
operator-sdk bundle validate quay.io/$repo/namespace-configuration-operator-bundle:latest --select-optional name=operatorhub
oc new-project namespace-configuration-operator
oc label namespace namespace-configuration-operator openshift.io/cluster-monitoring="true"
operator-sdk cleanup namespace-configuration-operator -n namespace-configuration-operator
operator-sdk run bundle --install-mode AllNamespaces -n namespace-configuration-operator quay.io/$repo/namespace-configuration-operator-bundle:latest
```

### Testing

#### Testing NamespaceConfig

```shell
oc apply -f ./test/namespace-config-test.yaml
oc apply -f ./test/namespaces.yaml
```

#### Testing GroupConfig

```shell
oc apply -f ./test/group-config-test.yaml
oc apply -f ./test/groups.yaml
```

#### Testing UserConfig

```shell
oc apply -f ./test/user-config-test.yaml
oc apply -f ./test/users.yaml
for username in test-user-config test-user-config2 ; do
export username
export uid=$(oc get user $username -o jsonpath='{.metadata.uid}')
cat ./test/identities.yaml | envsubst | oc apply -f -
done
```

### Releasing

```shell
git tag -a "<tagname>" -m "<commit message>"
git push upstream <tagname>
```

If you need to remove a release:

```shell
git tag -d <tagname>
git push upstream --delete <tagname>
```

If you need to "move" a release to the current main

```shell
git tag -f <tagname>
git push upstream -f <tagname>
```

### Cleaning up

```shell
operator-sdk cleanup namespace-configuration-operator -n namespace-configuration-operator
oc delete operatorgroup operator-sdk-og
oc delete catalogsource namespace-configuration-operator-catalog
```
