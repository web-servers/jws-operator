#!/usr/bin/env bash

kubectl create -f deploy/crds/jwsservers.web.servers.org_v1alpha1_jbosswebserver_crd.yaml
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
kubectl create -f deploy/kubernetes_operator.yaml
