# Parameters to use in CRD

## replicas (mandatory all configuration)
The number of pods of the JBoss Web Server image you want to run.

```
  replicas: 2
```

## applicationName (mandatory all configuration)
The name of the application, it must be unique in the namespace/project. Note that it is used to create the route to access
to that application.

```
  applicationName: test
```

## useSessionClustering (off if not filled)
Use the DNSping or KUBEping session clustering if filled, default don't use session clustering (Note that image needs to be based on JWS images as the feature use ENV_FILES environment variable and a shell script to add the clustering in server.xml)

```
  useSessionClustering: true
```
## routeHostname
Create a route or not (NONE) and tell if the route uses TLS (tls) and allow to specify a hostname.
```
  routeHostname: NONE
```
The route will NOT be created by the operator, it needs to be create by hands.
```
  routeHostname: tls
```
The operator will create a passthrough route to the tomcat.

## certificateVerification
Use the TLS connector with a client certificates. The value are required, optional or empty see Tomcat connector docs and look for certificateVerification in the connector.
```
  certificateVerification: required
```

## TLSSecret
Secret to use for the server certificate (server.cert) and server key (server.key) and optional CA certificat for the client certificates (ca.cert).
```
  tlsSecret: tlssecret
```

## TLSPassord
The passpharse used to protect the server key.
```
  tlsPassword: changeit
```

## Resources
The configuration of the resources used by the webserver, ie CPU and memory, Use limits and requests.
Those are used for the auto scaling.
```
   resources:
     limits:
       cpu: 500m
     requests:
       cpu: 200m
```
See Horizontal Pod Autoscaling in openshift or kubernetes for more details how to use it.

## PersistentLogs
If PersistentLogs is true catalina.out of every pod will be saved in a PersistentVolume in order to remain available after a possible pod failure.
```
  persistentLogs: true
```

## EnableAccessLogs
If EnableAccessLogs is true but PersistentLogs is false access log will just get produced but not saved in a PV. Access logs of every pod will be saved in a PV in case that EnableAccessLogs and PersistentLogs are true in parallel.
```
  enableAccessLogs: true
```

## IsNotJWS
This parameter is used with PersistentLogs or/and EnableAccessLogs to show the operator how to configure the container for persistent logs because JWS image needs different configuration than the ASF tomcat imag. Setting it to true means that the given image is ASF tomcat image.
```
  isNotJWS: true
```
## volumeName
If PersistentLogs is true, volumeName is the name of PersistentVolume used to store the access_log and catalina.out
```
  volumeName: pv0000
```

## storageClass
If PersistentLogs is true, storageClass is the name of storageClass of the PersistentVolume used to store the access_log and catalina.out
```
  storageClassvolumeName: nfs-client
```

## webImage (to deploy from existing images)
The webImage controls how to deploy pods from existing images.
It has the applicationImage (Mandatory), webApp (might be empty) and webServerHealthCheck (a default is used when empty)


### applicationImage (to deploy an existing image or build from an existing image)
The URL of the image you want to use with the operator. For example:

```
applicationImage: docker.io/jfclere/tomcat-demo
```
### imagePullSecret
The secret to use to pull images for the repository, the secret must contain the key .dockerconfigjson and will be mounted by
the operator to be used like --authfile /mount_point/.dockerconfigjson to pull the image to deploy the pods.
Note that the file might contain several user/password or token to access to the images in the ImageStream, the image builder and the images built by the operator.

### webApp
Describes how the operator will build the webapp to add to application image, if not present the application is just deployed.
It has the sourceRepositoryUrl (Mandatory), sourceRepositoryRef, contextDir, webAppWarImage, webAppWarImagePushSecret,Name and builder.

### webServerHealthCheck
Describes how the operator will create the health check for the created pods.


## webImageStream (to deploy from an ImageStream, openshift only)
The webImageStream controls how the operator will use an ImageStream that provides images to run or to build upon. The latest image in the stream is used.
It has the imageStreamName (Mandatory), imageStreamNamespace, webSources (might be empty) and webServerHealthCheck (a default is used when empty)

### imageStreamName
The name of the image stream you created to allow the operator to find the base images.

```bash
oc create -f xpaas-streams/jws56-tomcat9-image-stream.json
imagestream.image.openshift.io/jboss-webserver56-tomcat9-openshift created
```

Here: imageStream: `jboss-webserver56-tomcat9-openshift:latest`

### imageStreamNamespace
The namespace/project in which you created the image stream

```bash
oc create -f xpaas-streams/jws56-tomcat9-image-stream.json -n jfc
imagestream.image.openshift.io/jboss-webserver56-tomcat9-openshift created
```

Here: imageStreamNamespace: jfc

### webSources
Describes where the sources are located and how build them, if empty the latest image in ImageStream is deployed)
It has the sourceRepositoryUrl (Mandatory), sourceRepositoryRef, ContextDir and webSourcesParams

#### sourceRepositoryUrl
The URL where the sources are located. The source should contain a maven pom.xml to allow for a maven build. The produced war is put
in the webapps directory of image /opt/jws-5.x/tomcat/webapps. See `artifactDir` as well.

```
 sourceRepositoryUrl: 'https://github.com/jfclere/demo-webapp.git'
```

#### sourceRepositoryRef
The branch of the source repository that will be used.

```
sourceRepositoryRef: main
```

#### contextDir

The sub directory where the pom.xml is located and where the `mvn install` should be run.

```
  contextDir: /
```

#### webSourcesParams
Those are additional parameter of webSourcesParams to describe how to build the application images.

##### artifactDir (webSourcesParams)
The artifactDir is a parameter of the SourceBuildStrategy the operator is using. It is the directory where maven places the war it creates for the webapp.
The contents of artifactDir is copied in the webapps directory of the image used to deploy the application /opt/jws-5.x/tomcat/webapps. The default value is target.

##### mavenMirrorUrl (webSourcesParams)
The mavenMirrorUrl is a parameter of the SourceBuildStrategy the operator is using. It is the maven proxy URL that maven will use to build the webapp. It is required if the cluster doesn't have access to the Internet.

##### genericWebhookSecret (webSourcesParams)
This explains how to use a secret for a generic webhook to trigger a build.

1 - Create a base64 secret string:
Base64 encoded string secret can be created by base64 tool. In the following example, the secret "qwerty" is used

```bash
echo -n "qwerty" | base64
cXdlcnR5
```
2 - Create a secret.yaml like:

```
kind: Secret
apiVersion: v1
metadata:
  name: jws-secret
data:
  WebHookSecretKey: cXdlcnR5
```
3 - Create the secret:
```bash
oc create -f secret.yaml
secret/jws-secret created
```

So here we use:
```
genericWebhookSecret: jws-secret
```

To test it:
You can send a request via _curl_ to an URL which looks like:

```
https://<openshift_api_host:port>/apis/build.openshift.io/v1/namespaces/<namespace>/buildconfigs/<name>/webhooks/<secret>/generic
```

1 - Get the URL:

```bash
oc describe BuildConfig | grep webhooks
	URL:		https://api.jclere.rhmw-runtimes.net:6443/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test/webhooks/<secret>/generic
```

2 - Replace "<secret>" with the secret value (here qwerty).

3 - Send a _curl_ request:

```bash
curl -k -X POST https://api.jclere.rhmw-runtimes.net:6443/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test/webhooks/qwerty/generic
{"kind":"Build","apiVersion":"build.openshift.io/v1","metadata":{"name":"test-2","namespace":"jfc","selfLink":"/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test-2/instantiate","uid":"a72dd529-edc6-4e1c-898e-7c0dbbea176e","resourceVersion":"846159","creationTimestamp":"2020-10-30T12:29:30Z","labels":{"application":"test","buildconfig":"test","openshift.io/build-config.name":"test","openshift.io/build.start-policy":"Serial"},"annotations":{"openshift.io/build-config.name":"test","openshift.io/build.number":"2"},"ownerReferences":[{"apiVersion":"build.openshift.io/v1","kind":"BuildConfig","name":"test","uid":"1f78fa3f-2f3b-421b-9f49-192184cc2280","controller":true}],"managedFields":[{"manager":"openshift-apiserver","operation":"Update","apiVersion":"build.openshift.io/v1","time":"2020-10-30T12:29:30Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:openshift.io/build-config.name":{},"f:openshift.io/build.number":{}},"f:labels":{".":{},"f:application":{},"f:buildconfig":{},"f:openshift.io/build-config.name":{},"f:openshift.io/build.start-policy":{}},"f:ownerReferences":{".":{},"k:{\"uid\":\"1f78fa3f-2f3b-421b-9f49-192184cc2280\"}":{".":{},"f:apiVersion":{},"f:controller":{},"f:kind":{},"f:name":{},"f:uid":{}}}},"f:spec":{"f:output":{"f:to":{".":{},"f:kind":{},"f:name":{}}},"f:serviceAccount":{},"f:source":{"f:contextDir":{},"f:git":{".":{},"f:ref":{},"f:uri":{}},"f:type":{}},"f:strategy":{"f:sourceStrategy":{".":{},"f:env":{},"f:forcePull":{},"f:from":{".":{},"f:kind":{},"f:name":{}},"f:pullSecret":{".":{},"f:name":{}}},"f:type":{}},"f:triggeredBy":{}},"f:status":{"f:conditions":{".":{},"k:{\"type\":\"New\"}":{".":{},"f:lastTransitionTime":{},"f:lastUpdateTime":{},"f:status":{},"f:type":{}}},"f:config":{".":{},"f:kind":{},"f:name":{},"f:namespace":{}},"f:phase":{}}}}]},"spec":{"serviceAccount":"builder","source":{"type":"Git","git":{"uri":"https://github.com/jfclere/demo-webapp.git","ref":"master"},"contextDir":"/"},"strategy":{"type":"Source","sourceStrategy":{"from":{"kind":"DockerImage","name":"image-registry.openshift-image-registry.svc:5000/jfc/jboss-webserver54-tomcat9-openshift@sha256:75dcdf81011e113b8c8d0a40af32dc705851243baa13b68352706154174319e7"},"pullSecret":{"name":"builder-dockercfg-rvbh8"},"env":[{"name":"MAVEN_MIRROR_URL"},{"name":"ARTIFACT_DIR"}],"forcePull":true}},"output":{"to":{"kind":"ImageStreamTag","name":"test:latest"}},"resources":{},"postCommit":{},"nodeSelector":null,"triggeredBy":[{"message":"Generic WebHook","genericWebHook":{"secret":"\u003csecret\u003e"}}]},"status":{"phase":"New","config":{"kind":"BuildConfig","namespace":"jfc","name":"test"},"output":{},"conditions":[{"type":"New","status":"True","lastUpdateTime":"2020-10-30T12:29:30Z","lastTransitionTime":"2020-10-30T12:29:30Z"}]}}
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {},
  "status": "Success",
  "message": "invalid Content-Type on payload, ignoring payload and continuing with build",
  "code": 200
}
```

The build is triggered.

> **_NOTE_**
> In case of __User \"system:anonymous\" cannot create resource__ error, it can be resolved by adding unauthenticated users to the system:webhook role binding or by token:
> ``` TOKEN=`oc create token builder` ```
> ```curl -H "Authorization: Bearer $TOKEN" -k -X POST https://api.jclere.rhmw-runtimes.net:6443/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test/webhooks/qwerty/generic```

3 - Use it in github:

Go to Setting+Webhooks+Add webhook in your github project and add the URL in the Payload URL, set Content type: application/json, Disable SSL verification if needed and click Add webhook. See https://docs.openshift.com/container-platform/4.6/builds/triggering-builds-build-hooks.html for more details.

##### githubWebhookSecret (webSourcesParams)
That is a web hook specific to GitHub, it works like `genericWebhookSecret`

```
githubWebhookSecret: jws-secret
```
Note that it is not possible to test the Github webhook by hands: The playload is generated by github and it is NOT empty.


## webServerHealthCheck (webImage and webImageStream)
The health check that the operator will use. The default behavior is to use the health valve which doesn't require any parameters.

### serverReadinessScript
String for the pod readiness health check logic. If left empty the default health check is used (it checks http://localhost:8080/health using OpenShift internal)
Example :

```
serverReadinessScript: /bin/bash -c " /usr/bin/curl --noproxy '*' -s 'http://localhost:8080/health' | /usr/bin/grep -i 'status.*UP'"
```

For the formats see the README.md.

### serverLivenessScript
The script that checks if the pod is running. It's use is optional.

### Name (webapp)
The name of the webapp, default: ROOT.war

### webAppWarImage (webapp)
That is the URL of images where the operator will push what he builds.

### webAppWarImagePushSecret (webapp)
The secret to use to push images to the repository, the secret must contain the key .dockerconfigjson and will be mounted by
the operator to be used like --authfile /mount_point/.dockerconfigjson to push the image to repository. Note that if you need a pull secret for the FROM image the webAppImagePushSecret must contain it too.

### builder (webapp)
It describes how the webapp is build and the docker image is made and push to a docker repository.

#### image (webapp.builder)
That is the image to use to build
```
builder: quay.io/jfclere/tomcat10-buildah
```
#### imagePullSecret (webapp.builder)
If there is an imagePullSecret, that it should also contain the secret to pull the image of the image builder if needed.

#### applicationBuildScript (webapp.builder)
That is the script to use to build and push the image, if empty a default script using maven and buildah is used.
