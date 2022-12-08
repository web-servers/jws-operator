# Related to the JWS operator
This contains a set of script and yaml files to use to prepare a cluster to run the jws-operator test on it.

# Script to install PV PersistentVolume and SC StorageClass for PersistentLogs
The PersistentLogs allows to keep the access_log and catalina files in a PersistentVolume for analysis.
For the tests a nfs server pod is used to expose a mount point used in the PersistentVolume, the webserver pods use a PVC PersistentVolumeClaim to mount in each pod the nfs exposed poimt.
To create what is needed:
```
bash nfs_install.sh
```
After the tests you can remove:
```bash nfs_delete.sh
```


# Script to test as a no admin user
The script test-jws_src_test_resources_operators_jws_update-yaml-files.sh and the doc here  allow to test the operator as a "normal" user instead "admin".

## set OPERATOR_IMAGE and OPERATOR_BUNDLE_IMAGE
```
export OPERATOR_IMAGE=registry.redhat.io/jboss-webserver-5/webserver-openjdk8-rhel8-operator:1.0-18
export OPERATOR_BUNDLE_IMAGE=registry.redhat.io/jboss-webserver-5/webserver-openjdk8-operator-bundle:1.0.0-5
```

## run the script
```
bash test-jws_src_test_resources_operators_jws_update-yaml-files.sh
```
it should be able to guess the OPERATOR_IMAGE from OPERATOR_BUNDLE_IMAGE, but to be on safe side export the 2 variables.

## create project

oc new-project jws-operator || oc project jws-operator

## install operator pieces in deploy
oc apply -f configmap-jws-operator.gen.yaml

oc apply -f operator-group.yaml

oc apply -f operator-subscription.yaml

## check the result (it takes ~ 1 minutes) for example
```
[jfclere@localhost deploy]$ oc get pods
NAME                            READY     STATUS    RESTARTS   AGE
jws-operator-849d8d7bcb-v5fjr   1/1       Running   0          34s
jws-operator-zkspr              1/1       Running   0          56s
```
and
```
[jfclere@localhost deploy]$ oc describe csv jws-operator.v1.0.0 | grep containerImage
              containerImage=registry.redhat.io/jboss-webserver-5/webserver-openjdk8-rhel8-operator:1.0-18
```

## give the permissions to ${TEST_USER}

oc adm policy add-role-to-user basic-user ${TEST_USER}

#create the user

htpasswd -c /home/jfclere/TMP/htpasswd.txt ${TEST_USER}

then use the console to finish the user creation...

#give permissions

oc create clusterrole get-customresourcedefinitions-jwsservers-clusterrole --verb=get --resource=customresourcedefinitions --resource-name=webservers.web.servers.org

oc adm policy add-cluster-role-to-user get-customresourcedefinitions-jwsservers-clusterrole ${TEST_USER}

oc create clusterrole create-customresourcedefinitions-clusterrole --verb=create --resource=customresourcedefinitions

oc adm policy add-cluster-role-to-user create-customresourcedefinitions-clusterrole ${TEST_USER}

oc create clusterrole create-jwsservers-clusterrole --verb=create --resource=webservers

oc adm policy add-cluster-role-to-user create-jwsservers-clusterrole ${TEST_USER}
