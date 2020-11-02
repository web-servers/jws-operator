## Parameter to use in CRD

# replicas
That is the number of pods of the JWS image you want to run. Use at least 2!
```
  replicas: 2
```

# applicationName
That is the name of the application, it must be unique in the namespace/project. Note it is used to create the route to access
to that application.
```
  applicationName: test
 ```
# applicationImage
That is the URL to find the image you want to use with the operator. For example:
```
applicationImage: docker.io/jfclere/tomcat-demo
```

# imageStreamName

That is the ImageStream you created to allow the operator to find the base images:

```bash
oc create -f xpaas-streams/jws53-tomcat9-image-stream.json
imagestream.image.openshift.io/jboss-webserver53-tomcat9-openshift created
```
Here: imageStream: `jboss-webserver53-tomcat9-openshift:latest`

# imageStreamNamespace

That is the namespace/project in which you create the ImageStream
```bash
oc create -f xpaas-streams/jws53-tomcat9-image-stream.json -n jfc
imagestream.image.openshift.io/jboss-webserver53-tomcat9-openshift created
```
Here: imageStreamNamespace: jfc

# genericWebhookSecret

That is a secret for a webhook to trigger a build.
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
{"kind":"Build","apiVersion":"build.openshift.io/v1","metadata":{"name":"test-2","namespace":"jfc","selfLink":"/apis/build.openshift.io/v1/namespaces/jfc/buildconfigs/test-2/instantiate","uid":"a72dd529-edc6-4e1c-898e-7c0dbbea176e","resourceVersion":"846159","creationTimestamp":"2020-10-30T12:29:30Z","labels":{"application":"test","buildconfig":"test","openshift.io/build-config.name":"test","openshift.io/build.start-policy":"Serial"},"annotations":{"openshift.io/build-config.name":"test","openshift.io/build.number":"2"},"ownerReferences":[{"apiVersion":"build.openshift.io/v1","kind":"BuildConfig","name":"test","uid":"1f78fa3f-2f3b-421b-9f49-192184cc2280","controller":true}],"managedFields":[{"manager":"openshift-apiserver","operation":"Update","apiVersion":"build.openshift.io/v1","time":"2020-10-30T12:29:30Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:openshift.io/build-config.name":{},"f:openshift.io/build.number":{}},"f:labels":{".":{},"f:application":{},"f:buildconfig":{},"f:openshift.io/build-config.name":{},"f:openshift.io/build.start-policy":{}},"f:ownerReferences":{".":{},"k:{\"uid\":\"1f78fa3f-2f3b-421b-9f49-192184cc2280\"}":{".":{},"f:apiVersion":{},"f:controller":{},"f:kind":{},"f:name":{},"f:uid":{}}}},"f:spec":{"f:output":{"f:to":{".":{},"f:kind":{},"f:name":{}}},"f:serviceAccount":{},"f:source":{"f:contextDir":{},"f:git":{".":{},"f:ref":{},"f:uri":{}},"f:type":{}},"f:strategy":{"f:sourceStrategy":{".":{},"f:env":{},"f:forcePull":{},"f:from":{".":{},"f:kind":{},"f:name":{}},"f:pullSecret":{".":{},"f:name":{}}},"f:type":{}},"f:triggeredBy":{}},"f:status":{"f:conditions":{".":{},"k:{\"type\":\"New\"}":{".":{},"f:lastTransitionTime":{},"f:lastUpdateTime":{},"f:status":{},"f:type":{}}},"f:config":{".":{},"f:kind":{},"f:name":{},"f:namespace":{}},"f:phase":{}}}}]},"spec":{"serviceAccount":"builder","source":{"type":"Git","git":{"uri":"https://github.com/jfclere/demo-webapp.git","ref":"master"},"contextDir":"/"},"strategy":{"type":"Source","sourceStrategy":{"from":{"kind":"DockerImage","name":"image-registry.openshift-image-registry.svc:5000/jfc/jboss-webserver53-tomcat9-openshift@sha256:75dcdf81011e113b8c8d0a40af32dc705851243baa13b68352706154174319e7"},"pullSecret":{"name":"builder-dockercfg-rvbh8"},"env":[{"name":"MAVEN_MIRROR_URL"},{"name":"ARTIFACT_DIR"}],"forcePull":true}},"output":{"to":{"kind":"ImageStreamTag","name":"test:latest"}},"resources":{},"postCommit":{},"nodeSelector":null,"triggeredBy":[{"message":"Generic WebHook","genericWebHook":{"secret":"\u003csecret\u003e"}}]},"status":{"phase":"New","config":{"kind":"BuildConfig","namespace":"jfc","name":"test"},"output":{},"conditions":[{"type":"New","status":"True","lastUpdateTime":"2020-10-30T12:29:30Z","lastTransitionTime":"2020-10-30T12:29:30Z"}]}}
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

# githubWebhookSecret
That is a web hook specific to GitHub, it works like `genericWebhookSecret`
```
githubWebhookSecret: qwerty
```

# jwsAdminUsername
That is the admin user created in /opt/jws-5.3/tomcat/conf/tomcat-users.xml for the JWS-5.3 health check only.
```
jwsAdminUsername: tomcat
```

# jwsAdminPassword
That is the password of the user created in /opt/jws-5.3/tomcat/conf/tomcat-users.xml for the JWS-5.3 health check only.
```
  jwsAdminPassword: tomcat
```
# serverReadinessScript
That is the script used to check if a pod is ready. That is only mandary for JWS-5.3 iamges.
An empty or not existing is the default. The default is to check http://localhost:8080/health using OpenShift internal.
For example the JWS-5.3 is:
```
  serverReadinessScript: >-
    /bin/bash -c "/usr/bin/curl --noproxy '*' -s -u
    ${JWS_ADMIN_USERNAME}:${JWS_ADMIN_PASSWORD}
    'http://localhost:8080/manager/jmxproxy/?get=Catalina%3Atype%3DServer&att=stateName'
    | /usr/bin/grep -iq 'stateName *= *STARTED'"
```
For the formats see the README.md.

# serverLivenessScript
That is the script to check that a pod is alive. That is NOT mandatory.

# sourceRepositoryUrl
That the URL were the sources are located, the source should have a maven pom.xml to allow a maven build, the produced war is put
in the webapps directory of image /opt/jws-5.x/tomcat/webapps. See `ARTIFACT_DIR" here too.
```
 sourceRepositoryUrl: 'https://github.com/jfclere/demo-webapp.git'
```
# sourceRepositoryRef
That is the branch you want to build.
```
sourceRepositoryRef: master
```
# contextDir
That is the sub directory where the pom.xml is located and where the `mvn install` should be run.
```
  contextDir: /
```
# ARTIFACT_DIR
The ARTIFACT_DIR is a parameter of SourceBuildStrategy the operator is using. It is the directory were maven put the war it creates for the webapp.
The contents of ARTIFACT_DIR is copied in the webapps directory of the image /opt/jws-5.x/tomcat/webapps the default value is target.
# MAVEN_MIRROR_URL
The MAVEN_MIRROR_URL is a parameter of SourceBuildStrategy the operator is using. It is the maven proxy URL maven will use to build the webapp. You need it if your cluster has not access to the Internet.
