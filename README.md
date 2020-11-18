# JWS Operator

This repository contains the source code a simple Openshift Operator to manage JWS.

## What does it provide?

This prototype mimics the features provided by the [JWS Tomcat8 Basic Template](https://github.com/openshift/openshift-ansible/blob/release-3.11/roles/openshift_examples/files/examples/x86_64/xpaas-templates/jws31-tomcat8-basic-s2i.json). It allows the automated deployment of Tomcat instances.

## Development Workflow (we did it to create the files in the repository)

The prototype has been written in Golang. it uses [dep](https://golang.github.io/dep/) as dependency manager and the [operator-sdk](https://github.com/operator-framework/operator-sdk) as development Framework and project manager. This SDK allows the generation of source code to increase productivity. It is solely used to conveniently write and build an Openshift or Kubernetes operator (the end-user does not need the operator-sdk to deploy a pre-build version of the operator)·

The development workflow used in this prototype is standard to all Operator development:

1. Build the operator-sdk version we need

```bash
$ make setup
```

2 . Add a Custom Resource Definition

```bash
$ operator-sdk add api --api-version=web.servers.org/v1alpha1 --kind=JbossWebServer
```

3. Define its attributes (by editing the generated file _jbosswebserver_types.go_)
4. Update the generated code. This needs to be done every time CRDs are altered

```bash
$ operator-sdk generate k8s
```

5. Define the specifications of the CRD (by editing the generated file _deploy/crds/web.servers.org_jbosswebservers_crd.yaml_) and update the generated code

6. Add a Controller for that Custom Resource

```bash
$ operator-sdk add controller --api-version=web.servers.org/v1alpha1 --kind=JbossWebServer
```

7. Write the Controller logic and adapt roles to give permissions to necessary resources
8. Generate the CR and CVS doing the following (adjust the version when needed):

```bash
$ operator-sdk generate crds
$ operator-sdk generate csv --csv-version 0.1.0
```

9. Generate the OLM and catalog (in deploy/olm-catalog).

```bash
$ olm-catalog gen-csv --csv-version 0.1.0
```

## Building the Operator

### Requirements

To build the operator, you will first need to install both of these tools:

- [Dep](https://golang.github.io/dep/)
- [Operator-sdk](https://github.com/operator-framework/operator-sdk)

### Procedure

Now that the tools are installed, follow these few steps to build it up:

1. clone the repo in \$GOPATH/src/github.com/web-servers
2. Start by building the project dependencies using `dep ensure` from the root directory of this project.
3. Then, simply run `operator-sdk build <imagetag>` to build the operator.

You will need to push it to a Docker Registry accessible by your Openshift Server in order to deploy it. I used docker.io:

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

## Testing Using an operator prepared by Red Hat (Testing brewed images, internal)

Download the tar.gz file and import it in docker and then push it to your docker repo something like:

```bash
$ wget http://download.eng.bos.redhat.com/brewroot/packages/jboss-webserver-5-webserver54-openjdk8-tomcat9-rhel8-operator-container/1.0/2/images/docker-image-sha256:a0eba0294e43b6316860bafe9250b377e6afb4ab1dae79681713fa357556f801.x86_64.tar.gz
$ docker load -i docker-image-sha256:3c424d48db2ed757c320716dc5c4c487dba8d11ea7a04df0e63d586c4a0cf760.x86_64.tar.gz
Loaded image: pprokopi/jboss-webserver-openjdk8-operator:jws-5.4-rhel-8-containers-candidate-96397-20200820162758-x86_64
```

The \${TAG} is the internal build tag.

The load command returns the tag of the image from the build something like: \${TAG}, use it to rename image and push it:

```bash
$ export IMAGE=docker.io/${USER}/jws-operator:v0.0.1
$ docker tag ${TAG} ${IMAGE}
$ docker login docker.io
$ docker push $IMAGE
```

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

3. Install the JWS Tomcat Basic Image Stream in the _openshift_ project. For testing purposes, this repository provides a version of the corresponding script (_xpaas-streams/jws53-tomcat9-image-stream.json_) using the **unsecured Red Hat Registy** (registry.access.redhat.com). Please make sure to use the [latest version](https://github.com/openshift/openshift-ansible) with a secured registry for production use.

```bash
$ oc create -f xpaas-streams/jws53-tomcat9-image-stream.json -n openshift
```

As the image stream isn't namespace-specific, creating this resource in the _openshift_ project makes it convenient to reuse it across multiple namespaces. The following resources, more specific, will need to be created for every namespace.
If you don't use the **-n openshift** or use another ImageStream name you will have to adjust the imageStreamNamespace: to \$NAMESPACE and imageStreamName: to the correct value in the Custom Resource file _deploy/crds/jws_v1alpha1_tomcat_cr.yaml_.

4. Create the necessary resources

```bash
$ oc create -f deploy/crds/web.servers.org_v1alpha1_jbosswebserver_crd.yaml -n $NAMESPACE
$ oc create -f deploy/service_account.yaml -n $NAMESPACE
$ oc create -f deploy/role.yaml -n $NAMESPACE
$ oc create -f deploy/role_binding.yaml -n $NAMESPACE
```

5. Deploy the operator using the deploy/operator.yaml file, make sure the image is what you build (IMAGE is something like docker.io/\${USER}/jws-operator:v0.0.1)

```bash
$ oc create -f deploy/operator.yaml
```

6. Create a Tomcat instance (Custom Resource). An example has been provided in _deploy/crds/web.servers.org_v1alpha1_jbosswebserver_cr.yaml_
   make sure you adjust sourceRepositoryUrl, sourceRepositoryRef (branch) and contextDir (subdirectory) to you webapp sources, branch and context.
   like:

```
  JbossWebSources:
    sourceRepositoryUrl: https://github.com/jfclere/demo-webapp.git
    sourceRepositoryRef: "master"
    contextDir: /
  JbossWebImageStream:
    imageStreamNamespace: openshift
    imageStreamName: jboss-webserver54-openjdk8-tomcat9-ubi8-openshift:latest
```

Then deploy your webapp.

```bash
$ oc apply -f deploy/crds/web.servers.org_v1alpha1_jbosswebserver_cr.yaml
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
oc delete jbosswebserver.web.servers.org/example-jbosswebserver
oc delete deployment.apps/jws-operator
```

Note that the first _oc delete_ deletes what the operator creates for the example-jbosswebserver application, these second _oc delete_ deletes the operator and all resource it needs to run. The ImageStream can be deleted manually if needed.

10. What is supported?

10.1 changing the number of running replicas for the application: in your Custom Resource change _replicas: 2_ to the value you want.

## Deploy for an existing JWS or Tomcat image

1. Install the operator as describe before

Note that kubernetes doesn't have templates and you have to adjust deploy/kubernetes_operator.template to have:

image: @OP_IMAGE_TAG@ set to right value and then use kubernetes apply -f deploy/kubernetes_operator.template to deploy the operator.

2. Prepare your image and push it somewhere
   See https://github.com/jfclere/tomcat-openshift or https://github.com/apache/tomcat/tree/master/modules/stuffed to build the images.

3. Create a Tomcat instance (Custom Resource). An example has been provided in _deploy/crds/web.servers.org_v1alpha1_jbosswebserver_cr.yaml_

```
  applicationName: jws-app
  applicationImage: docker.io/jfclere/tomcat-demo
```

Make sure imageStreamName is commented out otherwise the operator will try to build from the sources

4. Then deploy your webapp.

```bash
$ oc apply -f deploy/crds/web.servers.org_v1alpha1_jbosswebserver_cr.yaml
```

5. If you are on OpenShift the operator will create the route for you and you can use it

```bash
oc get routes
NAME      HOST/PORT                                            PATH      SERVICES   PORT      TERMINATION   WILDCARD
jws-app   jws-app-jws-operator.apps.jclere.rhmw-runtimes.net             jws-app    <all>                   None
```

Then go to http://jws-app-jws-operator.apps.jclere.rhmw-runtimes.net/demo-1.0/demo using a browser.

6. On kubernetes you have to create a balancer to expose the service and later something depending on your cloud to expose the application

```bash
kubectl expose deployment jws-app --type=LoadBalancer --name=jws-balancer
kubectl kubectl get svc
NAME              TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
jws-balancer      LoadBalancer   10.100.57.140   <pending>     8080:32567/TCP   4m6s
```

The service jws-balancer then can be used to expose the application.

## Configuring Readyness or Liveness probes:

serverReadinessScript and serverLivenessScript allow to use a custom live or ready probe, we support 2 formats:

```
serverLivenessScript: cmd arg1 arg2 ...
serverLivenessScript: shell shellarg1 shellargv2 ... "cmd line for the shell"
```

Don't forget '\' if you need to escape something in the cmd line. Don't use ' ' in the arg that is the separator we support.

In case you don't use the HealthCheckValve you have to configure at least a serverReadinessScript.

For example if you are using the JWS 5.3 images you need the following:

```
  # For pre JWS-5.4 image you need to set username/password and use the following health check.
  JbossWebServerHealthCheck:
    serverReadinessScript: /bin/bash -c "/usr/bin/curl --noproxy '*' -s -u ${JWS_ADMIN_USERNAME}:${JWS_ADMIN_PASSWORD} 'http://localhost:8080/manager/jmxproxy/?get=Catalina%3Atype%3DServer&att=stateName' | /usr/bin/grep -iq 'stateName *= *STARTED'"
    JbossWebServer53HealthCheck:
      jwsAdminUsername: tomcat
      jwsAdminPassword: tomcat
```

The 5.3 are using the manager webapp and jmx to figure if the server is started.

For example if you are using a openjdk:8-jre-alpine based image and /test is your health URL:

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
