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

Use the DNSping session clustering if filled, default don't use session clustering (Note that image needs to be based on JWS images as the feature use ENV_FILES environment variable and a shell script to add the clustering in server.xml)
```
  useSessionClustering: true
```

## applicationImage (customized images) (Method 1)

The URL of the image you want to use with the operator. For example:

```
applicationImage: docker.io/jfclere/tomcat-demo
```

## webImageStream (Method 2)

The image stream that provides images to run or to build upon. The latest image in the stream is used.

### imageStreamName (mandatory BuildImage)

The name of the image stream you created to allow the operator to find the base images.

```bash
oc create -f xpaas-streams/jws54-tomcat9-image-stream.json
imagestream.image.openshift.io/jboss-webserver54-tomcat9-openshift created
```

Here: imageStream: `jboss-webserver54-tomcat9-openshift:latest`

### imageStreamNamespace (BuildImage only)

The namespace/project in which you created the image stream

```bash
oc create -f xpaas-streams/jws54-tomcat9-image-stream.json -n jfc
imagestream.image.openshift.io/jboss-webserver54-tomcat9-openshift created
```

Here: imageStreamNamespace: jfc

### webSources

Describes where the sources are located and how build them

#### sourceRepositoryUrl (Mandatory BuildImage)

The URL where the sources are located. The source should contain a maven pom.xml to allow for a maven build. The produced war is put
in the webapps directory of image /opt/jws-5.x/tomcat/webapps. See `artifactDir` as well.

```
 sourceRepositoryUrl: 'https://github.com/jfclere/demo-webapp.git'
```

#### sourceRepositoryRef (Mandatory BuildImage)

The branch of the source repository that will be used.

```
sourceRepositoryRef: master
```

#### contextDir (BuildImage only)

The sub directory where the pom.xml is located and where the `mvn install` should be run.

```
  contextDir: /
```

#### webSourcesParams

Those are additional parameter of webSourcesParams to describe how to build the application images.

##### artifactDir (BuidImage only)

The artifactDir is a parameter of the SourceBuildStrategy the operator is using. It is the directory where maven places the war it creates for the webapp.
The contents of artifactDir is copied in the webapps directory of the image used to deploy the application /opt/jws-5.x/tomcat/webapps. The default value is target.

##### mavenMirrorUrl (BuildImage only)

The mavenMirrorUrl is a parameter of the SourceBuildStrategy the operator is using. It is the maven proxy URL that maven will use to build the webapp. It is required if the cluster doesn't have access to the Internet.

##### genericWebhookSecret (BuildImage only)

This is a secret for a generic webhook to trigger a build.
Create a secret.yaml like:

```
kind: Secret
apiVersion: v1
metadata:
  name: qwerty
data:
  WebHookSecretKey: cXdlcnR5Cg==
```

For the value of WebHookSecretKey use a file and base64 to encode it:
Put in secret.txt

```
qwerty
```

And run base64

```bash
base64 secret.txt
cXdlcnR5Cg==
```

Here: genericWebhookSecret: qwerty

To test it:
1 - get the URL:

```bash
oc describe BuildConfig | grep webhooks
	URL:		https://api.jclere.rhmw-runtimes.net:6443/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test/webhooks/<secret>/generic
```

2 - Create a minimal JSON file (payload.json)

```
{}
```

3 - Cut the URL replacing <secret> by its value and use the minimal JSON file:

```bash
curl -H "X-GitHub-Event: push" -H "Content-Type: application/json" -k -X POST --data-binary @payload.json https://api.jclere.rhmw-runtimes.net:6443/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test/webhooks/qwerty/generic
{"kind":"Build","apiVersion":"build.openshift.io/v1","metadata":{"name":"test-2","namespace":"jfc","selfLink":"/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test-2/instantiate","uid":"a72dd529-edc6-4e1c-898e-7c0dbbea176e","resourceVersion":"846159","creationTimestamp":"2020-10-30T12:29:30Z","labels":{"application":"test","buildconfig":"test","openshift.io/build-config.name":"test","openshift.io/build.start-policy":"Serial"},"annotations":{"openshift.io/build-config.name":"test","openshift.io/build.number":"2"},"ownerReferences":[{"apiVersion":"build.openshift.io/v1","kind":"BuildConfig","name":"test","uid":"1f78fa3f-2f3b-421b-9f49-192184cc2280","controller":true}],"managedFields":[{"manager":"openshift-apiserver","operation":"Update","apiVersion":"build.openshift.io/v1","time":"2020-10-30T12:29:30Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:openshift.io/build-config.name":{},"f:openshift.io/build.number":{}},"f:labels":{".":{},"f:application":{},"f:buildconfig":{},"f:openshift.io/build-config.name":{},"f:openshift.io/build.start-policy":{}},"f:ownerReferences":{".":{},"k:{\"uid\":\"1f78fa3f-2f3b-421b-9f49-192184cc2280\"}":{".":{},"f:apiVersion":{},"f:controller":{},"f:kind":{},"f:name":{},"f:uid":{}}}},"f:spec":{"f:output":{"f:to":{".":{},"f:kind":{},"f:name":{}}},"f:serviceAccount":{},"f:source":{"f:contextDir":{},"f:git":{".":{},"f:ref":{},"f:uri":{}},"f:type":{}},"f:strategy":{"f:sourceStrategy":{".":{},"f:env":{},"f:forcePull":{},"f:from":{".":{},"f:kind":{},"f:name":{}},"f:pullSecret":{".":{},"f:name":{}}},"f:type":{}},"f:triggeredBy":{}},"f:status":{"f:conditions":{".":{},"k:{\"type\":\"New\"}":{".":{},"f:lastTransitionTime":{},"f:lastUpdateTime":{},"f:status":{},"f:type":{}}},"f:config":{".":{},"f:kind":{},"f:name":{},"f:namespace":{}},"f:phase":{}}}}]},"spec":{"serviceAccount":"builder","source":{"type":"Git","git":{"uri":"https://github.com/jfclere/demo-webapp.git","ref":"master"},"contextDir":"/"},"strategy":{"type":"Source","sourceStrategy":{"from":{"kind":"DockerImage","name":"image-registry.openshift-image-registry.svc:5000/jfc/jboss-webserver54-tomcat9-openshift@sha256:75dcdf81011e113b8c8d0a40af32dc705851243baa13b68352706154174319e7"},"pullSecret":{"name":"builder-dockercfg-rvbh8"},"env":[{"name":"MAVEN_MIRROR_URL"},{"name":"ARTIFACT_DIR"}],"forcePull":true}},"output":{"to":{"kind":"ImageStreamTag","name":"test:latest"}},"resources":{},"postCommit":{},"nodeSelector":null,"triggeredBy":[{"message":"Generic WebHook","genericWebHook":{"secret":"\u003csecret\u003e"}}]},"status":{"phase":"New","config":{"kind":"BuildConfig","namespace":"jfc","name":"test"},"output":{},"conditions":[{"type":"New","status":"True","lastUpdateTime":"2020-10-30T12:29:30Z","lastTransitionTime":"2020-10-30T12:29:30Z"}]}}
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {

  },
  "status": "Success",
  "message": "no git information found in payload, ignoring and continuing with build",
  "code": 200
}
```

The build is triggered.

4 - Use it in github:

Go to Setting+Webhooks+Add webhook in your github project and add the URL in the Payload URL, set Content type: application/json, Disable SSL verification if needed and click Add webhook. See https://docs.openshift.com/container-platform/4.6/builds/triggering-builds-build-hooks.html for more details.

##### githubWebhookSecret (BuildImage only)

That is a web hook specific to GitHub, it works like `genericWebhookSecret`

```
githubWebhookSecret: qwerty
```
Note that it is not possible to test the Github webhook by hands: The playload is generated by github and it is NOT empty.

## webServerHealthCheck

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
