mkdir jws-operator
cd jws-operator
operator-sdk init --domain web.servers.org --repo github.com/web-servers/jws-operator
operator-sdk create api --version v1alpha1 --kind WebServer --resource --controller

Add into PROJECT
"group: webservers"

(Optional) Update comment into greoupversion_info.go
Package v1alpha1 contains API Schema definitions for the webservers v1alpha1 API group

Update config/samples/kustomization.yaml according github repo
Rename samples config/samples/v1alpha1_webserver.yaml -> config/samples/webservers_v1alpha1_webserver.yaml

Updated golang image in Docker file to 1.23 version

Login to quay.io
podman login quay.io

Set IMG env variable:
export IMG=quay.io/${USER}/jws-operator:new-sdk

### Checkpoint (Optional)
Try to build image:
make manifests docker-build docker-push

Try to create bundle
make bundle
make bundle-build bundle-push BUNDLE_IMG=quay.io/mmadzin/jws-operator-bundle:new-sdk
###

Original versions of following files were copied
api/v1alpha1/webserver_types.go -> api/v1alpha1/webserver_types.go
controllers/helper.go -> internal/controller/helper.go
controllers/service.go -> internal/controller/service.go
controllers/servicemonitor.go -> internal/controller/servicemonitor.go
controllers/templates.go -> internal/controller/templates.go
controllers/webserver_controller.go -> internal/controller/webserver_controller.go

update package in helper.go, service.go, servicemonitor.go, templates.go, webserver_controller.go (from controllers -> controller)

TODO
update templates.go connected to k8s.io/apimachinery/pkg/api/resource

### Checkpoint (Optional)
Try to build image:
rm -rf bundle
go mod tidy
make manifests docker-build docker-push

Try to create bundle
make bundle
make bundle-build bundle-push BUNDLE_IMG=quay.io/mmadzin/jws-operator-bundle:new-sdk
###

