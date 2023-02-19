package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"

	klient "github.com/Senjuti256/customcluster/pkg/client/clientset/versioned"
	kInfFac "github.com/Senjuti256/customcluster/pkg/client/informers/externalversions"
	"github.com/Senjuti256/customcluster/pkg/controller"
)

func main() {
	klog.InitFlags(nil)
	var kubeconfig *string

	klog.Info("Searching for kubeConfig")

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	klog.Info("Building config from the kubeConfig")
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Printf("Building config from flags failed, %s, trying to build inclusterconfig", err.Error())
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Printf("error %s building inclusterconfig", err.Error())
		}
	}
     
	klog.Info("getting the custom clientset")
	klientset, err := klient.NewForConfig(config)
	if err != nil {
		log.Printf("getting klient set %s\n", err.Error())
	}
    
	klog.Info("getting the k8s client")
	kubeclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("getting std client %s\n", err.Error())
	}
    
	infoFactory:= kInfFac.NewSharedInformerFactory(klientset, 20*time.Second)
	ch := make(chan struct{})
	c := controller.NewController(kubeclient, klientset, infoFactory.Samplecontroller().V1alpha1().Customclusters())             //*
	//c,err:= controller.NewController(klientset,10*time.Minute)
	
    klog.Info("Starting channel and Run mthod of controller")
	infoFactory.Start(ch)
	if err := c.Run(ch); err != nil {
		log.Printf("error running controller %s\n", err.Error())
	}
}

/*func NewController(kubeclient *kubernetes.Clientset, klientset *klient.Clientset, customclusterInformer v1alpha1.CustomclusterInformer) {
	panic("unimplemented")
}*/