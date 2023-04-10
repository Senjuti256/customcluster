Steps to run the controller :=

First I started a cluster in my local machine. 
               
               ``` KIND_EXPERIMENTAL_PROVIDER=podman ```
               ``` minikube config set rootless true ```
               ``` minikube start - -driver=podman - -container-runtime=containerd ```
               

Once the cluster is set up and is running fine : 
            
               ``` kubectl apply -f manifests/crdefinition.yaml ```
               
               ``` kubectl apply -f manifests/cr.yaml ```
               

 As there were no pods in my default namespace so I created my-nginx-pod in the default namespace.

To check whether the custom resource was actually created or not 
                ``` kubectl  get crds ```
                ``` kubectl api-resources | grep customclusters ```

Next step is to build and run the application.
                ``` go build -o main . ```
                ``` ./main ```
