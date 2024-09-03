# Adding a TLS/https route to a JWS6 webserver created by the operator.
The 2.x version of operator creates a http route/hostname, the document here explains how to redirect the http to https modifying a JWS6 image and how to create a TLS https route

## Modify the JWs6 image.
The JWS6 images contain the RewriteValve, it needs to be enable in the server.xml and add  rewrite.config the image. We do that use a DockerFile and podman.

### Create a Dockerfile
Something like:
```
FROM registry.redhat.io/jboss-webserver-6/jws60-openjdk17-openshift-rhel8:latest

COPY rewrite.config /opt/jws-6.0/tomcat/conf/Catalina/localhost/rewrite.config

RUN sed -i '/org.apache.catalina.valves.HealthCheckValve/ a \\        <Valve className="org.apache.catalina.valves.rewrite.RewriteValve"/>' /opt/jws-6.0/tomcat/conf/server.xml
```

### Create a rewrite.config file:
Something like:
```
RewriteCond %{HTTPS} =off
RewriteRule ^/?(.*) https://%{HTTP_HOST}%{REQUEST_URI} [R,NE,L]
```

### Build the image:
```
podman build .
STEP 1/3: FROM registry.redhat.io/jboss-webserver-6/jws60-openjdk17-openshift-rhel8:latest
STEP 2/3: COPY rewrite.config /opt/jws-6.0/tomcat/conf/Catalina/localhost/rewrite.config
--> fd7920c56791
STEP 3/3: RUN sed -i '/org.apache.catalina.valves.HealthCheckValve/ a \\        <Valve className="org.apache.catalina.valves.rewrite.RewriteValve"/>' /opt/jws-6.0/tomcat/conf/server.xml
COMMIT
--> e5efe9c7fd1a
e5efe9c7fd1a771e6d50616220b501e68ad855c73f71cd10f06df63c2c6e385a
```

### Tag it and push it:
```
podman tag e5efe9c7fd1a quay.io/${USER}/test-tls
```
Note the *e5efe9c7fd1a* you get from the build or via podman images.
```
podman push quay.io/rhn_engineering_jclere/test-tls
Getting image source signatures
Copying blob f47bdcdccfb5 done   | 
Copying blob 156997994489 done   | 
Copying blob 01284470164a skipped: already exists  
Copying blob d66a972c0896 skipped: already exists  
Copying blob 8f12bbb35a5d skipped: already exists  
Copying config e5efe9c7fd done   | 
Writing manifest to image destination
```

### Create a webserver using the operator:
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
    applicationImage: quay.io/i${USER}/test-tls
```

### Find the http route/hostname and create the TLS one:
```
oc get routes
NAME            HOST/PORT                                                       PATH   SERVICES        PORT    TERMINATION     WILDCARD
jws-app         jws-app-jclere-namespace.apps.MYCLUSTERBASENAME                jws-app         <all>                   None
```
The hostname we want to use for TLS is *jws-app-jclere-namespace.apps.MYCLUSTERBASENAME*, create the TLS/https as following:
```
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: jws-app-tls
spec:
  host: jws-app-jclere-namespace.apps.MYCLUSTERBASENAME
  path: /
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
  to:
    kind: Service
    name: jws-app
```
Note the *path: /* otherwise Openshift will complain that jws-app-jclere-namespace.apps.MYCLUSTERBASENAME already exists.
Check the result:
```
oc get routes
NAME            HOST/PORT                                                       PATH   SERVICES        PORT    TERMINATION     WILDCARD
jws-app         jws-app-jclere-namespace.apps.MYCLUSTERBASENAME                        jws-app         <all>                   None
jws-app-tls     jws-app-jclere-namespace.apps.MYCLUSTERBASENAME                 /      jws-app         <all>   edge/Redirect   None
```
And via curl:
```
curl -v -k http://jws-app-jclere-namespace.apps.jMYCLUSTERBASENAME/
* Host jws-app-jclere-namespace.apps.MYCLUSTERBASENAME:80 was resolved.
* IPv6: (none)
* IPv4:x.x.x.x
*   Trying x.x.x.x:80...
* Connected to jws-app-jclere-namespace.apps.MYCLUSTERBASENAME (x.x.x.x) port 80
> GET / HTTP/1.1
> Host: jws-app-jclere-namespace.apps.MYCLUSTERBASENAME
> User-Agent: curl/8.6.0
> Accept: */*
> 
< HTTP/1.1 302 Found
< content-length: 0
< location: https://jws-app-jclere-namespace.apps.MYCLUSTERBASENAME/
< cache-control: no-cache
```
And:
```
curl -v -k https://jws-app-jclere-namespace.apps.MYCLUSTERBASENAME/
* Host jws-app-jclere-namespace.apps.MYCLUSTERBASENAME:443 was resolved.
....
* TLSv1.3 (IN), TLS handshake, Newsession Ticket (4):
* TLSv1.3 (IN), TLS handshake, Newsession Ticket (4):
* old SSL session ID is stale, removing
< HTTP/1.1 200 
< set-cookie: JSESSIONID=EF1F697630C38A2E085E6FF9A4720568; Path=/; Secure; HttpOnly
< content-type: text/html;charset=UTF-8
< content-length: 13
< date: Fri, 30 Aug 2024 12:27:03 GMT
< set-cookie: 2fc63e1f0641df2e33da499756a16425=399bec9ab17cc03659cd2c10f21a2cfe; path=/; HttpOnly; Secure; SameSite=None
< 
Hello World!
* Connection #0 to host jws-app-jclere-namespace.apps.MYCLUSTERBASENAME left intact
```
Note that -k option to curl because I have not installed the CA certificate of MYCLUSTERBASENAME.
