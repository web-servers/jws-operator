kubectl delete sa/nfs-storage-sa
kubectl delete deployment/nfs-client-provisioner
kubectl delete deployment/my-nfs
kubectl delete sc/nfs-client
kubectl delete pv/pv0000
kubectl delete pv/pv0001
kubectl delete services/nfs-service0
