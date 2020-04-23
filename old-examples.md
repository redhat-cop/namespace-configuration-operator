## Configuration Examples

Here is a list of use cases in which the Namespace Configuration Controller can be useful.
Examples will be deployed in the `test-namespace-config` (you can pick any other name):

```shell
oc new-project test-namespace-config
```

### T-Shirt Sized Quotas

During the provisioning of the projects to dev teams some organizations start with T-shirt sized quotas. Here is an example of how this can be done with the Namespace Configuration Controller

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: small-size
spec:
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
---
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: large-size
spec:
  selector:
    matchLabels:
      size: large  
  resources:
  - apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: large-size
    spec:
      hard:
        requests.cpu: "8"
        requests.memory: "4Gi"  
```

We can test the above configuration as follows:

```shell
oc apply -f examples/tshirt-quotas.yaml -n test-namespace-config
oc new-project large-project
oc label namespace large-project size=large
oc new-project small-project
oc label namespace small-project size=small
```

### Default Network Policy

Network policy are like firewall rules. There can be some reasonable defaults.
In most cases isolating one project from other projects is a good way to start. In OpenShift this is the default behavior of the multitenant SDN plugin.
The configuration would be as follows:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: multitenant
spec:
  selector:
    matchLabels:
      multitenant: "true"  
  resources:
  - apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: allow-from-same-namespace
    spec:
      podSelector:
      ingress:
      - from:
        - podSelector: {}
  - apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: allow-from-default-namespace
    spec:
      podSelector:
      ingress:
      - from:
        - namespaceSelector:
            matchLabels:
              name: default
```

We can deploy it with the following commands:

```shell
oc apply -f examples/multitenant-networkpolicy.yaml -n test-namespace-config
oc new-project multitenant-project
oc label namespace multitenant-project multitenant=true
```

### Defining the Overcommitment Ratio

I don't personally use limit range much. I prefer to define quotas and let the developers decide if they need a few large pods or many small pods.
That said limit range can still be useful to define the ratio between request and limit, which at the node level will determine the node overcommitment ratio.
Here is how it can be done:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: overcommit-limitrange
spec:
  selector:
    matchLabels:
      overcommit: "limited"  
  resources:
  - apiVersion: "v1"
    kind: "LimitRange"
    metadata:
      name: "overcommit-limits"
    spec:
      limits:
        - type: "Container"
          maxLimitRequestRatio:
            cpu: 100
            memory: 1
```

We can deploy it with the following commands:

```shell
oc apply -f examples/overcommit-limitrange.yaml -n test-namespace-config
oc new-project overcommit-project
oc label namespace overcommit-project overcommit=limited
```

### ServiceAccount with Special Permission

Another scenario is an application that needs to talk to the master API and needs specific permissions to do that. As an example, we are creating a service account with the `registry-viewer` and `registry-editor` accounts. Here is what we can do:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: special-sa
spec:
  selector:
    matchLabels:
      special-sa: "true"
  resources:
  - apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: special-sa
  - apiVersion: authorization.openshift.io/v1
    kind: RoleBinding
    metadata:
      name: special-sa-registry-editor
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: registry-editor
    subjects:
    - kind: ServiceAccount
      name: special-sa
  - apiVersion: authorization.openshift.io/v1
    kind: RoleBinding
    metadata:
      name: special-sa-registry-viewer
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: registry-viewer
    subjects:
    - kind: ServiceAccount
      name: special-sa
```

Here is how it can be deployed:

```shell
oc apply -f examples/serviceaccount-permissions.yaml -n test-namespace-config
oc new-project special-sa
oc label namespace special-sa special-sa=true
```

## Pod with Special Permissions

Another scenario is a pod that needs to run with special permissions, i.e. a custom PodSecurityPolicy, and we don't want to give permission to the dev team to grant PodSecurityPolicy permissions.
In OpenShift SCCs have represented the PodSecurityPolicy since the beginning of the product.
SCCs are not compatible with `namespace-configuration-operator` because of the way SCC profiles are granted to serviceaccounts.
With PodSecurityPolicy, this grant is done simply with a RoleBinding object.
Here is how this might work:

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: forbid-privileged-pods
spec:
  privileged: false
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  runAsUser:
    rule: RunAsAny
  fsGroup:
    rule: RunAsAny
  volumes:
  - '*'
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: forbid-privileged-pods
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - forbid-privileged-pods
---  
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: unprivileged-pods
spec:
  selector:
    matchLabels:
      unprivileged-pods: "true"
  resources:
  - apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: unprivileged-pods
  - apiVersion: authorization.openshift.io/v1
    kind: RoleBinding
    metadata:
      name: unprivileged-pods-rb
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: forbid-privileged-pods
    subjects:
    - kind: ServiceAccount
      name: unprivileged-pods
```

Here is how this example can be run:

```shell
oc apply -f examples/special-pod.yaml -n test-namespace-config
oc new-project special-pod
oc label namespace special-pod unprivileged-pods=true
```

## Cleaning up

To clean up the previous example you can run the following:

```shell
oc delete -f examples/special-pod.yaml -n test-namespace-config
oc delete -f examples/serviceaccount-permissions.yaml -n test-namespace-config
oc delete -f examples/overcommit-limitrange.yaml -n test-namespace-config
oc delete -f examples/multitenant-networkpolicy.yaml -n test-namespace-config
oc delete -f examples/tshirt-quotas.yaml -n test-namespace-config
oc delete project special-pod special-sa overcommit-project multitenant-project small-project large-project test-namespace-config
```