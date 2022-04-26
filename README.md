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

0. make sure you have kube-apiserver, etcd and kubectl installed, they are needed for docker-build to make local tests.
1. Clone the repo in $GOPATH/src/github.com/web-servers
2. Set a name for your image. Default value is docker.io/${USER}/jws-operator:latest
3. The first time you build you have to download controller-gen in bin
```bash
$ make controller-gen
```
4. Sync the vendor directory
```bash
$ go mod vendor
```
5. Then, simply run `make manifests docker-build docker-push` to build the operator and push it to your image registry.


You will need to push it to a Docker Registry accessible by your Openshift Server in order to deploy it. For example:

```bash
$ mkdir -p $GOPATH/src/github.com/web-servers
$ cd $GOPATH/src/github.com/web-servers
$ git clone https://github.com/web-servers/jws-operator.git
$ export IMG=quay.io/${USER}/jws-operator
$ cd jws-operator
$ podman login quay.io
$ make manifests docker-build docker-push
```

**Note** the Makefile uses _go mod tidy_, _go mod vendor_ then _go build_ to build the executable and podman to build and push the image.  
**Note** the build is done using a docker image: Check the Dockerfile, note the FROM golang:1.17 as builder so don't forget to adjust it with changing the go version in go.mod.  
**Note** To generate the `vendor` directory which is needed to build the operator internally in RH build system, check out the repository and run `go mod vendor` (add -v for verbose output) and wait for the directory to get updated.  
**Note** The TEST_ASSET_KUBE_APISERVER, TEST_ASSET_ETCD and TEST_ASSET_KUBECTL can be used to define kube-apiserver, etcd and kubectl if they are not in $PATH (see https://book.kubebuilder.io/reference/envtest.html for more).

## Install the operator using OLM

Make sure you have OLM installed, otherwise install it. See https://olm.operatorframework.io/docs/getting-started/
To build the bundle and deploy the operator do something like the following:
```bash
make bundle
podman login quay.io
make bundle-build bundle-push BUNDLE_IMG=quay.io/${USER}/jws-operator-bundle:0.0.0
operator-sdk run bundle quay.io/${USER}/jws-operator-bundle:0.0.0
```
To remove
```bash
operator-sdk cleanup jws-operator
```
**Note** Check the installModes: in bundle/manifests/jws-operator.clusterserviceversion.yaml (all AllNamespaces is openshift-operators)

## Install the operator from sources.

The operator is pre-built and containerized in a docker image. By default, the deployment has been configured to utilize that image. Therefore, deploying the operator can be done by following these simple steps:

```bash
make deploy IMG=quay.io/${USER}/jws-operator
```

To check for the operator installation you can check the operator pods
```bash
kubectl get pods -n jws-operator-system
```
You should get something like:
```
NAME                                               READY   STATUS    RESTARTS   AGE
jws-operator-controller-manager-789dcf556f-2cl2q   2/2     Running   0          2m13s
```


## Deploy a WebServer with a webapp built from the sources

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
$ oc create -f xpaas-streams/jws56-tomcat9-image-stream.json -n openshift
```

As the image stream isn't namespace-specific, creating this resource in the _openshift_ project makes it convenient to reuse it across multiple namespaces. The following resources, which are more specific, will need to be created for every namespace.
If you don't use the **-n openshift** or use another ImageStream name you will have to adjust the imageStreamNamespace: to \$NAMESPACE and imageStreamName: to the correct value in the Custom Resource file _config/samples/jws_v1alpha1_tomcat_cr.yaml_.

4. Create a Tomcat instance (Custom Resource). An example has been provided in _config/samples/web.servers.org_webservers_imagestream_cr.yaml_ .
   Make sure to adjust sourceRepositoryUrl, sourceRepositoryRef (branch) and contextDir (subdirectory) to you webapp sources, branch and context.
   like:

```
  applicationName: jws-app
  replicas: 2
  webImageStream:
    imageStreamNamespace: openshift
    imageStreamName: webserver56-openjdk8-tomcat9-ubi8-image-stream
    webSources:
      sourceRepositoryUrl: https://github.com/jboss-openshift/openshift-quickstarts.git
      sourceRepositoryRef: "1.2"
      contextDir: tomcat-websocket-chat
```

5. Then deploy your webapp.

```bash
$ oc apply -f config/samples/web.servers.org_webservers_imagestream_cr.yaml
```

6. If the DNS is not setup in your Openshift installation, you will need to add the resulting route to your local `/etc/hosts` file in order to resolve the URL. It has point to the IP address of the node running the router. You can determine this address by running `oc get endpoints` with a cluster-admin user.

7. Finally, to access the newly deployed application, simply use the created route with _/demo-1.0/demo_

```bash
oc get routes
NAME      HOST/PORT                                            PATH      SERVICES   PORT      TERMINATION   WILDCARD
jws-app   jws-app-jws-operator.apps.jclere.rhmw-runtimes.net             jws-app    <all>                   None
```

Then go to http://jws-app-jws-operator.apps.jclere.rhmw-runtimes.net/demo-1.0/demo using a browser.

8. To remove everything

```bash
oc delete webserver.web.servers.org/example-webserver
oc delete deployment.apps/jws-operator
```

Note that the first _oc delete_ deletes what the operator creates for the example-webserver application, these second _oc delete_ deletes the operator and all resource it needs to run. The ImageStream can be deleted manually if needed.

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

4. Create a Tomcat instance (Custom Resource). An example has been provided in _config/samples/web.servers.org_webservers_cr.yaml_

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

5. Then deploy your webapp.

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

## Testing
To run a test with a real cluster you need a real cluster (kubernetes or openshift). A secret is needed to run a bunch of tests.
You can create the secret using something like:
```
kubectl create secret generic secretfortests --from-file=.dockerconfigjson=$HOME/.docker/config.json --type=kubernetes.io/dockerconfigjson
```
Some tests are pulling from the redhat portal make sure you have access to it (otherwise some tests will fail), some tests need to push to quay.io make sure you have access there.
The repositories you have to be able to pull from for the tests are:
```
registry.redhat.io/jboss-webserver-5/webserver56-openjdk8-tomcat9-openshift-rhel8
quay.io/jfclere/tomcat10-buildah
quay.io/jfclere/tomcat10
```
The quay.io ones are public

You also need to be able to push to:
```
quay.io/${USER}/test
```
When on openshift the jboss-webserver56-openjdk8-tomcat9-ubi8-image-stream ImageStream is used by the tests, to create it
```
oc create -f xpaas-streams/jws56-tomcat9-image-stream.json
oc secrets link default secretfortests --for=pull
```

To run the test to:
```
make realtest
```
The whole testsuite takes about 40 minutes...

**Note** When running the tests on OpenShift make sure to test in your own namespace and DON'T use default. Also make sure you have added "anyuid" to the ServiceAccount builder:
```bash
oc adm policy add-scc-to-user anyuid -z builder
```

## What to do next?

Below are some features that may be relevant to add in the near future.

**Adding Support for Custom Configurations**

The JWS Image Templates provide custom configurations using databases such as MySQL, PostgreSQL, and MongoDB. We could add support for these configurations defining a custom resource for each of these platforms and managing them in the Reconciliation loop.

**Handling Image Updates**

This may be tricky depending on how we decide to handle Tomcat updates. We may need to implement data migration along with backups to ensure the reliability of the process. The operator can support updates in 2 ways: Pushing a new image in the ImageStream (OpenShift only) or updating the CR yaml file

**Adding Full Support for Kubernetes Clusters**

The Operator supports some Openshift specific resources such as DeploymentConfigs, Routes, and ImageStreams. Those are not available on Kubernetes cluster. Building from source in Kubernetes requires an additional image builder image, like the BuildConfig the builder needs to use a Docker repository to push what it is building. See https://github.com/web-servers/image-builder-jws for the builder.
