set -e
# The goal here is to create YAML files needed for installing Operator on cluster
# For this we need:
# - CatalogSource with CRD and CSV of the operator as a ConfigMap
# - OperatorGroup
# - Subscription
#
# To get CRD and CSV for operator we need to extract it from associated bundle image.
#
# References:
# https://docs.openshift.com/container-platform/4.5/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-operator-from-operatorhub-using-cli_olm-adding-operators-to-a-cluster

indent() {
  sed "s/^/      /" | sed "s/^      \($1\)/    - \1/"
}

OPERATOR_IMAGE=registry.redhat.io/jboss-webserver-5/webserver-openjdk8-rhel8-operator:1.0-18
OPERATOR_BUNDLE_IMAGE=registry.redhat.io/jboss-webserver-5/webserver-openjdk8-operator-bundle:1.0.0-5

echo "Generating YAML files for operator ${OPERATOR_IMAGE}"
echo "                 from bundle image ${OPERATOR_BUNDLE_IMAGE}"

ORIG_PWD=$(pwd)
DEPLOY_DIR=$ORIG_PWD/deploy

pushd $(mktemp -d)

# extract CRD and CSV from bundle image to ./manifests dir
podman pull ${OPERATOR_BUNDLE_IMAGE}
podman image save ${OPERATOR_BUNDLE_IMAGE} > bundle.tar
tar -xvf bundle.tar
cat manifest.json  | jq  -cr '.[0].Layers | .[]' | xargs -n1 tar -xvf

mkdir -p deploy

## take only the latest CRD and CSV from the manifests (there should be only 1 for each but try to be safe...)
CRD_FILE=$(find manifests -name '*_crd.yaml' | sort -r | head -1)
CSV_FILE=$(find manifests -name '*clusterserviceversion.yaml' | sort -r | head -1)
CURRENT_CSV=$(grep name $CSV_FILE | head -1 | cut -f2 -d: | tr -d " ")

echo "CRD_FILE: ${CRD_FILE}"
echo "CSV_FILE: ${CSV_FILE}"
echo "CURRENT_CSV: ${CURRENT_CSV}"

CRD=$(cat $CRD_FILE | grep -v -- "---" | indent apiVersion)
CSV=$(cat $CSV_FILE | indent apiVersion)

cat <<EOF | sed 's/^  *$//' > deploy/configmap-jws-operator.gen.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jws-operator

data:
  customResourceDefinitions: |-
$CRD
  clusterServiceVersions: |-
$CSV
  packages: |-
    - packageName: jws
      channels:
      - name: alpha
        currentCSV: $CURRENT_CSV
---
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: jws-operator
spec:
  configMap: jws-operator
  displayName: JWS Operator (QE)
  publisher: Red Hat
  sourceType: internal
EOF

# point metadata to actual location of the tested operator
sedOrigImage=$(grep -Po '.*containerImage: \K.*[^ ]*' manifests/jws-operator.clusterserviceversion.yaml)
echo "sedOrigImage: ${sedOrigImage}"
hash=$(echo ${sedOrigImage} | awk -F @ ' { print $2 } ')
echo "hash: ${hash}"
base=$(dirname ${OPERATOR_BUNDLE_IMAGE})
echo "base: ${base}"
component=$(grep "com.redhat.component" ./root/buildinfo/Dockerfile* | awk -F \" ' {  print $2 } ' | sed s:bundle-::)
echo "component: ${component}"
if [ -z ${OPERATOR_IMAGE} ]; then
  docker pull ${base}/${component}@${hash}
  if [ $? -ne 0 ]; then
    echo "Can't guess OPERATOR_IMAGE: ${OPERATOR_IMAGE} guessed wrong!!!"
    echo "try to export OPERATOR_IMAGE and restart"
    exit 1
  fi
  sedTargetImage=${base}/${component}@${hash}
else
  sedTargetImage=${OPERATOR_IMAGE}
fi
sed -i 's|'$sedOrigImage'|'$sedTargetImage'|g' deploy/configmap-jws-operator.gen.yaml

cat << EOF > deploy/operator-group.yaml
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: example
  namespace: jws-operator
spec:
  targetNamespaces:
  - jws-operator
EOF

cat << EOF > deploy/operator-subscription.yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: jws-operator
  generateName: jws-
  namespace: jws-operator
spec:
  source: jws-operator
  sourceNamespace: jws-operator
  name: jws
  startingCSV: $CURRENT_CSV
  channel: alpha
EOF

cp deploy/* $DEPLOY_DIR
popd
