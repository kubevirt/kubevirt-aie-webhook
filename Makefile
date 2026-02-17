IMAGE_REGISTRY ?= quay.io/kubevirt
IMAGE_NAME ?= kubevirt-aie-webhook
IMAGE_TAG ?= latest
IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

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

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push:
	docker push $(IMG)

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
	KUBECONFIG=$$(scripts/kubevirtci.sh kubeconfig) go test ./tests/... -v -count=1 -timeout 20m

.PHONY: clean
clean:
	rm -rf kubevirt-aie-webhook _kubevirt
