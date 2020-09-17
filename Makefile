IMAGE ?= docker.io/${USER}/jws-operator:latest
PROG  := jws-operator

.DEFAULT_GOAL := help

## setup                                    Ensure the operator-sdk is installed.
setup:
	./build/setup-operator-sdk.sh

## tidy                                     Ensures modules are tidy.
tidy:
	export GOPROXY=proxy.golang.org
	go mod tidy

## vendor                                   Ensures vendor directory is up to date
vendor: go.mod go.sum
	go mod vendor
	go generate -mod=vendor ./...

## codegen                                  Ensures code is generated.
codegen: setup
	operator-sdk generate k8s
	operator-sdk generate openapi

## build/_output/bin/                       Creates the directory where the executable is outputted.
build/_output/bin/:
	mkdir -p build/_output/bin/

## build/_output/bin/jws-operator     Compiles the operator
build/_output/bin/jws-operator: $(shell find pkg) $(shell find cmd) vendor | build/_output/bin/
	CGO_ENABLED=0 go build -mod=vendor -a -o build/_output/bin/jws-operator github.com/web-servers/jws-operator/cmd/manager

.PHONY: build

## build                                    Builds the operator
build: tidy build/_output/bin/jws-operator

## image                                    Builds the operator's image
image: build
	docker build -t "$(IMAGE)" . -f build/Dockerfile

## push                                     Push Docker image to the docker.io repository.
push: image
	docker push "$(IMAGE)"

## clean                                    Remove all generated build files.
clean:
	rm -rf build/_output
	rm -f deploy/kubernetes_operator.yaml

## generate-kubernetes_operator.yaml        Generates the deployment file for Kubernetes
generate-kubernetes_operator.yaml:
	sed 's|@OP_IMAGE_TAG@|$(IMAGE)|' deploy/kubernetes_operator.template > deploy/kubernetes_operator.yaml

## run-openshift                            Run the JWS operator on OpenShift.
run-openshift:
	oc create -f deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_crd.yaml
	oc create -f deploy/service_account.yaml
	oc create -f deploy/role.yaml
	oc create -f deploy/role_binding.yaml
	oc process -f deploy/openshift_operator.template IMAGE=${IMAGE} | oc create -f -


## run-kubernetes                           Run the Tomcat operator on kubernetes.
run-kubernetes: generate-kubernetes_operator.yaml
	kubectl create -f deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_crd.yaml
	kubectl create -f deploy/service_account.yaml
	kubectl create -f deploy/role.yaml
	kubectl create -f deploy/role_binding.yaml
	kubectl apply -f deploy/kubernetes_operator.yaml


help : Makefile
	@sed -n 's/^##//p' $<
