CHART_REPO_URL ?= http://example.com
HELM_REPO_DEST ?= /tmp/gh-pages
OPERATOR_NAME ?=$(shell basename -z `pwd`)
HELM_VERSION ?= v3.11.0
KIND_VERSION ?= v0.20.0
KUBECTL_VERSION ?= v1.27.3
K8S_MAJOR_VERSION ?= 1.27
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.11.1
# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.31.0
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION ?= 1.26.0

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.1

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# example.com/memcached-operator-bundle:$VERSION and example.com/memcached-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= quay.io/redhat-cop/$(OPERATOR_NAME)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.21

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	
##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: kind-setup
kind-setup: kind kubectl helm
	$(KIND) delete cluster
	$(KIND) create cluster --image docker.io/kindest/node:$(KUBECTL_VERSION) --config=./integration/cluster-kind.yaml
	$(HELM) upgrade ingress-nginx ./integration/helm/ingress-nginx -i --create-namespace -n ingress-nginx --atomic
	$(KUBECTL) wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize kubectl ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize kubectl ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize kubectl ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize kubectl ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
KUSTOMIZE ?= $(LOCALBIN)/kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || { curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || echo "Downloading controller-gen to ${CONTROLLER_GEN}..." && GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
ENVTEST ?= $(LOCALBIN)/setup-envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || echo "Downloading setup-envtest to ${ENVTEST}..." && GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests --interactive=false -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM ?= $(LOCALBIN)/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

# Generate helm chart
.PHONY: helmchart
helmchart: helmchart-clean kustomize helm
	mkdir -p ./charts/${OPERATOR_NAME}/templates
	mkdir -p ./charts/${OPERATOR_NAME}/crds
	repo=${OPERATOR_NAME} envsubst < ./config/local-development/tilt/env-replace-image.yaml > ./config/local-development/tilt/replace-image.yaml
	$(KUSTOMIZE) build ./config/helmchart -o ./charts/${OPERATOR_NAME}/templates
	sed -i 's/release-namespace/{{.Release.Namespace}}/' ./charts/${OPERATOR_NAME}/templates/*.yaml
	rm ./charts/${OPERATOR_NAME}/templates/v1_namespace_release-namespace.yaml ./charts/${OPERATOR_NAME}/templates/apps_v1_deployment_${OPERATOR_NAME}-controller-manager.yaml
	mv ./charts/${OPERATOR_NAME}/templates/apiextensions.k8s.io_v1_customresourcedefinition* ./charts/${OPERATOR_NAME}/crds
	cp ./config/helmchart/templates/* ./charts/${OPERATOR_NAME}/templates
	version=${VERSION} envsubst < ./config/helmchart/Chart.yaml.tpl  > ./charts/${OPERATOR_NAME}/Chart.yaml
	version=${VERSION} image_repo=$${IMG%:*} envsubst < ./config/helmchart/values.yaml.tpl  > ./charts/${OPERATOR_NAME}/values.yaml
	sed -i '1s/^/{{ if .Values.enableMonitoring }}/' ./charts/${OPERATOR_NAME}/templates/monitoring.coreos.com_v1_servicemonitor_${OPERATOR_NAME}-controller-manager-metrics-monitor.yaml
	echo {{ end }} >> ./charts/${OPERATOR_NAME}/templates/monitoring.coreos.com_v1_servicemonitor_${OPERATOR_NAME}-controller-manager-metrics-monitor.yaml
	$(HELM) lint ./charts/${OPERATOR_NAME}	

.PHONY: helmchart-repo
helmchart-repo: helmchart
	mkdir -p ${HELM_REPO_DEST}/${OPERATOR_NAME}
	$(HELM) package -d ${HELM_REPO_DEST}/${OPERATOR_NAME} ./charts/${OPERATOR_NAME}
	$(HELM) repo index --url ${CHART_REPO_URL} ${HELM_REPO_DEST}

.PHONY: helmchart-repo-push
helmchart-repo-push: helmchart-repo	
	git -C ${HELM_REPO_DEST} add .
	git -C ${HELM_REPO_DEST} status
	git -C ${HELM_REPO_DEST} commit -m "Release ${VERSION}"
	git -C ${HELM_REPO_DEST} push origin "gh-pages" 

HELM_TEST_IMG_NAME ?= ${OPERATOR_NAME}
HELM_TEST_IMG_TAG ?= helmchart-test

# Deploy the helmchart to a kind cluster to test deployment.
# If the test-metrics sidecar in the prometheus pod is ready, the metrics work and the test is successful.
.PHONY: helmchart-test
helmchart-test: kind-setup helmchart
	$(MAKE) IMG=${HELM_TEST_IMG_NAME}:${HELM_TEST_IMG_TAG} docker-build
	docker tag ${HELM_TEST_IMG_NAME}:${HELM_TEST_IMG_TAG} docker.io/library/${HELM_TEST_IMG_NAME}:${HELM_TEST_IMG_TAG}
	$(KIND) load docker-image ${HELM_TEST_IMG_NAME}:${HELM_TEST_IMG_TAG} docker.io/library/${HELM_TEST_IMG_NAME}:${HELM_TEST_IMG_TAG}
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --version v1.7.1 --set installCRDs=true
	$(HELM) repo add prometheus-community https://prometheus-community.github.io/helm-charts
	$(HELM) install kube-prometheus-stack prometheus-community/kube-prometheus-stack -n default -f integration/kube-prometheus-stack-values.yaml
	$(HELM) install prometheus-rbac integration/helm/prometheus-rbac -n default
	$(HELM) upgrade -i ${OPERATOR_NAME}-local charts/${OPERATOR_NAME} -n ${OPERATOR_NAME}-local --create-namespace \
	  --set enableCertManager=true \
	  --set image.repository=${HELM_TEST_IMG_NAME} \
	  --set image.tag=${HELM_TEST_IMG_TAG}
	$(KUBECTL) wait --namespace ${OPERATOR_NAME}-local --for=condition=ready pod --selector=app.kubernetes.io/name=${OPERATOR_NAME} --timeout=90s
	$(KUBECTL) wait --namespace default --for=condition=ready pod prometheus-kube-prometheus-stack-prometheus-0 --timeout=180s
	$(KUBECTL) exec prometheus-kube-prometheus-stack-prometheus-0 -n default -c test-metrics -- /bin/sh -c "echo 'Example metrics...' && cat /tmp/ready"

.PHONY: helmchart-clean
helmchart-clean:
	rm -rf ./charts

.PHONY: kind
KIND ?= $(LOCALBIN)/kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	test -s $(LOCALBIN)/kind || echo "Downloading kind to ${KIND}..." && GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@${KIND_VERSION}

.PHONY: kubectl
KUBECTL ?= $(LOCALBIN)/kubectl
kubectl: ## Download kubectl locally if necessary.
ifeq (,$(wildcard $(KUBECTL)))
	@{ \
	set -e ;\
	echo "Downloading kubectl to ${KUBECTL}..." ;\
	OS=$(shell go env GOOS) ;\
	ARCH=$(shell go env GOARCH) ;\
	curl --create-dirs -sSLo ${KUBECTL} https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/$${OS}/$${ARCH}/kubectl ;\
	chmod +x ${KUBECTL} ;\
	}
endif

.PHONY: helm
HELM ?= $(LOCALBIN)/helm
helm: ## Download helm locally if necessary.
ifeq (,$(wildcard $(HELM)))
	echo "Downloading helm to ${HELM}..."
	OS=$(shell go env GOOS) ;\
	ARCH=$(shell go env GOARCH) ;\
	curl --create-dirs -sSLo ${HELM}.tar.gz https://get.helm.sh/helm-${HELM_VERSION}-$${OS}-$${ARCH}.tar.gz ;\
	tar -xf ${HELM}.tar.gz -C $(LOCALBIN)/ ;\
	mv ./bin/$${OS}-$${ARCH}/helm ${HELM}
endif

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
	@{ \
	set -e ;\
	echo "Downloading operator-sdk to $(OPERATOR_SDK)..." ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
endif

.PHONY: clean
clean:
	rm -rf $(LOCALBIN) ./bundle ./bundle-* ./charts
