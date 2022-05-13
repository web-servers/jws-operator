To update the operator SDK start from a new empty jws-operator directory and change to it.
```bash
mv jws-operator jws-operator.old
mkdir jws-operator
cd jws-operator
operator-sdk init --domain web.servers.org --repo github.com/web-servers/jws-operator
```

Fix the Makefile... Add in go-install-tool: (See https://github.com/kubernetes-sigs/kubebuilder/issues/2559)
```
echo "Installing $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install -v $(2) ;\
```
Create the new structure:
```bash
operator-sdk create api \
    --group=webservers \
    --version=v1alpha1 \
    --kind=WebServer \
    --resource \
    --controller
```
Copy the previous go files to the jws-operator
```bash
mv api/v1alpha1/webserver_types.go api/v1alpha1/webserver_controller.go.new
mv controllers/suite_test.go controllers/suite_test.go.new
mv controllers/webserver_controller.go controllers/webserver_controller.go.new
cp ../jws-operator.old/api/v1alpha1/webserver_types.go api/v1alpha1/webserver_types.go
cp ../jws-operator.old/controllers/suite_test.go controllers/suite_test.go
cp ../jws-operator.old/controllers/webserver_controller.go controllers/webserver_controller.go
```
Compare the files, adjust the go file and make sure you don't remove what the SDK added.
Test the structure.
```bash
make manifests
```
That creates the CRD, CRs etc... check they makes sense (compare with the old ones for example, read migration docs).
Copy back the go sources and any of the old file you need, then go to the main README.md and build the updated operator.
