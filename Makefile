GOPATH   ?= $(shell go env GOPATH)
GOBIN    ?= $(GOPATH)/bin
IMG_REPO ?= sandlock
IMG_TAG  ?= latest

OPERATOR_IMG    := $(IMG_REPO)/operator:$(IMG_TAG)
SUPERVISOR_IMG  := $(IMG_REPO)/supervisor:$(IMG_TAG)
CONTROLPLANE_IMG := $(IMG_REPO)/controlplane:$(IMG_TAG)

GO := go
DOCKER := docker

.PHONY: all build test generate fmt vet \
        web-install web-build web-dev \
        docker-build kind-up kind-down kind-load deploy-crds deploy undeploy

all: build

## ── Build ─────────────────────────────────────────────────────────────────────

build:
	$(GO) build -o bin/operator       ./cmd/operator
	$(GO) build -o bin/supervisor     ./cmd/supervisor
	$(GO) build -o bin/controlplane   ./cmd/controlplane
	$(GO) build -o bin/sandlock       ./cmd/sandlock

test:
	$(GO) test ./... -v -count=1

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

## ── Codegen (requires controller-gen) ────────────────────────────────────────

CONTROLLER_GEN := $(GOBIN)/controller-gen

.PHONY: controller-gen
controller-gen:
	$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases

## ── Container images ─────────────────────────────────────────────────────────

docker-build:
	$(DOCKER) build -t $(SUPERVISOR_IMG) \
	  -f images/claude-code/Dockerfile .
	$(DOCKER) build -t $(OPERATOR_IMG) \
	  --build-arg BINARY=operator \
	  -f build/Dockerfile.operator .
	$(DOCKER) build -t $(CONTROLPLANE_IMG) \
	  --build-arg BINARY=controlplane \
	  -f build/Dockerfile.operator .

## ── Helm ─────────────────────────────────────────────────────────────────────

HELM_RELEASE ?= sandlock
HELM_NAMESPACE ?= sandlock-system

helm-deps:
	helm dependency update deploy/chart

helm-lint: helm-deps
	helm lint deploy/chart

helm-template: helm-deps
	helm template $(HELM_RELEASE) deploy/chart

helm-install: helm-deps
	helm upgrade --install $(HELM_RELEASE) deploy/chart \
	  --namespace $(HELM_NAMESPACE) --create-namespace \
	  $(HELM_ARGS)

helm-uninstall:
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

## ── Web dashboard ────────────────────────────────────────────────────────────

web-install:
	cd web && npm install

web-build: web-install
	cd web && npm run build

web-dev:
	cd web && npm run dev

## ── kind cluster ─────────────────────────────────────────────────────────────

kind-up:
	kind create cluster --config deploy/kind-config.yaml || true
	kubectl config use-context kind-sandlock

kind-down:
	kind delete cluster --name sandlock

kind-load:
	kind load docker-image $(OPERATOR_IMG) --name sandlock
	kind load docker-image $(SUPERVISOR_IMG) --name sandlock
	kind load docker-image $(CONTROLPLANE_IMG) --name sandlock

## ── Deploy ───────────────────────────────────────────────────────────────────

deploy-crds:
	kubectl apply -f config/crd/bases/

deploy: deploy-crds
	kubectl apply -f deploy/namespaces.yaml
	kubectl apply -f deploy/rbac/
	kubectl apply -f deploy/operator/

undeploy:
	kubectl delete -f deploy/operator/ --ignore-not-found
	kubectl delete -f deploy/rbac/ --ignore-not-found
	kubectl delete -f deploy/namespaces.yaml --ignore-not-found
	kubectl delete -f config/crd/bases/ --ignore-not-found
