IMAGE ?= docker.io/${USER}/jws-operator:latest
PROG  := jws-operator
NAMESPACE :=`oc project -q`
VERSION ?= 1.1.0
.DEFAULT_GOAL := help
DATETIME := `date -u +'%FT%TZ'`
CONTAINER_IMAGE ?= "${IMAGE}"

## setup                                    Ensure the operator-sdk is installed.
setup:
	./build/setup-operator-sdk.sh

setup-e2e-test:
	./build/setup-operator-sdk-e2e-tests.sh

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
	podman build -t "$(IMAGE)" . -f build/Dockerfile
	$(MAKE) generate-operator.yaml

## push                                     Push Docker image to the docker.io repository.
push: image
	podman push "$(IMAGE)"

## clean                                    Remove all generated build files.
clean:
	rm -rf build/_output/

## generate-kubernetes_operator.yaml        Generates the deployment file for Kubernetes
generate-operator.yaml:
	sed 's|@OP_IMAGE_TAG@|$(IMAGE)|' deploy/operator.template > deploy/operator.yaml

## run-openshift                            Run the JWS operator on OpenShift.
run-openshift: push
	oc create -f deploy/crds/web.servers.org_webservers_crd.yaml
	oc create -f deploy/service_account.yaml
	oc create -f deploy/role.yaml
	oc create -f deploy/role_binding.yaml
	oc apply -f deploy/operator.yaml

## run-kubernetes                           Run the Tomcat operator on kubernetes.
run-kubernetes: push
	kubectl create -f deploy/crds/web.servers.org_webservers_crd.yaml
	kubectl create -f deploy/service_account.yaml
	kubectl create -f deploy/role.yaml
	kubectl create -f deploy/role_binding.yaml
	kubectl apply -f deploy/operator.yaml

clean-cluster:
	kubectl delete -f deploy/crds/web.servers.org_webservers_crd.yaml || true
	kubectl delete -f deploy/service_account.yaml || true
	kubectl delete -f deploy/role.yaml || true
	kubectl delete -f deploy/role_binding.yaml || true

test: test-local

test-local: test-e2e-5

test-remote: push test-e2e-5

test-e2e-5: clean-cluster build setup-e2e-test
	oc delete namespace "jws-e2e-tests" || true
	oc new-project "jws-e2e-tests" || true
	oc create -f xpaas-streams/jws54-tomcat9-image-stream.json -n jws-e2e-tests || true
	LOCAL_OPERATOR=true OPERATOR_NAME=jws-operator-1 ./operator-sdk-e2e-tests test local ./test/e2e/5 --verbose --debug --operator-namespace jws-e2e-tests --local-operator-flags "--zap-devel --zap-level=5" --global-manifest ./deploy/crds/web.servers.org_webservers_crd.yaml --go-test-flags "-timeout=30m"

generate-csv:
	operator-sdk generate crds
	operator-sdk generate csv --verbose --csv-version $(VERSION) --update-crds
	mkdir manifests/jws/$(VERSION)/ || true
	mv deploy/olm-catalog/jws-operator/manifests/* manifests/jws/$(VERSION)/
	rm -r deploy/olm-catalog

customize-csv: generate-csv
	DATETIME=$(DATETIME) CONTAINER_IMAGE=$(CONTAINER_IMAGE) OPERATOR_VERSION=$(VERSION) build/customize_csv.sh

catalog:
	podman build -f build/catalog.Dockerfile -t my-test-catalog:latest .
	podman tag my-test-catalog:latest quay.io/${USER}/my-test-catalog:latest
	podman push quay.io/${USER}/my-test-catalog:latest
	sed s:@USER@:${USER}: catalog.yaml.template > catalog.yaml
	sed s:@NAMESPACE@:${NAMESPACE}: operatorgroup.yaml.template > operatorgroup.yaml
	sed s:@NAMESPACE@:${NAMESPACE}: subscription.yaml.template > subscription.yaml
	@echo ""
	@echo "Use oc create -f catalog.yaml to install the CatalogSource for the operator"
	@echo ""
	@echo "Use oc create -f operatorgroup.yaml and oc create -f subscription.yaml to install the operator in ${NAMESPACE}"
	@echo "or use the openshift web interface on the installed operator"

help : Makefile
	@sed -n 's/^##//p' $<
