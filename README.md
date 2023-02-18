KIND_EXPERIMENTAL_PROVIDER=podman
minikube config set rootless true
minikube start --driver=podman --container-runtime=containerd

kubectl apply -f manifests/crdefinition.yaml
kubectl apply -f manifests/cr.yaml
kubectl apply -f my-nginx-pod.yaml

kubectl api-resources | grep customclusters
kubectl get crds

go build -o main .
./main.go
