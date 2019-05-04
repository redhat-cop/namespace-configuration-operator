# Namespace Configuration Controller

The namespace configuration controller helps keeping a namespace's configuration aligned with one of more policies specified as a CRD.

The NamespaceConfig CR allow specifying one or more objects that will be created in the selected namespaces:

Dev teams may of may not be granted permissions to create these objects. In case they haven't the namespace configuration controller can be a way to enforce namespace configuration policies and govern the way namespaces are configured.

A NamespaceConfig CRD looks as follows:

```yaml
apiVersion: redhat-cop.redhat.io/v1alpha1
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
  - other resources
```

The selector will select the namespaces to which this configuration should be applied.
In this example a quota object is created.

The namespace field of defined resources should not be specified and if it exists it will be overwritten with the namespace name of the namespace to which the configuration is being applied.

## Configuration Examples

Here is a list of use cases in which the Namespace Configuration Controller can be useful

### T-Shirt Sized Quotas

During the provisioning of the projects to dev teams some, organizations start with T-shirt sized quotas. Here is an example of how this can be done with the Namespace Configuration Controller

```yaml
apiVersion: redhat-cop.redhat.io/v1alpha1
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
apiVersion: redhat-cop.redhat.io/v1alpha1
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

```yaml
oc new-project test-namespace-config
oc apply -f examples/tshirt-quotas.yaml -n test-namespace-config
oc new-project large-project
oc label namespace large-project size=large
oc new-project small-project
oc label namespace small-project size=small
```

### Default Network Policy

Network policy are like firwall rules. There can be some reasonable defaults.
In most cases isolating one's project from other project is a good way to start. This in openshift is know as the default beahvior of the multitenant SDN plugin.
The configuration would look as follows:
```
apiVersion: namespaceconfig.raffaelespazzoli.systems/v1alpha1
kind: NamespaceConfig
metadata:
  name: multitenant
spec:
  selector:
    matchLabels:
      multitenant: "true"  
  networkpolicies:
  - apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: allow-from-same-namespace
    spec:
      podSelector:
      ingress:
      - from:
        - podSelector: {}
  - kind: NetworkPolicy
    apiVersion: networking.k8s.io/v1
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
```
oc apply -f examples/multitenant-networkpolicy.yaml
oc new-project multitenant-project
oc label namespace multitenant-project multitenant=true
```

### Defining the Overcommitment Ratio

I don't personally use limit range much. I prefer to define quotas and let the developers decide if they need a few large pods or many small pods.
That said limit range can still be useful to define the ration between request and limit, which at the node level will determined the node overcommit ratio.
Here is how it can be done:

```
apiVersion: namespaceconfig.raffaelespazzoli.systems/v1alpha1
kind: NamespaceConfig
metadata:
  name: overcommit-limitrange
spec:
  selector:
    matchLabels:
      overcommit: "limited"  
  limitranges:    
  - apiVersion: "v1"
    kind: "LimitRange"
    metadata:
      name: "overcommit-limits" 
    spec:
      limits:
        - type: "Container" 
          maxLimitRequestRatio:
            cpu: "100"
            memory: "0" 
```            

We can deploy it with the followng commands:
```
oc apply -f examples/overcommit-limitrange.yaml
oc new-project overcommit-project
oc label namespace overcommit-project overcommit=limited
```

### Distributing the Company CA Bundle to every Pod.

OpenShift is often configured with a self-generated root CA. This means that the pods in the cluster do not have the company CA buundle needed to trust external servers during outbound calls.
Note: this example does not work on OCP 3.11 and later as support for podpreset has been removed.
Here is how the namespace configuration controller to achieve this purpose:

```
apiVersion: namespaceconfig.raffaelespazzoli.systems/v1alpha1
kind: NamespaceConfig
metadata:
  name: company-ca
spec:
  selector:
    matchLabels:
      company_ca_bundle: "true"
  configmaps:
  - apiVersion: v1
    kind: Configmap
    metadata:
      name: company-ca 
    data:
      ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUM2akNDQWRLZ0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREFtTVNRd0lnWURWUVFEREJ0dmNHVnUKYzJocFpuUXRjMmxuYm1WeVFERTFOREl5T1RjNU5Ua3dIaGNOTVRneE1URTFNVFl3TlRVNFdoY05Nak14TVRFMApNVFl3TlRVNVdqQW1NU1F3SWdZRFZRUUREQnR2Y0dWdWMyaHBablF0YzJsbmJtVnlRREUxTkRJeU9UYzVOVGt3CmdnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUNlS1hNcFJNUW1xYnhURGpwV3VSc3kKcHVVbVcyQjRrWmtSZkRZN25KMEY5U0I4VFFHb1JWUGlhMVhBN05wMkdKeTBPK0x5YnNoZkNGbXhDSHBwVXZvTwpRcitjUzMzT3Y0aUhRaFJ1VlVuZ3ZqdFU4dlhYMTR1VkYxWXAzWUZaazRiV3RhTUJGS2FnVm51NmN4cktVU0o3CkVWTHBGTk5QQlRvbTBqdWI0QkIyb21TQjhhRGtRaU1hZmc1MDVYTVBTa3ZOaG15VFFSSlhrQUVFR0NOT3hXT2IKUVkyNlpMSDQ2WnJWRis0cWlnUEQvZnhoNDBPcFpMMjFjQnBhUjEvTFN2aTE3cDREZW9wMlFGbzlzMUJZVnVwZAp0N284amEzNVduSHFmMXp5SGh3S1Fqb0hyL0hlZmFUUkFQYlBFVEwrNEU5cUJiQzBEVG83YVgvcEJ0aG9DL01wCkFnTUJBQUdqSXpBaE1BNEdBMVVkRHdFQi93UUVBd0lDcERBUEJnTlZIUk1CQWY4RUJUQURBUUgvTUEwR0NTcUcKU0liM0RRRUJDd1VBQTRJQkFRQXU3QmpyTzBXMG5ia0FLeFJPLzN1Q2pLcnBYdXRiWVRWQmk5S3JkWktXY21QQgpkNWt4ZWJlSTBNam5SQ29ndEVXcFhGNFF2dEZrMnV6eXhiSmR2VGcxMDVZVmdtS1JFWktvakJnblE0OEVXK1IzCm9EZ3lPbjNCbVJxUGVId1Y5RTNFc1NUcXFrank4dFBvLzlWeWlMT0VNcUV3TWc2d3ZuL2lzRWhOUDJub3lZOFkKV1FsVWlTcHN2VFhBRFlpT3BrT096NVBzM0lubjR2K0lteHAzQ1dXVGt3MGNkQVhReGZXVXAyUU5TZFJpdk5laApoTnBmZkNia2JrdGtYOThMelNSY25DdWVjbVNPNTZtU3QrbldKSEpZVDV5YzZtWkVQY3BlbUJwanowUUg0QzlHCmlSUFlIVWRGQzBUb0JiZUoxWER0RUtIdHZBQWgzNHgrUjN3VzhBbncKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  podpresets:
  - apiVersion: settings.k8s.io/v1alpha1
    kind: PodPreset
    metadata:
      name: company-ca
    spec:
      selector:
      volumeMounts:
        - mountPath: /company-ca
          name: company-ca
          readOnly: true
      volumes:
        - name: company-ca
          secret:
            name: company-ca

```
here it is how it can be deployed:
```
oc apply -f examples/company-ca.yaml
oc new-project company-ca-project
oc label namespace company-ca-project company_ca_bundle=true
```

### ServiceAccount with Special Permission

Another scenario is an application needs to talk to the master API and needs to specific permissions to do that, but we don' want to give the dev team permission to grant those permissoions. As an example, we are creating a service account with regitry-viewer at the cluster level and registry-editor at the namespace level. Here is what we can do:

```
apiVersion: namespaceconfig.raffaelespazzoli.systems/v1alpha1
kind: NamespaceConfig
metadata:
  name: special-sa
spec:
  selector:
    matchLabels:
      special-sa: "true"
  serviceaccounts: 
  - apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: special-sa
  rolebingings:
  - apiVersion: authorization.openshift.io/v1
    kind: RoleBinding
    metadata:
      name: special-sa-rb
    roleRef:
      name: registry-editor
    subjects:
    - kind: ServiceAccount
      name: special-sa
  clusterrolebindings:
  - apiVersion: authorization.openshift.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: special-sa-crb
    roleRef:
      name: registry-viewer
    subjects:
    - kind: ServiceAccount
      name: special-sa 
```
In order to be able to grant these permission we need then namespace-configuratio-controller service account to also have these permissions.
here it is how it can be deployed:
```
oc adm policy add-cluster-role-to-user registry-editor -n namespace-configuration-controller -z namespace-configuration-controller
oc adm policy add-cluster-role-to-user registry-viewer -n namespace-configuration-controller -z namespace-configuration-controller
oc apply -f examples/serviceaccount-permissions.yaml
oc new-project special-sa
oc label namespace special-sa special-sa=true
```

## Pod with Special Permissions

Another scenario is pod that need to run with special permissions, i.e. a custom PodSecurityPolicy and we don't want to give permission to the dev team to grant PodSecurityPolicy permissions.
In OpenbShift SCC have represneted the PodSecurityPolicy since the beginning of the product.
SCCs are not compatible with namesace-configuration-controller because of the way SCCs profiles are granted to serviceaccounts.
With PodSecurityPolicy, this grant is done simply with a RoleBinding object.
Here how this might work:
```
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
apiVersion: namespaceconfig.raffaelespazzoli.systems/v1alpha1
kind: NamespaceConfig
metadata:
  name: unprivileged-pods
spec:
  selector:
    matchLabels:
      unprivileged-pods: "true"
  serviceaccounts: 
  - apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: unprivileged-pods
  rolebingings:
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
Also in this case we need to give additional privileges to the namespace-configuration-controller service account.
Here is how this example can be run:
```
oc apply -f examples/special-pod.yaml
oc adm policy add-cluster-role-to-user forbid-privileged-pods -n namespace-configuration-controller -z namespace-configuration-controller
oc new-project special-pod
oc label namespace special-pod unprivileged-pods=true
``` 

## Deploying the Operator

This is a cluster-level operator that you can deploy in any namespace, `namespace-configuration-operator` is recommeded.

```shell
oc apply -f deploy/crds/redhatcop_v1alpha1_namespaceconfig_crd.yaml
oc new-project namespace-configuration-operator
```

Deploy the cluster resources. Given that a number of elevated permissions are required to resources at a cluster scope the account you are currently logged in must have elevated rights.

```shell
oc apply -f deploy
```

## Local Development

Execute the following steps to develop the functionality locally. It is recommended that development be done using a cluster with `cluster-admin` permissions.

Clone the repository, then resolve all depdendencies using `dep`:

```shell
dep ensure
```

Using the [operator-sdk](https://github.com/operator-framework/operator-sdk), run the operator locally:

```shell
operator-sdk up local --namespace ""
```