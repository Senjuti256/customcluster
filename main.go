package main

import (
	"flag"
	"log"
	"context"
    "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
var (
 kuberconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)


func main() {
 flag.Parse()


 cfg, err := clientcmd.BuildConfigFromFlags("", *kuberconfig)
 if err != nil {
	log.Fatalf("Error building kubeconfig: %v", err)
 }


 clientset, err := kubernetes.NewForConfig(cfg)
 if err != nil {
    log.Fatalf("Error building example clientset: %v", err)
 }
 namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to list namespaces: %v", err)
	}

	log.Printf("Number of namespaces: %d", len(namespaces.Items))
}
