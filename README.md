# JWS Image Operator - Prototype
The purpose of this repository is to showcase a proof of concept of a simple Openshift Operator to manage JWS Images.

## What does it provide?
This prototype mimics the features provided by the JWS Tomcat8 Basic Template. It allows the automated deployment of Tomcat instances.

## Deploy to an Openshift Cluster
> Note that access credentials to *registry.redhat.io* needs to be configured in your Openshift Server to allow the pull of the JWS image streams. If not already configured please refer to [the documentation](https://docs.openshift.com/container-platform/3.11/install_config/configuring_red_hat_registry.html).

The operator is pre-built and containerized in a docker image. By default, the deployment has been configured to utilize that image. Therefore, deploying the operator can be done by following these simple steps:
1. Define a namespace
```bash
$ export NAMESPACE="jws-operator"
```
2. Login to your Openshift Server using `oc login` and use it to create a new project
```bash
$ oc new-project $NAMESPACE
```
3. Install the JWS Tomcat8 Basic Image Stream in the *openshift* project. For testing purposes, this repository provides a version of the corresponding script (*xpaas-streams/jws31-tomcat8-image-stream.json*). Please make sure to use the [latest version](https://github.com/openshift/openshift-ansible) for production use.
```bash
$ oc create -f xpaas-streams/jws31-tomcat8-image-stream.json -n openshift
```
4. Create the necessary resources
```bash
$ oc create -f deploy/crds/jws_v1alpha1_tomcat_crd.yaml -n $NAMESPACE
$ oc create -f deploy/service_account.yaml -n $NAMESPACE
$ oc create -f deploy/role.yaml -n $NAMESPACE
$ oc create -f deploy/role_binding.yaml -n $NAMESPACE
```
5. Deploy the operator
```bash
$ oc create -f deploy/operator.yaml
```
6. Create a Tomcat instance (Custom Resource). As an example, the repository provides a Custom Resource for Tomcat in *deploy/crds/jws_v1alpha1_tomcat_cr.yaml*
```bash
$ oc apply -f deploy/crds/jws_v1alpha1_tomcat_cr.yaml
```

## Development Workflow
The prototype has been written in Golang. it uses [dep](https://golang.github.io/dep/) as dependency manager and the [operator-sdk](https://github.com/operator-framework/operator-sdk) as development Framework and project manager. This SDK allows the generation of source code to increase productivity. The development workflow used in this prototype is standard to all Operator development:
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
To build the Operator, simply run `operator-sdk build <imagetag>`. You will then need to push it to a Docker Registry accessible by your Openshift Server in order to deploy it. I used docker.io:
```bash
$ export IMAGE=docker.io/<username>/jws-image-operator:v0.0.1
$ operator-sdk build $IMAGE
$ docker push $IMAGE
```
Finally, edit *deploy/operator.yaml* and change the imagetag to your image.

## What to do next?
Below are some features that may be relevant to add in the near future.

__Handling Configuration Changes__

The current Reconciliation loop (Controller logic) is very simple. It creates the necessary resources if they don't exist. Handling configuration changes of our Custom Resource and its Pods must be done to achieve stability.

__Adding Support for Custom Configurations__

The JWS Image Templates provide custom configurations using databases such as MySQL, PostgreSQL, and MongoDB. We could add support for these configurations defining a custom resource for each of these platforms and managing them in the Reconciliation loop.

__Handling Image Updates__

This may be tricky depending on how we decide to handle Tomcat updates. We may need to implement data migration and backups to ensure reliability of the process.

__Adding Support for Kubernetes Clusters__

This Operator prototype is currently using some Openshift specific resources such as DeploymentConfigs, Routes, and ImageStreams. In order to run on Kubernetes Clusters, equivalent resources available on Kubernetes have to be implemented.

## Difficulty Evaluation
I think the real difficulty of Operator development is that it needs a deep understanding of how both Kubernetes and Openshift works along with being aware of their respective stock resources/controllers and how they interact with each other. Otherwise, working with the operator-sdk is both simple and efficient. It takes a minute to get used to Golang but it probably won't scare off anyone familiar with structures and pointers.