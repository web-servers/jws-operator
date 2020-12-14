# (JBoss Web Server) JWS Operator

This repository contains the source code of a simple Openshift Operator that manages JWS.

## What does it provide?

This prototype mimics the features provided by the [JWS Tomcat8 Basic Template](https://github.com/openshift/openshift-ansible/blob/release-3.11/roles/openshift_examples/files/examples/x86_64/xpaas-templates/jws31-tomcat8-basic-s2i.json). It allows the automated deployment of Tomcat instances.

## Development Workflow (how the operator was created)

The prototype has been written in Golang. It uses the [operator-sdk](https://github.com/operator-framework/operator-sdk) as development Framework and project manager. This SDK allows the generation of source code to increase productivity. It is solely used to conveniently write and build an Openshift or Kubernetes operator (the end-user does not need the operator-sdk to deploy a pre-build version of the operator)·

The development workflow used in this prototype is standard to all Operator development:

1. Install the operator-sdk version we need

```bash
$ make setup
```

2 . Add a Custom Resource Definition

```bash
$ operator-sdk add api --api-version=web.servers.org/v1alpha1 --kind=WebServer
```

3. Define its attributes (by editing the generated file _webserver_types.go_)
4. Update the generated code. This needs to be done every time CRDs are altered

```bash
$ operator-sdk generate k8s
```

5. Define the specifications of the CRD (by editing the generated file _deploy/crds/web.servers.org_webservers_crd.yaml_) and update the generated code (if needed)

6. Add a Controller for that Custom Resource

```bash
$ operator-sdk add controller --api-version=web.servers.org/v1alpha1 --kind=WebServer
```

7. Write the Controller logic and adapt roles to give permissions to necessary resources
8. Generate the CRD and CSV doing the following (adjust the version when needed):

```bash
$ operator-sdk generate crds
$ operator-sdk generate csv --csv-version 1.0.0
```

## Building the Operator

### Requirements

To build the operator, you will first need to install the following:

- [Golang] (https://golang.org/doc/install)
- [Docker] (https://docs.docker.com/engine/install/)

### Procedure

Now that the required tools are installed, follow these few steps to build it:

1. Clone the repo in $GOPATH/src/github.com/web-servers
2. Set a name for your image. Default value is docker.io/${USER}/jws-operator:latest
3. Then, simply run `make push` to build the operator and push it to your image registry.

You will need to push it to a Docker Registry accessible by your Openshift Server in order to deploy it. For example:

```bash
$ mkdir -p $GOPATH/src/github.com/web-servers
$ cd $GOPATH/src/github.com/web-servers
$ git clone https://github.com/web-servers/jws-operator.git
$ export IMAGE=docker.io/${USER}/jws-operator:v0.0.1
$ cd jws-operator
$ docker login docker.io
$ make push
```

Note the Makefile uses _go mod tidy_, _go mod vendor_ then _go build_ to build the executable and docker to build and push the image.

## Deploy from sources to an Openshift Cluster

The operator is pre-built and containerized in a docker image. By default, the deployment has been configured to utilize that image. Therefore, deploying the operator can be done by following these simple steps:

1. Define a namespace

```bash
$ export NAMESPACE="jws-operator"
```

2. Login to your Openshift Server using `oc login` and use it to create a new project

```bash
$ oc new-project $NAMESPACE
```

3. Install the JWS Tomcat Basic Image Stream in the _openshift_ project namespace. For testing purposes, this repository provides a version of the corresponding script (_xpaas-streams/jws54-tomcat9-image-stream.json_) using the **unsecured Red Hat Registy** (registry.access.redhat.com). Please make sure to use the [latest version](https://github.com/openshift/openshift-ansible) with a secured registry for production use.

```bash
$ oc create -f xpaas-streams/jws54-tomcat9-image-stream.json -n openshift
```

As the image stream isn't namespace-specific, creating this resource in the _openshift_ project makes it convenient to reuse it across multiple namespaces. The following resources, which are more specific, will need to be created for every namespace.
If you don't use the **-n openshift** or use another ImageStream name you will have to adjust the imageStreamNamespace: to \$NAMESPACE and imageStreamName: to the correct value in the Custom Resource file _deploy/crds/jws_v1alpha1_tomcat_cr.yaml_.

4. Create the necessary resources


```bash
make generate-operator.yaml
make run-openshift
```

5. Create a Tomcat instance (Custom Resource). An example has been provided in _deploy/crds/web.servers.org_webservers_imagestream_cr.yaml_ .
   Make sure to adjust sourceRepositoryUrl, sourceRepositoryRef (branch) and contextDir (subdirectory) to you webapp sources, branch and context.
   like:

```
  applicationName: jws-app
  replicas: 2
  webImageStream:
    imageStreamNamespace: openshift
    imageStreamName: jboss-webserver54-openjdk8-tomcat9-ubi8-openshift
    webSources:
      sourceRepositoryUrl: https://github.com/jboss-openshift/openshift-quickstarts.git
      sourceRepositoryRef: "1.2"
      contextDir: tomcat-websocket-chat
```

6. Then deploy your webapp.

```bash
$ oc apply -f deploy/crds/web.servers.org_webservers_imagestream_cr.yaml
```

7. If the DNS is not setup in your Openshift installation, you will need to add the resulting route to your local `/etc/hosts` file in order to resolve the URL. It has point to the IP address of the node running the router. You can determine this address by running `oc get endpoints` with a cluster-admin user.

8. Finally, to access the newly deployed application, simply use the created route with _/demo-1.0/demo_

```bash
oc get routes
NAME      HOST/PORT                                            PATH      SERVICES   PORT      TERMINATION   WILDCARD
jws-app   jws-app-jws-operator.apps.jclere.rhmw-runtimes.net             jws-app    <all>                   None
```

Then go to http://jws-app-jws-operator.apps.jclere.rhmw-runtimes.net/demo-1.0/demo using a browser.

9. To remove everything

```bash
oc delete webserver.web.servers.org/example-webserver
oc delete deployment.apps/jws-operator
```

Note that the first _oc delete_ deletes what the operator creates for the example-webserver application, these second _oc delete_ deletes the operator and all resource it needs to run. The ImageStream can be deleted manually if needed.

10. What is supported?

10.1 changing the number of running replicas for the application: in your Custom Resource change _replicas: 2_ to the value you want.

## Deploy for an existing JWS or Tomcat image

The operator is pre-built and containerized in a docker image. By default, the deployment has been configured to utilize that image. Therefore, deploying the operator can be done by following these simple steps:

1. Define a namespace

```bash
$ export NAMESPACE="jws-operator"
```

2. Login to your Openshift Server using `oc login` and use it to create a new project

```bash
$ oc new-project $NAMESPACE
```

3. Prepare your image and push it somewhere
   See https://github.com/jfclere/tomcat-openshift or https://github.com/apache/tomcat/tree/master/modules/stuffed to build the images.

4. Create the necessary resources


```bash
make generate-operator.yaml
make run-openshift
```

5. Create a Tomcat instance (Custom Resource). An example has been provided in _deploy/crds/web.servers.org_webservers_cr.yaml_

```
  applicationName: jws-app
  replicas: 2
  webImage:
    applicationImage: quay.io/jfclere/jws-image:5.4
```

6. Then deploy your webapp.

```bash
$ oc apply -f deploy/crds/web.servers.org_webservers_cr.yaml
```

6. On kubernetes you have to create a balancer to expose the service and later something depending on your cloud to expose the application

```bash
kubectl expose deployment jws-app --type=LoadBalancer --name=jws-balancer
kubectl kubectl get svc
NAME              TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
jws-balancer      LoadBalancer   10.100.57.140   <pending>     8080:32567/TCP   4m6s
```

The service jws-balancer then can be used to expose the application.

7. To remove everything

```bash
oc delete webserver.web.servers.org/example-webserver
oc delete deployment.apps/jws-operator
```

Note that the first _oc delete_ deletes what the operator creates for the example-webserver application, these second _oc delete_ deletes the operator and all resource it needs to run. The ImageStream can be deleted manually if needed.

10. What is supported?

10.1 changing the number of running replicas for the application: in your Custom Resource change _replicas: 2_ to the value you want.

## Configuring Readiness or Liveness probes:

serverReadinessScript and serverLivenessScript allow to use a custom liveness or readiness probe, we support 2 formats:

```
serverLivenessScript: cmd arg1 arg2 ...
serverLivenessScript: shell shellarg1 shellargv2 ... "cmd line for the shell"
```

Don't forget '\' if you need to escape something in the cmd line. Don't use ' ' in the arg that is the separator we support.

In case you don't use the HealthCheckValve you have to configure at least a serverReadinessScript.

For example if you are using the JWS 5.4 images you could use the following:

```
  webServerHealthCheck:
    serverReadinessScript: /bin/bash -c " /usr/bin/curl --noproxy '*' -s 'http://localhost:8080/health' | /usr/bin/grep -i 'status.*UP'"
```

If you are using a openjdk:8-jre-alpine based image and /test is your health URL:

```
  serverReadinessScript: /bin/busybox wget http://localhost:8080/test -O /dev/null
```

Note that HealthCheckValve requires tomcat 9.0.38+ or 10.0.0-M8 to work as expected and it was introducted in 9.0.15.

## What to do next?

Below are some features that may be relevant to add in the near future.

**Handling Configuration Changes**

The current Reconciliation loop (Controller logic) is very simple. It creates the necessary resources if they don't exist. Handling configuration changes of our Custom Resource and its Pods must be done to achieve stability.

**Adding Support for Custom Configurations**

The JWS Image Templates provide custom configurations using databases such as MySQL, PostgreSQL, and MongoDB. We could add support for these configurations defining a custom resource for each of these platforms and managing them in the Reconciliation loop.

**Handling Image Updates**

This may be tricky depending on how we decide to handle Tomcat updates. We may need to implement data migration along with backups to ensure the reliability of the process.

**Adding Full Support for Kubernetes Clusters**

This Operator prototype is currently using some Openshift specific resources such as DeploymentConfigs, Routes, and ImageStreams. In order to build from sources on Kubernetes Clusters, equivalent resources available on Kubernetes have to be implemented.
