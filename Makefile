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

.PHONY: clean
clean:
	rm -f kubevirt-aie-webhook
