
# Image URL to use all building/pushing image targets
REGISTRY ?= quay.io
REPOSITORY ?= $(REGISTRY)/redhat-cop/namespace-configuration-operator

IMG := $(REPOSITORY):latest

VERSION := v0.0.1

BUILD_COMMIT := $(shell ./scripts/build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./scripts/build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./scripts/build/get-build-hostname.sh)

LDFLAGS := "-X github.com/redhat-cop/namespace-configuration-operator/version.Version=$(VERSION) \
	-X github.com/redhat-cop/namespace-configuration-operator/version.Vcs=$(BUILD_COMMIT) \
	-X github.com/redhat-cop/namespace-configuration-operator/version.Timestamp=$(BUILD_TIMESTAMP) \
	-X github.com/redhat-cop/namespace-configuration-operator/version.Hostname=$(BUILD_HOSTNAME)"

all: manager

# Run tests
native-test: generate fmt vet
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o build/_output/bin/namespace-configuration-operator  -ldflags $(LDFLAGS) github.com/redhat-cop/namespace-configuration-operator/cmd/manager

# Build manager binary
manager-osx: generate fmt vet
	GOOS=darwin go build -o build/_output/bin/namespace-configuration-operator -ldflags $(LDFLAGS) github.com/redhat-cop/namespace-configuration-operator/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install:
	cat deploy/crds/*crd.yaml | kubectl apply -f-

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Docker Login
docker-login:
	@docker login -u $(DOCKER_USER) -p $(DOCKER_PASSWORD) $(REGISTRY)

# Tag for Dev
docker-tag-dev:
	@docker tag $(IMG) $(REPOSITORY):dev

# Tag for Dev
docker-tag-release:
	@docker tag $(IMG) $(REPOSITORY):$(VERSION)
	@docker tag $(IMG) $(REPOSITORY):latest	

# Push for Dev
docker-push-dev:  docker-tag-dev
	@docker push $(REPOSITORY):dev

# Push for Release
docker-push-release:  docker-tag-release
	@docker push $(REPOSITORY):$(VERSION)
	@docker push $(REPOSITORY):latest

# Build the docker image
docker-build:
	docker build . -t ${IMG} -f build/Dockerfile

# Push the docker image
docker-push:
	docker push ${IMG}

# Travis Latest Tag Deployment
travis-latest-deploy: docker-login docker-build docker-push

# Travis Dev Deployment
travis-dev-deploy: docker-login docker-build docker-push-dev

# Travis Release
travis-release-deploy: docker-login docker-build docker-push-release