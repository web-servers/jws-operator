# Monitoring JWS6 webserver created by the operator.
The JWS operator 2.0.x doesn't create the service and servicemonitor you need to follow the Metrics, but they can be created by hands. Note that _won't_ work for JWS5 images.

## Create your webserver:
```
apiVersion: web.servers.org/v1alpha1
kind: WebServer
metadata:
  name: example-image-webserver
  namespace: jclere-namespace
spec:
  applicationName: jws-app
  replicas: 2
  webImage:
    applicationImage: registry.redhat.io/jboss-webserver-6/jws60-openjdk17-openshift-rhel8
```
Note that *jws-app* and *example-image-server* correspond to the labels in service and servicemonitor.

## Create a service something like:
```
apiVersion: v1
kind: Service
metadata:
  labels:
    WebServer: example-image-webserver
    app.kubernetes.io/name: example-image-webserver
    application: jws-app
    deploymentConfig: jws-app
  name: example-image-webserver-admin
  namespace: jclere-namespace
spec:
  clusterIP: None
  clusterIPs:
  - None
  internalTrafficPolicy: Cluster
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
  - name: admin
    port: 9404
    protocol: TCP
    targetPort: 9404
  selector:
    WebServer: example-image-webserver
    app.kubernetes.io/name: example-image-webserver
    application: jws-app
    deploymentConfig: jws-app
  sessionAffinity: None
  type: ClusterIP
```

## Create the servicemonitor:
```
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    WebServer: example-image-webserver
    app.kubernetes.io/name: example-image-webserver
    application: jws-app
    deploymentConfig: jws-app
  name: example-image-webserver
  namespace: jclere-namespace
spec:
  endpoints:
  - bearerTokenSecret:
      key: ""
    port: admin
  namespaceSelector: {}
  selector:
    matchLabels:
      WebServer: example-image-webserver
      app.kubernetes.io/name: example-image-webserver
      application: jws-app
      deploymentConfig: jws-app
```

## Check the Metrics:
On the Openshift console go to *Observe/Metrics, type *tomcat* and choose the Metrics you want to follow.  Note that the JVM metrics are exposed too, type *jvm_* to choose the one you want.
