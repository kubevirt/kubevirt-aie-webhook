CONTAINER_ENGINE ?= $(shell KUBEVIRT_CRI=$${KUBEVIRT_CRI} hack/container-engine.sh)
DOCKER_PREFIX ?= quay.io/kubevirt
IMAGE_NAME ?= kubevirt-aie-webhook
DOCKER_TAG ?= latest
IMG ?= $(DOCKER_PREFIX)/$(IMAGE_NAME):$(DOCKER_TAG)

.PHONY: build
build:
	go build -o kubevirt-aie-webhook .

.PHONY: test
test:
	go test ./... -v -count=1

.PHONY: lint
lint:
	go vet ./...
	golangci-lint run ./...

.PHONY: image-build
image-build:
	$(CONTAINER_ENGINE) build -t $(IMG) .

.PHONY: image-push
image-push:
	$(CONTAINER_ENGINE) push $(IMG)

.PHONY: helm-template
helm-template:
	helm template kubevirt-aie-webhook deploy/helm/kubevirt-aie-webhook

.PHONY: cluster-up
cluster-up:
	scripts/kubevirtci.sh up

.PHONY: cluster-down
cluster-down:
	scripts/kubevirtci.sh down

.PHONY: cluster-sync
cluster-sync:
	scripts/kubevirtci.sh sync

.PHONY: functest
functest:
	KUBECONFIG=$$(scripts/kubevirtci.sh kubeconfig) go test ./tests/... -v -count=1 -timeout 20m $(FUNC_TEST_ARGS)

.PHONY: clean
clean:
	rm -rf kubevirt-aie-webhook _kubevirt
