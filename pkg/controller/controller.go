package controller

import (
	"context"
	"fmt"

	//"os"
	//"os/signal"
	//"syscall"
	"time"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	//"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	//"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	V1alpha1 "github.com/Senjuti256/customcluster/pkg/apis/sde.dev/v1alpha1"
	clientset "github.com/Senjuti256/customcluster/pkg/client/clientset/versioned"
	informers "github.com/Senjuti256/customcluster/pkg/client/informers/externalversions"
)

const controllerAgentName = "controller"

type controller struct {
    kubeClientset     kubernetes.Interface
    customClientset   clientset.Interface
    customInformer    cache.SharedIndexInformer
    workqueue         workqueue.RateLimitingInterface
    informer          cache.Controller
    recorder          record.EventRecorder
   // createPods        metav1.CreateOptions
    //deletePods        metav1.DeleteOptions
}

func newController(kubeconfig string, resyncPeriod time.Duration) (*controller, error) {
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil {
        return nil, fmt.Errorf("failed to build config from kubeconfig: %v", err)
    }

    kubeClientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create kube clientset: %v", err)
    }

    customClientset, err := clientset.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create custom clientset: %v", err)
    }

    customInformerFactory := informers.NewSharedInformerFactory(customClientset, resyncPeriod)
    customInformer := customInformerFactory.Samplecontroller().V1alpha1().Customclusters().Informer()
    workqueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CustomClusters")

    
    controller := &controller{
    	kubeClientset:   kubeClientset,
    	customClientset: customClientset,
    	customInformer:  customInformer,
    	workqueue:       workqueue,
        //recorder:          recorder,
    }

    klog.Info("Setting up event handlers")

    customInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    controller.handleAdd,
        UpdateFunc: controller.handleUpdate,
        DeleteFunc: controller.handleDelete,
    })

    return controller, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.

func (c *controller) Run(stopCh <-chan struct{}) {
    defer c.workqueue.ShutDown()
    
    // Start the informer factories to begin populating the informer caches
	klog.Info("Starting the customcluster controller")
    
    // Wait for the caches to be synced before starting workers
    if !cache.WaitForCacheSync(stopCh, c.customInformer.HasSynced) {
        runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
        return
    }
    go c.customInformer.Run(stopCh)

    // Launch the goroutine for workers to process the CR
	klog.Info("Starting workers")
    go c.runWorker()
    
    go wait.Until(c.runWorker, time.Second, stopCh)
	klog.Info("Started workers")

    <-stopCh
    
    klog.Info("Shutting down the worker")

}

func (c *controller) runWorker() {
    for c.processNextItem() {
    }
}

func (c *controller) processNextItem() bool {
    item, quit := c.workqueue.Get()
    if quit {
        return false
    }
    defer c.workqueue.Done(item)

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Errorf("error while calling Namespace Key func on cache for item %s: %s", key, err.Error())
		return false
	}


    error := c.syncHandler(item.(string))
    if error != nil {
        c.workqueue.AddRateLimited(item)
        runtime.HandleError(fmt.Errorf("failed to process item with key %q: %v", item, error))
        return true
    }

    c.workqueue.Forget(key)
    return true
}

func (c *controller) syncHandler(key string) error {
    namespace, name, err := cache.SplitMetaNamespaceKey(key)
    if err != nil {
        return fmt.Errorf("failed to split key into namespace and name: %v", err)
    }
    
    //filterout if pods are available or not
    custom, err := c.customClientset.SamplecontrollerV1alpha1().Customclusters(namespace).Get(context.Background(),name, metav1.GetOptions{            
    	TypeMeta:        metav1.TypeMeta{},
    	ResourceVersion: "",
    })            
    if err != nil {
        if errors.IsNotFound(err) {
            glog.Infof("CustomCluster %s/%s has been deleted", namespace, name)
            return nil
        }
        return fmt.Errorf("failed to retrieve CustomCluster %s/%s: %v", namespace, name, err)
    }

    count := custom.Spec.Count
    //message := custom.Spec.Message
    labelSelector := fmt.Sprintf("customcluster=%s", name)

    pods, err := c.kubeClientset.CoreV1().Pods(namespace).List(context.TODO(),metav1.ListOptions{
    	TypeMeta:             metav1.TypeMeta{},
    	LabelSelector:        labelSelector,
    	FieldSelector:        "",
    	Watch:                false,
    	AllowWatchBookmarks:  false,
    	ResourceVersion:      "",
    	ResourceVersionMatch: "",
    	TimeoutSeconds:       new(int64),
    	Limit:                0,
    	Continue:             "",
    })             
    if err != nil {
        return fmt.Errorf("failed to list pods: %v", err)
    }

    currentCount := len(pods.Items)
    fmt.Println("Current number of pods in cluster %v", currentCount)
    
    if currentCount < count {
        cnt:=count-currentCount
        //for i := currentCount; i < count; i++ {
            podName := fmt.Sprintf("%s", name)
            if err := c.createPods(c,&kubernetes.Clientset{},custom,cnt);                                                       //*
            err != nil {
                return fmt.Errorf("failed to create pod %s/%s: %v", namespace, podName, err)
            //}
        }
    } else if currentCount > count {
        //for i := currentCount - 1; i >= count; i-- {
            cnt := currentCount-count
            //pod := &pods.Items
            if err := c.deletePods(c,&kubernetes.Clientset{},custom,cnt);                                                        //*
            err != nil {
                //return fmt.Errorf("failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
                panic(err)
            }
        //}
    }

    return nil
}

func createPods(c *controller,clientset *kubernetes.Clientset,custom *V1alpha1.Customcluster,cnt int) {
    if cnt > 0 {
        for i := 1; i <= cnt; i++ {
            pod := &v1.Pod{}
            pod, err := clientset.CoreV1().Pods(metav1.NamespaceDefault).Create(context.Background(), pod, metav1.CreateOptions{})
            if err != nil {
                panic(err)
            }
            fmt.Printf("Created pod %q for CRD %q with message: %q\n", pod.Name, custom.Name, custom.Spec.Message)
        }
    }
}

func updatePods(c *controller,clientset *kubernetes.Clientset, oldCustom *V1alpha1.Customcluster, newCustom *V1alpha1.Customcluster,cnt int) {
    if oldCustom.Spec.Count != newCustom.Spec.Count || oldCustom.Spec.Message != newCustom.Spec.Message {
        deletePods(c,clientset, oldCustom,cnt)
        createPods(c,clientset, newCustom,cnt)
    }
}

func deletePods(c *controller,clientset *kubernetes.Clientset, custom *V1alpha1.Customcluster,cnt int) {
    pods, err := clientset.CoreV1().Pods(metav1.NamespaceDefault).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", custom.Name)})
    if err != nil {
        panic(err)
    }
    for _, pod := range pods.Items{
        if cnt>0{
        err = clientset.CoreV1().Pods(metav1.NamespaceDefault).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
        cnt--
        }
        if err != nil {
            if !errors.IsNotFound(err) {
                panic(err)
            }
        } else {
            fmt.Printf("Deleted pod %q for CRD %q\n", pod.Name, custom.Name)
        }
    }
}

/*func (c *controller) deletePod(pod *v1.Pod) error {
    err := c.kubeClientset.CoreV1().Pods(pod.Namespace).Delete(context.TODO(),pod.Name,metav1.DeleteOptions{})   
    if err != nil && !errors.IsNotFound(err) {
    return err
    }

    return nil
}
*/

func (c *controller) handleObject(obj interface{}) {
    key, err := cache.MetaNamespaceKeyFunc(obj)
    if err != nil {
        runtime.HandleError(fmt.Errorf("failed to get key for object %+v: %v", obj, err))
        return
    }
    c.workqueue.Add(key)
}

func (c *controller) handleAdd(obj interface{}) {
    c.handleObject(obj)
}

func (c *controller) handleUpdate(oldObj, newObj interface{}) {
    oldCustom := oldObj.(*V1alpha1.Customcluster)                         
    newCustom := newObj.(*V1alpha1.Customcluster)                         
    if oldCustom.Spec.Count != newCustom.Spec.Count || oldCustom.Spec.Message != newCustom.Spec.Message {
        // Periodic resync will send update events for all known CustomClusters.
        // Two different versions of the same CustomCluster will always have different RVs.
        cnt :=newCustom.Spec.Count-oldCustom.Spec.Count
        updatePods(c,&kubernetes.Clientset{},oldCustom,newCustom,cnt)
    }
    c.handleObject(newObj)
}

func (c *controller) handleDelete(obj interface{}) {
    key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
    if err != nil {
        runtime.HandleError(fmt.Errorf("failed to get key for object %+v: %v", obj, err))
        return
    }
    c.workqueue.Add(key)
}
