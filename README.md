# JWS Image Operator - Prototype
The purpose of this repository is to showcase a proof of concept of a simple Openshift Operator to manage JWS Images.

## What does it provide?
This prototype mimics the features provided by the [JWS Tomcat8 Basic Template](https://github.com/openshift/openshift-ansible/blob/release-3.11/roles/openshift_examples/files/examples/x86_64/xpaas-templates/jws31-tomcat8-basic-s2i.json). It allows the automated deployment of Tomcat instances.

## Development Workflow (we did it to create the files in the repository)
The prototype has been written in Golang. it uses [dep](https://golang.github.io/dep/) as dependency manager and the [operator-sdk](https://github.com/operator-framework/operator-sdk) as development Framework and project manager. This SDK allows the generation of source code to increase productivity. It is solely used to conveniently write and build an Openshift or Kubernetes operator (the end-user does not need the operator-sdk to deploy a pre-build version of the operator)·

The development workflow used in this prototype is standard to all Operator development:
1. Add a Custom Resource Definition
```bash
$ operator-sdk add api --api-version=jws.apache.org/v1alpha1 --kind=Tomcat
```
2. Define its attributes (by editing the generated file *tomcat_types.go*)
3. Update the generated code. This needs to be done every time CRDs are altered
```bash
$ operator-sdk generate k8s
```
4. Define the specifications of the CRD (by editing the generated file *deploy/crds/tomcat_v1alpha1_tomcat_crd.yaml*) and update the generated code
5. Add a Controller for that Custom Resource
```bash
$ operator-sdk add controller --api-version=jws.apache.org/v1alpha1 --kind=Tomcat
```
6. Write the Controller logic and adapt roles to give permissions to necessary resources

## Building the Operator
### Requirements
To build the operator, you will first need to install both of these tools:
* [Dep](https://golang.github.io/dep/)
* [Operator-sdk](https://github.com/operator-framework/operator-sdk)

### Procedure
Now that the tools are installed, follow these few steps to build it up:

1. check out the repo in $GOPATH/src/github.com
2. Start by building the project dependencies using `dep ensure` from the root directory of this project.
3. Then, simply run `operator-sdk build <imagetag>` to build the operator.

You will need to push it to a Docker Registry accessible by your Openshift Server in order to deploy it. I used docker.io:
```bash
$ cd $GOPATH/src/github.comsrc/github.com
$ git checkout https://github.com/web-servers/jws-image-operator.git
$ export IMAGE=docker.io/<username>/jws-image-operator:v0.0.1
$ dep ensure
$ operator-sdk build $IMAGE
$ docker login docker.io
$ docker push $IMAGE
```

## Deploy to an Openshift Cluster
The operator is pre-built and containerized in a docker image. By default, the deployment has been configured to utilize that image. Therefore, deploying the operator can be done by following these simple steps:
1. Define a namespace
```bash
$ export NAMESPACE="jws-operator"
```
2. Login to your Openshift Server using `oc login` and use it to create a new project
```bash
$ oc new-project $NAMESPACE
```
3. Install the JWS Tomcat Basic Image Stream in the *openshift* project. For testing purposes, this repository provides a version of the corresponding script (*xpaas-streams/jws53-tomcat9-image-stream.json*) using the __unsecured Red Hat Registy__ (registry.access.redhat.com). Please make sure to use the [latest version](https://github.com/openshift/openshift-ansible) with a secured registry for production use.
```bash
$ oc create -f xpaas-streams/jws53-tomcat9-image-stream.json -n openshift
```
As the image stream isn't namespace-specific, creating this resource in the _openshift_ project makes it convenient to reuse it across multiple namespaces. The following resources, more specific, will need to be created for every namespace.

4. Create the necessary resources
```bash
$ oc create -f deploy/crds/jws_v1alpha1_tomcat_crd.yaml -n $NAMESPACE
$ oc create -f deploy/service_account.yaml -n $NAMESPACE
$ oc create -f deploy/role.yaml -n $NAMESPACE
$ oc create -f deploy/role_binding.yaml -n $NAMESPACE
```
5. Deploy the operator using the template
```bash
$ oc process -f deploy/operator.yaml IMAGE=${IMAGE} | oc create -f -
```
6. Create a Tomcat instance (Custom Resource). An example has been provided in *deploy/crds/jws_v1alpha1_tomcat_cr.yaml*
```bash
$ oc apply -f deploy/crds/jws_v1alpha1_tomcat_cr.yaml
```
7. If the DNS is not setup in your Openshift installation, you will need to add the resulting route to your local `/etc/hosts` file in order to resolve the URL. It has point to the IP address of the node running the router. You can determine this address by running `oc get endpoints` with a cluster-admin user.

8. Finally, to access the newly deployed application, simply use the created route with */websocket-chat*
```bash
oc get routes
NAME      HOST/PORT                                            PATH      SERVICES   PORT      TERMINATION   WILDCARD
jws-app   jws-app-jws-operator.apps.jclere.rhmw-runtimes.net             jws-app    <all>                   None
```
Then go to http://jws-app-jws-operator.apps.jclere.rhmw-runtimes.net/websocket-chat using a browser.

## What to do next?
Below are some features that may be relevant to add in the near future.

__Handling Configuration Changes__

The current Reconciliation loop (Controller logic) is very simple. It creates the necessary resources if they don't exist. Handling configuration changes of our Custom Resource and its Pods must be done to achieve stability.

__Adding Support for Custom Configurations__

The JWS Image Templates provide custom configurations using databases such as MySQL, PostgreSQL, and MongoDB. We could add support for these configurations defining a custom resource for each of these platforms and managing them in the Reconciliation loop.

__Handling Image Updates__

This may be tricky depending on how we decide to handle Tomcat updates. We may need to implement data migration along with backups to ensure the reliability of the process.

__Adding Support for Kubernetes Clusters__

This Operator prototype is currently using some Openshift specific resources such as DeploymentConfigs, Routes, and ImageStreams. In order to run on Kubernetes Clusters, equivalent resources available on Kubernetes have to be implemented.
