# JWS Image Operator
This repository contains the source code a simple Openshift Operator to manage JWS Images.

## What does it provide?
This prototype mimics the features provided by the [JWS Tomcat8 Basic Template](https://github.com/openshift/openshift-ansible/blob/release-3.11/roles/openshift_examples/files/examples/x86_64/xpaas-templates/jws31-tomcat8-basic-s2i.json). It allows the automated deployment of Tomcat instances.

## Development Workflow (we did it to create the files in the repository)
The prototype has been written in Golang. it uses [dep](https://golang.github.io/dep/) as dependency manager and the [operator-sdk](https://github.com/operator-framework/operator-sdk) as development Framework and project manager. This SDK allows the generation of source code to increase productivity. It is solely used to conveniently write and build an Openshift or Kubernetes operator (the end-user does not need the operator-sdk to deploy a pre-build version of the operator)·

The development workflow used in this prototype is standard to all Operator development:
1. Build the operator-sdk version we need and add a Custom Resource Definition
```bash
$ make setup
$ operator-sdk add api --api-version=web.servers.org/v1alpha1 --kind=JBossWebServer
```
2. Define its attributes (by editing the generated file *jbosswebserver_types.go*)
3. Update the generated code. This needs to be done every time CRDs are altered
```bash
$ operator-sdk generate k8s
```
4. Define the specifications of the CRD (by editing the generated file *deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_crd.yaml*) and update the generated code
5. Add a Controller for that Custom Resource
```bash
$ operator-sdk add controller --api-version=web.servers.org --help/v1alpha1 --kind=JBossWebServer
```
6. Write the Controller logic and adapt roles to give permissions to necessary resources

## Building the Operator
### Requirements
To build the operator, you will first need to install both of these tools:
* [Dep](https://golang.github.io/dep/)
* [Operator-sdk](https://github.com/operator-framework/operator-sdk)

### Procedure
Now that the tools are installed, follow these few steps to build it up:

1. clone the repo in $GOPATH/src/github.com
2. Start by building the project dependencies using `dep ensure` from the root directory of this project.
3. Then, simply run `operator-sdk build <imagetag>` to build the operator.

You will need to push it to a Docker Registry accessible by your Openshift Server in order to deploy it. I used docker.io:
```bash
$ cd $GOPATH/src/github.com
$ git clone https://github.com/web-servers/jws-image-operator.git
$ export IMAGE=docker.io/${USER}/jws-image-operator:v0.0.1
$ cd jws-image-operator
$ docker login docker.io
$ make push
```
Note the Makefile uses *go mod tidy*, *go mod vendor* then *go build* to build the executable and docker to build and push the image.

## Testing Using an operator prepared by Red Hat (Testing brewed images, internal)
Download the tar.gz file and import it in docker and then push it to your docker repo something like:
```bash
$ wget http://download.eng.bos.redhat.com/brewroot/packages/jboss-webserver-5-webserver54-openjdk8-tomcat9-rhel8-operator-container/1.0/2/images/docker-image-sha256:a0eba0294e43b6316860bafe9250b377e6afb4ab1dae79681713fa357556f801.x86_64.tar.gz
$ docker load -i docker-image-sha256:3c424d48db2ed757c320716dc5c4c487dba8d11ea7a04df0e63d586c4a0cf760.x86_64.tar.gz
Loaded image: pprokopi/jboss-webserver-openjdk8-operator:jws-5.4-rhel-8-containers-candidate-96397-20200820162758-x86_64
```
The ${TAG} is the internal build tag.

The load command returns the tag of the image from the build something like: ${TAG}, use it to rename image and push it:
```bash
$ export IMAGE=docker.io/${USER}/jws-image-operator:v0.0.1
$ docker tag ${TAG} ${IMAGE}
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
If you don't use the __-n openshift__ or use another ImageStream name you will have to adjust the imageStreamNamespace: to $NAMESPACE and imageStreamName: to the correct value in the Custom Resource file *deploy/crds/jws_v1alpha1_tomcat_cr.yaml*.
 
4. Create the necessary resources
```bash
$ oc create -f deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_crd.yaml -n $NAMESPACE
$ oc create -f deploy/service_account.yaml -n $NAMESPACE
$ oc create -f deploy/role.yaml -n $NAMESPACE
$ oc create -f deploy/role_binding.yaml -n $NAMESPACE
```
5. Deploy the operator using the template (IMAGE is something like docker.io/${USER}/jws-image-operator:v0.0.1)
```bash
$ oc process -f deploy/operator.yaml IMAGE=${IMAGE} | oc create -f -
```
6. Create a Tomcat instance (Custom Resource). An example has been provided in *deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_cr.yaml*
make sure you adjust sourceRepositoryUrl, sourceRepositoryRef (branch) and contextDir (subdirectory) to you webapp sources, branch and context.
like:
```
  sourceRepositoryUrl: https://github.com/jfclere/demo-webapp.git
  sourceRepositoryRef: "master"
  contextDir: /
  imageStreamNamespace: openshift
  imageStreamName: jboss-webserver54-openjdk8-tomcat9-ubi8-openshift:latest
```
Then deploy your webapp.
```bash
$ oc apply -f deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_cr.yaml
```
7. If the DNS is not setup in your Openshift installation, you will need to add the resulting route to your local `/etc/hosts` file in order to resolve the URL. It has point to the IP address of the node running the router. You can determine this address by running `oc get endpoints` with a cluster-admin user.

8. Finally, to access the newly deployed application, simply use the created route with */demo-1.0/demo*
```bash
oc get routes
NAME      HOST/PORT                                            PATH      SERVICES   PORT      TERMINATION   WILDCARD
jws-app   jws-app-jws-operator.apps.jclere.rhmw-runtimes.net             jws-app    <all>                   None
```
Then go to http://jws-app-jws-operator.apps.jclere.rhmw-runtimes.net/demo-1.0/demo using a browser.

9. To remove everything
```bash
oc delete jbosswebserver.web.servers.org/example-jbosswebserver
oc delete deployment.apps/jws-image-operator
```
Note that the first *oc delete* deletes what the operator creates for the example-jbosswebserver application, these second *oc delete* deletes the operator and all resource it needs to run. The ImageStream can be deleted manually if needed.

10. What is supported?

10.1 changing the number of running replicas for the application: in your Custom Resource change *replicas: 2* to the value you want.

## What to do next?
Below are some features that may be relevant to add in the near future.

__Handling Configuration Changes__

The current Reconciliation loop (Controller logic) is very simple. It creates the necessary resources if they don't exist. Handling configuration changes of our Custom Resource and its Pods must be done to achieve stability.

__Adding Support for Custom Configurations__

The JWS Image Templates provide custom configurations using databases such as MySQL, PostgreSQL, and MongoDB. We could add support for these configurations defining a custom resource for each of these platforms and managing them in the Reconciliation loop.

__Handling Image Updates__

This may be tricky depending on how we decide to handle Tomcat updates. We may need to implement data migration along with backups to ensure the reliability of the process.

__Adding Support for Kubernetes Clusters__

This Operator prototype is currently using some Openshift specific resources such as DeploymentConfigs, Routes, and ImageStreams. In order to run on Kubernetes Clusters, equivalent resources available on Kubernetes have to be implemented or skipped.
