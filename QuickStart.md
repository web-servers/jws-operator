The JWS operator brings two main features, deploy a prepared image or build an image from sources and a existing ImageStream.

## Deploy a prepared image
You have to prepare an image with the health check valve or any other way to check that the webapp you want to deploy is running.
The JWS-5.4 image have the Apache Tomcat health check valve already configured if you are building your image from the ASF Tomcat
make sure you have the health check valve enabled in the server.xml. See https://tomcat.apache.org/tomcat-9.0-doc/config/valve.html#Health_Check_Valve
for more detail. If you are using you own health check logic you have to fill See
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

## Build a image and deploy it
