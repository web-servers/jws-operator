# (JBoss Web Server) JWS Operator

This repository contains the source code of a simple Openshift Operator that manages JWS.

## What does it provide?

This prototype mimics the features provided by the [JWS Tomcat8 Basic Template](https://github.com/openshift/openshift-ansible/blob/release-3.11/roles/openshift_examples/files/examples/x86_64/xpaas-templates/jws31-tomcat8-basic-s2i.json). It allows the automated deployment of Tomcat instances.

## Development Workflow

The operator has been written in Golang. It uses the [operator-sdk](https://github.com/operator-framework/operator-sdk) as development Framework and project manager. This SDK allows the generation of source code to increase productivity. It is solely used to conveniently write and build an Openshift or Kubernetes operator (the end-user does not need the operator-sdk to deploy a pre-build version of the operator)·

The development workflow used in this prototype is standard to all Operator development, check the operator SDK doc for that.

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
$ export IMG=docker.io/${USER}/jws-operator
$ cd jws-operator
$ podman login docker.io
$ make manifests docker-build docker-push
```

Note the Makefile uses _go mod tidy_, _go mod vendor_ then _go build_ to build the executable and podman to build and push the image.

## Deploy from source to a kubernetes Cluster

See https://github.com/web-servers/jws-operator/blob/main/kubernetes.md

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
If you don't use the **-n openshift** or use another ImageStream name you will have to adjust the imageStreamNamespace: to \$NAMESPACE and imageStreamName: to the correct value in the Custom Resource file _config/samples/jws_v1alpha1_tomcat_cr.yaml_.

4. Create the necessary resources


```bash
make deploy
```

5. Create a Tomcat instance (Custom Resource). An example has been provided in _config/samples/web.servers.org_webservers_imagestream_cr.yaml_ .
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
$ oc apply -f config/samples/web.servers.org_webservers_imagestream_cr.yaml
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

5. Create a Tomcat instance (Custom Resource). An example has been provided in _config/samples/web.servers.org_webservers_cr.yaml_

```
apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example-image-webserver
spec:
  applicationName: jws-app
  replicas: 2
  webImage:
    applicationImage: quay.io/jfclere/tomcat10:latest
```

6. Then deploy your webapp.

```bash
$ oc apply -f config/samples/web.servers.org_webservers_cr.yaml
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
oc delete deployment.apps/jws-operator --namespace jws-operator-system
```

or better to clean everything:
```bash
oc delete webserver.web.servers.org/example-webserver
make undeploy
```

Note that the first _oc delete_ deletes what the operator creates for the example-webserver application, these second _oc delete_ deletes the operator and all resource it needs to run. The ImageStream can be deleted manually if needed.

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

**Adding Support for Custom Configurations**

The JWS Image Templates provide custom configurations using databases such as MySQL, PostgreSQL, and MongoDB. We could add support for these configurations defining a custom resource for each of these platforms and managing them in the Reconciliation loop.

**Handling Image Updates**

This may be tricky depending on how we decide to handle Tomcat updates. We may need to implement data migration along with backups to ensure the reliability of the process. The operator can support updates in 2 ways: Pushing a new image in the ImageStream (OpenShift only) or updating the CR yaml file

**Adding Full Support for Kubernetes Clusters**

The Operator supports some Openshift specific resources such as DeploymentConfigs, Routes, and ImageStreams. Those are not available on Kubernetes cluster. Building from source in Kubernetes requires an additional image builder image, like the BuildConfig the builder needs to use a Docker repository to push what it is building. See https://github.com/web-servers/image-builder-jws for the builder.
