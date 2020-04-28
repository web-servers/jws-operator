DOCKER_REPO ?= docker.io/
IMAGE ?= ${USER}/jws-image-operator
TAG ?= v0.0.2
PROG  := jws-image-operator

.DEFAULT_GOAL := help

## setup            Ensure the operator-sdk is installed.
setup:
	./build/setup-operator-sdk.sh

## dep              Ensure deps are locally available.
dep:
	dep ensure

## codegen          Ensure code is generated.
codegen: setup
	operator-sdk generate k8s
	operator-sdk generate openapi

## build            Compile and build the JWS operator.
build: dep codegen
	operator-sdk build "${DOCKER_REPO}$(IMAGE):$(TAG)"

## push             Push Docker image to the docker.io repository.
push: build
	docker push "${DOCKER_REPO}$(IMAGE):$(TAG)"

## clean            Remove all generated build files.
clean:
	rm -rf build/_output

## run-openshift    Run the JWS operator on OpenShift.
run-openshift:
	./build/run-openshift.sh

help : Makefile
	@sed -n 's/^##//p' $<
