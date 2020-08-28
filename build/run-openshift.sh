#!/usr/bin/env bash

oc create -f deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_crd.yaml
oc create -f deploy/service_account.yaml
oc create -f deploy/role.yaml
oc create -f deploy/role_binding.yaml
oc apply -f deploy/operator.yaml