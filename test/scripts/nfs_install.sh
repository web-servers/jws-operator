oc create sa nfs-storage-sa
oc adm policy add-scc-to-user privileged -z nfs-storage-sa
oc create -f nfs.yaml
ip=`kubectl get services/nfs-service0 --output=jsonpath={..spec.clusterIP}`
echo $ip
sed s:1.2.3.4:$ip:g nfs-client-provisioner-tmp.yaml > nfs-client-provisioner.yaml
sed s:1.2.3.4:$ip:g nfs-pv-tmp.yaml > nfs-pv.yaml
sed s:1.2.3.4:$ip:g nfs-pv1-tmp.yaml > nfs-pv1.yaml
oc create -f nfs-client-provisioner.yaml
oc create -f nfs-client-sc.yaml
oc create -f nfs-pv.yaml
oc create -f nfs-pv1.yaml
