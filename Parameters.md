## Parameter to use in CRD

# imageStreamNamespace

That is the ImageStream you created to allow the operator to find the base images:

```bash
oc create -f xpaas-streams/jws53-tomcat9-image-stream.json
imagestream.image.openshift.io/jboss-webserver53-tomcat9-openshift created
```
here: ImageStream=jboss-webserver53-tomcat9-openshift

# imageStreamNamespace

That is the namespace/project in with you create the ImageStream
```bash
oc create -f xpaas-streams/jws53-tomcat9-image-stream.json -n jfc
imagestream.image.openshift.io/jboss-webserver53-tomcat9-openshift created
```
Here: imageStreamNamespace=jfc

# githubWebhookSecret

That is the secret github will use to trigger a build.
Create a secret.yaml like:
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
```
base64 secret.txt
cXdlcnR5Cg==
```
DRAFT in progress...


imagestream.image.openshift.io/jboss-webserver52-tomcat9-openshift created
test-1-n2tr8
 imageStreamNamespace: jfc
  githubWebhookSecret: qwerty
  jwsAdminPassword: toto
  imageStreamName: 'jboss-webserver53-tomcat9-openshift:latest'
  sourceRepositoryUrl: 'https://github.com/jfclere/demo-webapp.git'
  sourceRepositoryRef: master
  genericWebhookSecret: qwerty
  serverReadinessScript: >-
    /bin/bash -c "/usr/bin/curl --noproxy '*' -s -u
    ${JWS_ADMIN_USERNAME}:${JWS_ADMIN_PASSWORD}
    'http://localhost:8080/manager/jmxproxy/?get=Catalina%3Atype%3DServer&att=stateName'
    | /usr/bin/grep -iq 'stateName *= *STARTED'"
  replicas: 2
  applicationName: test
  contextDir: /
  jwsAdminUsername: toto
