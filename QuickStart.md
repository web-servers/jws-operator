The JWS operator brings two main features, deploy a prepared image or build an image from sources and a existing ImageStream.

## Deploy a prepared image
You have to prepare an image with the health check valve or any other way to check that the webapp you want to deploy is running.

The JWS-5.4 image have the Apache Tomcat health check valve already configured.

If you are building your image from the ASF Tomcat
make sure you have the health check valve enabled in the server.xml.
See https://tomcat.apache.org/tomcat-9.0-doc/config/valve.html#Health_Check_Valve
for more detail.

If you are using you own health check logic you have to fill See
https://github.com/web-servers/jws-operator/blob/master/Parameters.md#serverreadinessscript

The minimal yaml file you need is something like:
```
apiVersion: web.servers.org/v1alpha1
kind: JBossWebServer
metadata:
  name: example
spec:
  replicas: 2
  applicationName: jws-app
  applicationImage: docker.io/jfclere/tomcat-demo
```
In cluster just run
```
oc create -f minimal.yaml
```
The operator will deploy 2 pods running the image docker.io/jfclere/tomcat-demo, 2 services and a route to access to the application.

The route is something like http://jws-app-jws-operator.example.[cluster base name]/

## Build a image and deploy it
You have to create an ImageStream, use a file with something like in the following file:
https://github.com/web-servers/jws-operator/blob/master/xpaas-streams/jws54-tomcat9-image-stream.json

Create the ImageStream jboss-webserver54-openjdk8-tomcat9-ubi8-openshift:
```
oc create -f https://github.com/web-servers/jws-operator/blob/master/xpaas-streams/jws54-tomcat9-image-stream.json
```

Prepare you webapps to build with mvn install, put it in a git url your cluster can access, then create a minimal yaml file like the following
```
apiVersion: web.servers.org/v1alpha1
kind: JBossWebServer
metadata:
  name: example
spec:
  replicas: 2
  applicationName: jws-app
  imageStreamName: 'jboss-webserver54-tomcat9-openshift:latest'
  sourceRepositoryUrl: 'https://github.com/jfclere/demo-webapp.git'
  sourceRepositoryRef: master
  contextDir: /
  genericWebhookSecret: " "
```
The operator will compile and build an image using the war mvn install creates in target. The image is stored in an Image Stream. The operator creates a ReplicaSet 2 service and router. It deploys 2 pods.

The route is something like http://jws-app-jws-operator.example.[cluster base name]/
