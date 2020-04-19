# Testing

## Testing UserConfig

```shell
oc apply -f ./test/user-identity.yaml
oc apply -f ./test/user-config-test.yaml
```

## Testing GroupConfig

```shell
oc apply -f ./test/group.yaml
oc apply -f ./test/group-config-test.yaml
```

