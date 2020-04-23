# Namespace Configuration Operator

[![Build Status](https://travis-ci.org/redhat-cop/namespace-configuration-operator.svg?branch=master)](https://travis-ci.org/redhat-cop/namespace-configuration-operator) [![Docker Repository on Quay](https://quay.io/repository/redhat-cop/namespace-configuration-operator/status "Docker Repository on Quay")](https://quay.io/repository/redhat-cop/namespace-configuration-operator)

## Introduction

The `namespace-configuration-operator helps keeping configurations related to Users, Groups and Namespaces aligned with one of more policies specified as a CRs. The purpose is to provide the foundational building block to create an end-to-end onboarding process. 
By onboarding process we mean all the provisioning steps needed to a developer team working on one or more applications to OpenShift.
This usually involves configuring resources such as: Groups, RoleBindings, Namespaces, ResourceQuotas, NetworkPolicies, EgressNetworkPolicies, etc.... . Depending on the specific environment the list could continue.
Naturally such a process should be as automatic and scalable as possible.

With the namespace-configuration-operator one can create rules that will react to the creation of Users, Groups and Namespace and will create and enforce a set of resources.

Here are some examples of the type of onboarding processes that one could support:

1. [developer sandbox](./examples/user-sandbox/readme.md)
2. [team onboarding](./team-onboarding/readme.md) with support of the entire SDLC in a multitentant environment.

Policies can be expressed with the following CRDs:

| Watched Resource | CRD |
|--|--|
| Groups | [GroupConfig](#GroupConfig) |
| Users | [UserConfig](#UserConfig) |
| Namespace | [NamespaceConfig](#NamespaceConfig) |

These CRDs all share some commonalities:

1. Templated Resources
2. List of ignored jason path

### Templated Resources

Each has a parameter called `templatedResources`, which is an array. Each element of the array has two fields `objectTemplate` and `excludedPaths` (see below).

The `objectTemplate` field must contains a [go template](https://golang.org/pkg/text/template/) that resolves to a single API Resource expressed in `yaml`. The template is merged with the object selected by the CR. For example:

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

This create a rule in which every time a user from the `corp-ldap` provider is created, a namespace called `<username>-sandbox` is also created.

### Excluded Paths

The logic of the namespace-configuration-operator is to enforce that the resources resolved by processing the templates "stays in place". In other words if those resources are changed and/or deleted they will be reset by the operator.
But there are situations in which at least part of a resource is allowed to change. Common use cases are: annotations and in general the metadata section of a resource can be updated by the various operators watching that resources. The status field is often updated by the main operator managing that resources. Finally, when applicable the `spec.replicas` field should also be allowed to change.

To handle special use case, one can also specify additional *jsonpaths* that should be ignored when comparing the desired resource and the current resource and making a decision on whether that resource should be reset.

The following paths are always included:

1. `.metadata`
2. `.status`
3. `.spec.replicas`

## NamespaceConfig

The `NamespaceConfig` CR allows specifying one or more objects that will be created in the selected namespaces.

Namespaces can be selected by labels or annotations via a label selector for example:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: small-namespace
  selector:
    matchLabels:
      size: small  
  resources:
  - apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: small-size  
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

## GroupConfig

The `GroupConfig` CR allows specifying one or more objects that will be created in the selected Group.
Groups can be selected by labels or annotations via a label selector, similarly to the `NamespaceConfig`.

Often groups are created in OpenShift by a job that synchronizes an Identity Provider with OCP. So the idea is that when new groups are added or deleted the configuration in OpenShift will adapt automatically.

Although not enforced by the operator, GroupConfig are expected to create cluster-scoped resources like Namespaces, ClusterResourceQuotas and potentially some namespaced resources like RoleBindings.

## UserConfig

In OpenShift an external user is defined by two entities: Users and Identities. There is a relationship of on to many between Users and Identities. Given one user, there can be one Identity per authentication mechanism.

The `UserConfig` CR allows specifying one or more objects that will be created in the selected User.
Users can be selected by label or annotation like `NamespaceConfig` and `UserConfig`.
USers can also be selected by provider name (the name of the authentication mechanism) and identity extra field.

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

## Deploying the Operator

This is a cluster-level operator that you can deploy in any namespace, `namespace-configuration-operator` is recommended.

You can either deploy it using [`Helm`](https://helm.sh/) or creating the manifests directly.

NOTE:
**Given that a number of elevated permissions are required to create resources at a cluster scope, the account you are currently logged in must have elevated rights.**

### Deploying with Helm

Here are the instructions to install the latest release with Helm.

```shell
oc new-project namespace-configuration-operator

helm repo add namespace-configuration-operator https://redhat-cop.github.io/namespace-configuration-operator
helm repo update
export namespace_configuration_operator_chart_version=$(helm search namespace-configuration-operator/namespace-configuration-operator | grep namespace-configuration-operator/namespace-configuration-operator | awk '{print $2}')

helm fetch namespace-configuration-operator/namespace-configuration-operator --version ${namespace_configuration_operator_chart_version}
helm template namespace-configuration-operator-${namespace_configuration_operator_chart_version}.tgz --namespace namespace-configuration-operator | oc apply -f - -n namespace-configuration-operator

rm namespace-configuration-operator-${namespace_configuration_operator_chart_version}.tgz
```

### Deploying directly with manifests

Here are the instructions to install the latest release creating the manifest directly in OCP.

```shell
git clone git@github.com:redhat-cop/namespace-configuration-operator.git; cd namespace-configuration-operator
oc apply -f deploy/crds/redhatcop_v1alpha1_namespaceconfig_crd.yaml
oc new-project namespace-configuration-operator
oc -n namespace-configuration-operator apply -f deploy
```

## Local Development

Execute the following steps to develop the functionality locally. It is recommended that development be done using a cluster with `cluster-admin` permissions.

```shell
go mod download
```

optionally:

```shell
go mod vendor
```

Using the [operator-sdk](https://github.com/operator-framework/operator-sdk), run the operator locally:

```shell
oc apply -f deploy/crds/redhatcop.redhat.io_namespaceconfigs_crd.yaml
oc apply -f deploy/crds/redhatcop.redhat.io_groupconfigs_crd.yaml
oc apply -f deploy/crds/redhatcop.redhat.io_userconfigs_crd.yaml
OPERATOR_NAME='namespace-configuration-operator' operator-sdk --verbose run  --local --watch-namespace "" --operator-flags="--zap-level=debug"
```

## Test

### Testing NamespaceConfig

```shell
oc apply -f ./test/namespace-config-test.yaml
oc apply -f ./test/namespaces.yaml
```

### Testing GroupConfig

```shell
oc apply -f ./test/group-config-test.yaml
oc apply -f ./test/groups.yaml
```

### Testing UserConfig

```shell
oc apply -f ./test/user-config-test.yaml
oc apply -f ./test/users.yaml
for username in test-user-config test-user-config2 ; do
export username
export uid=$(oc get user $username -o jsonpath='{.metadata.uid}')
cat ./test/identities.yaml | envsubst | oc apply -f -
done
```

## Release Process

To release execute the following:

```shell
git tag -a "<version>" -m "release <version>"
git push upstream <version>
```

use this version format: vM.m.z
