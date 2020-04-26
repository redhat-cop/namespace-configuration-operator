# Developer SandBox

This example use case is about creating an onboarding process for a cluster in which users get their own sandbox and can freely experiment.
The requirements for this onboarding process is the following:

1. user logging in with the provider `my-provider` get a sandbox, which is a namespace with name `<username>-sandbox`
2. this namespace will have a resource quota defined on it to limit the resources usable by each user.
3. user cannot communicate with anything else within the corporate network (represented by this CIDR: `10.20.0.0/0`), but they can open connections to Internet services.
4. by default sandboxes cannot communicate with other sandboxes, but user are given the ability to connect different sandboxes by managing their own network policies.

An `UserConfig` CR that would satisfy those requirements would look like this:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: UserConfig
metadata:
  name: user-sandbox
spec:
  providerName: my-provider
  templates:
  - objectTemplate: |
      apiVersion: v1
      kind: Namespace
      metadata:
        name: {{ .Name }}-sandbox
  - objectTemplate: |
      apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: {{ .Name }}-sandbox
        namespace: {{ .Name }}-sandbox
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: admin
      subjects:
      - kind: User
        apiGroup: rbac.authorization.k8s.io
        name: {{ .Name }}
  - objectTemplate: |
      apiVersion: v1
      kind: ResourceQuota
      metadata:
        name: standard-sandbox
        namespace: {{ .Name }}-sandbox
      spec:
        hard:
          requests.cpu: "1"
          requests.memory: 1Gi
          requests.ephemeral-storage: 2Gi
  - objectTemplate: |
      kind: EgressNetworkPolicy
      apiVersion: network.openshift.io/v1
      metadata:
        name: air-gapped-sandbox
        namespace: {{ .Name }}-sandbox
      spec:
        egress:
        - type: Deny
          to:
            cidrSelector: 10.20.0.0/0
  - objectTemplate: |
      apiVersion: networking.k8s.io/v1
      kind: NetworkPolicy
      metadata:
        name: allow-from-same-namespace
        namespace: {{ .Name }}-sandbox
      spec:
        podSelector:
        ingress:
        - from:
          - podSelector: {}
        policyTypes:
          - Ingress
  - objectTemplate: |
      apiVersion: networking.k8s.io/v1
      kind: NetworkPolicy
      metadata:
        name: allow-from-openshift-ingress
        namespace: {{ .Name }}-sandbox
      spec:
        ingress:
        - from:
          - namespaceSelector:
              matchLabels:
                network.openshift.io/policy-group: ingress
        podSelector: {}
        policyTypes:
        - Ingress
```

Here is how you can test it:

```shell
oc apply -f ./examples/user-sandbox/user-config.yaml
oc apply -f ./examples/user-sandbox/users.yaml
for username in user1 user2 ; do
export username
export uid=$(oc get user $username -o jsonpath='{.metadata.uid}')
cat ./examples/user-sandbox/identities.yaml | envsubst | oc apply -f -
done
```

now impersonate either `user1` or `user2` and explore.

```shell
oc get projects --as=user1
```
