IMAGE ?= docker.io/${USER}/jws-image-operator:latest
PROG  := jws-image-operator

.DEFAULT_GOAL := help

## setup            Ensure the operator-sdk is installed.
setup:
	./build/setup-operator-sdk.sh

## tidy             Ensure modules are tidy.
tidy:
	export GOPROXY=proxy.golang.org
	go mod tidy
vendor: tidy
	go mod vendor

## codegen          Ensure code is generated.
codegen: setup
	operator-sdk generate k8s
	operator-sdk generate openapi

## build            Compile and build the JWS operator.
build/_output/bin/jws-image-operator: vendor
	go generate -mod=vendor ./...
build: build/_output/bin/jws-image-operator
	mkdir -p build/_output/bin/
	CGO_ENABLED=0 go build -mod=vendor -a -o build/_output/bin/jws-image-operator jws-image-operator/cmd/manager
image: build
	docker build -t "$(IMAGE)" . -f build/Dockerfile

## push             Push Docker image to the docker.io repository.
push: image
	docker push "$(IMAGE)"

## clean            Remove all generated build files.
clean:
	rm -rf build/_output
	rm deploy/kubernetes_operator.yaml

deploy/kubernetes_operator.yaml: deploy/operator.template
	sed 's|@OP_IMAGE_TAG@|$(IMAGE)|' deploy/operator.template > deploy/kubernetes_operator.yaml


## run-openshift    Run the JWS operator on OpenShift.
run-openshift:
	./build/run-openshift.sh

## run-kubernetes    Run the Tomcat operator on kubernetes.
run-kubernetes: deploy/kubernetes_operator.yaml
	./build/run-kubernetes.sh

help : Makefile
	@sed -n 's/^##//p' $<
