The JWS operator brings two main features, deploy a prepared image or build an image from sources and a existing ImageStream.

## Deploy a prepared image

You have to prepare an image with the health check valve or any other way to check that the webapp you want to deploy is running.

The JWS-5.4 image has the Apache Tomcat health check valve already configured.

If you are building your image from the ASF Tomcat
make sure you have the health check valve enabled in the server.xml.
See https://tomcat.apache.org/tomcat-9.0-doc/config/valve.html#Health_Check_Valve
for more detail.

If you are using you own health check logic you have to fill See
https://github.com/web-servers/jws-operator/blob/master/Parameters.md#serverreadinessscript

The minimal yaml file you need is something like:

```
apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example
spec:
  replicas: 2
  applicationName: jws-app
  applicationImage: docker.io/jfclere/tomcat-demo
```

Just run the following command in your cluster

```
oc create -f minimal.yaml
```

The operator will deploy 2 pods running the image docker.io/jfclere/tomcat-demo, 2 services and a route to access to the application.

The route is something like http://jws-app-jws-operator.example.[cluster base name]/

## Build a image base on JWS-5.4 and deploy it

If you want to use an image stream, use a file with something like in the following file:
https://github.com/web-servers/jws-operator/blob/master/xpaas-streams/jws54-tomcat9-image-stream.json

This creates the image stream jboss-webserver54-openjdk8-tomcat9-ubi8-openshift:

```
oc create -f https://github.com/web-servers/jws-operator/blob/master/xpaas-streams/jws54-tomcat9-image-stream.json
```

Prepare you webapps to build with mvn install, put it in a git URL that your cluster can access, then create a minimal yaml file like the following

```
apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example
spec:
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

Just run the following command in your cluster

```
oc create -f minimal.yaml
```

The operator will compile and build an image using the war mvn install creates in target. The image is stored in an Image Stream. The operator creates a ReplicaSet, 2 services and a Router. It deploys 2 pods.

The route is something like http://jws-app-jws-operator.example.[cluster base name]/
