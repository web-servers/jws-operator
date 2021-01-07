# This directory contains the test for the operator it requires:

1 - To be connected to an openshift cluster where you can run the JWS operator.

2 - An ImageStream named "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift"
```
oc create -f xpaas-streams/jws54-tomcat9-image-stream.json
```
3 - A project must be created
```
oc new-project test
```
4 - Is run by make test in the jws-operator directory.
```
make test
```
# The following is tested.

## 1 - BasicTest
It tests:
```
apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example-webserver-123456
spec:
  applicationName: example-webserver-123456
  replicas: 1
  webImage:
    applicationImage: quay.io/jfclere/jws-image:5.4
```
The test starts the pod, waits for it, scale to 2 replicas, waits for them and check that http://route_host/health is returning 200.

## 2 - ImageStreamTest
It tests:
```
apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example-webserver-123457
spec:
  # Add fields here
  applicationName: example-webserver-123457
  replicas: 1
  webImageStream:
    imageStreamNamespace: default
    imageStreamName: jboss-webserver54-openjdk8-tomcat9-ubi8-openshift
```
The test starts the pod, waits for it, scale to 2 replicas, waits for them and check that http://route_host/health is returning 200.

## 3 - SourcesTest
```
apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example-webserver-apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example-webserver-123458
spec:
  # Add fields here
  applicationName: example-webserver-123458
  replicas: 1
  useSessionClustering: true
  webImageStream:
    imageStreamNamespace: default
    imageStreamName: jboss-webserver54-openjdk8-tomcat9-ubi8-openshift
    webSources:
      sourceRepositoryUrl: "https://github.com/jfclere/demo-webapp"
      sourceRepositoryRef: "master"
      contextDir: /
```
The test starts the pod, waits for it, scale to 4 replicas, waits for them and check that http://route_host/demo-1.0/demo works correctly.
It test that the counter of the webapp is increased for each request and that the 4 pods are reachable and correctly using the ASF Tomcat session clustering.
