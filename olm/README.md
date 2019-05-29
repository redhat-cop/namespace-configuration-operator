# instructions on how to manually test the olm integration

Get the quay token

```shell
AUTH_TOKEN=$(curl -sH "Content-Type: application/json" -XPOST https://quay.io/cnr/api/v1/users/login -d '
{
    "user": {
        "username": "'"${QUAY_USERNAME}"'",
        "password": "'"${QUAY_PASSWORD}"'"
    }
}' | jq -r '.token')
```

validate the olm CSV

```shell
operator-courier verify olm/olm-catalog/
operator-courier verify olm/olm-catalog/ --ui_validate_io
```

go to this [site](https://operatorhub.io/preview) to visually validate the result

push the catalog to the quay application registry

```shell
operator-courier push olm/olm-catalog/ <your-quay-repo> namespace-configuration-operator 0.0.1 "${AUTH_TOKEN}"
```

deploy the operator source

```shell
oc apply -f ./olm/operator-source.yaml
```

to delete a wrong bundle run:
```shell
helm registry login -p <quay-password> -u <quay-username> quay.io/<your-quay-repo>/namespace-configuration-operator 
helm registry delete-package quay.io/<your-quay-repo>/namespace-configuration-operator@0.0.1
```