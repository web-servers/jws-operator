# create project

oc new-project jws-operator || oc project jws-operator

# install operator pieces
oc apply -f configmap-jws-operator.gen.yaml

oc apply -f operator-group.yaml

oc apply -f operator-subscription.yaml

# check the result (it takes ~ 1 minutes) for example
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

# give the permissions to ${TEST_USER}

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

