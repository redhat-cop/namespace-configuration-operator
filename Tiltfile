# -*- mode: Python -*-

compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager main.go'
image = 'quay.io/' + os.environ['repo'] + '/namespace-configuration-operator'

local_resource(
  'namespace-configuration-operator-compile',
  compile_cmd,
  deps=['./main.go','./api','./controllers'])


custom_build(
  image,
  'podman build -t $EXPECTED_REF --ignorefile ci.Dockerfile.dockerignore -f ./ci.Dockerfile .  && podman push $EXPECTED_REF $EXPECTED_REF',
  entrypoint=['/manager'],
  deps=['./bin'],
  live_update=[
    sync('./bin/manager',"/manager"),
  ],
  skips_local_docker=True,
)

allow_k8s_contexts(k8s_context())
k8s_yaml(kustomize('./config/local-development/tilt'))
k8s_resource('namespace-configuration-operator-controller-manager',resource_deps=['namespace-configuration-operator-compile'])