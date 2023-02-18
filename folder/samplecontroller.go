package controller

import (
	"context"
	"fmt"
	"log"

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
	//"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	V1alpha1 "github.com/Senjuti256/customcluster/pkg/apis/sde.dev/v1alpha1"
	clientset "github.com/Senjuti256/customcluster/pkg/client/clientset/versioned"
	cInformer "github.com/Senjuti256/customcluster/pkg/client/informers/externalversions/sde.dev/v1alpha1"
	cLister "github.com/Senjuti256/customcluster/pkg/client/listers/sde.dev/v1alpha1"
)

const controllerAgentName = "controller"

type controller struct {
    kubeclient    kubernetes.Interface
    customclient   clientset.Interface
    customInformer    cache.SharedIndexInformer
    workqueue         workqueue.RateLimitingInterface
    informer          cache.Controller
    recorder          record.EventRecorder
    // - resource (informer) cache has synced
	cpodSync cache.InformerSynced
	// - interface provided by informer
	cpodlister cLister.CustomclusterLister
	// - queue
	// stores the work that has to be processed, instead of performing
	// as soon as it's changed.
	// Helps to ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	//wq workqueue.RateLimitingInterface
}

/*func NewController(kubeconfig string, resyncPeriod time.Duration) (*controller, error) {
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
*/

func NewController(kubeClient kubernetes.Interface, customClient clientset.Interface, cpodInformer cInformer.CustomclusterInformer) *controller {
	c := &controller{
		kubeclient:   kubeClient,
		customclient: customClient,
		cpodSync:     cpodInformer.Informer().HasSynced,
		cpodlister:   cpodInformer.Lister(),
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Customcluster"),
	}

	// event handler when the customcluster resources are added/deleted/updated.
	cpodInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.handleAdd,
			UpdateFunc: func(old, obj interface{}) {
				oldcpod := obj.(*V1alpha1.Customcluster)
				newcpod := obj.(*V1alpha1.Customcluster)
				if newcpod == oldcpod {
					return
				}
				c.handleAdd(obj)
			},
			DeleteFunc: c.handleDel,
		},
    )
    return c
}
// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.

func (c *controller) Run(ch <-chan struct{}) error {

    defer c.workqueue.ShutDown()

    // Start the informer factories to begin populating the informer caches
	klog.Info("Starting the Customcluster controller")

	if ok := cache.WaitForCacheSync(ch, c.cpodSync); !ok {
		log.Println("cache was not sycned")}

    klog.Info("Starting workers")
	go wait.Until(c.runWorker, time.Second, ch)

	<-ch
    klog.Info("Shutting down the worker")
	return nil
}

func (c *controller) runWorker() {
    for c.processNextItem() {
    }
}

func (c *controller) processNextItem() bool {
    /*item, quit := c.workqueue.Get()
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
    return true*/
    obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			//utilruntime.HandleError(
                //fmt.Errorf("expected string in workqueue but got %#v", obj)             //)
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		//utilruntime.HandleError(err)
        fmt.Println("Error occured =",err.Error())
		return true
	}

	return true

}

func (c *controller) syncHandler(key string) error {
    namespace, name, err := cache.SplitMetaNamespaceKey(key)
    if err != nil {
        return fmt.Errorf("failed to split key into namespace and name: %v", err)
    }
    
    //filterout if pods are available or not
    custom, err := c.customclient.SamplecontrollerV1alpha1().Customclusters(namespace).Get(context.Background(),name, metav1.GetOptions{            
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
    //labelSelector := fmt.Sprintf("customcluster=%s", name)

    pods, err := c.kubeclient.CoreV1().Pods(namespace).List(context.TODO(),metav1.ListOptions{})             
    if err != nil {
        return fmt.Errorf("failed to list pods: %v", err)
    }

    currentCount := len(pods.Items)
    fmt.Print("Current number of pods in cluster = ", currentCount)
    
    if currentCount < count {
        cnt:=count-currentCount
       // podName := fmt.Sprintf("%s", name)
        //err :=
         c.createPods(custom,cnt);                                                       
            /*if err != nil {
                return fmt.Errorf("failed to create pod %s/%s: %v", namespace, podName, err)}*/
    } else if currentCount > count {
            cnt := currentCount-count
            //err := 
            c.deletePods(custom,cnt);                                                        
            /*if err != nil {
                panic(err)
            }*/
    }

    return nil
}

func (c *controller) createPods(custom *V1alpha1.Customcluster,cnt int) {   //error{
    if cnt > 0 {
        for i := 1; i <= cnt; i++ {
            pod := &v1.Pod{}
            pod, err := c.kubeclient.CoreV1().Pods(metav1.NamespaceDefault).Create(context.Background(), pod, metav1.CreateOptions{})
            if err != nil {
                panic(err)
            }
            fmt.Printf("Created pod %q for CRD %q with message: %q\n", pod.Name, custom.Name, custom.Spec.Message)
        }
    }
    //return nil
}

func (c *controller) updatePods(oldCustom *V1alpha1.Customcluster, newCustom *V1alpha1.Customcluster,cnt int) {    //error {
    if oldCustom.Spec.Count != newCustom.Spec.Count || oldCustom.Spec.Message != newCustom.Spec.Message {
        c.deletePods(oldCustom,cnt)
        c.createPods(newCustom,cnt)
    }
    //return nil
}

func (c *controller) deletePods(custom *V1alpha1.Customcluster,cnt int) {   //error{
    pods, err := c.kubeclient.CoreV1().Pods(metav1.NamespaceDefault).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", custom.Name)})
    if err != nil {
        panic(err)
    }
    for _, pod := range pods.Items{
        if cnt>0{
        err = c.kubeclient.CoreV1().Pods(metav1.NamespaceDefault).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})}
        if err != nil {
            if !errors.IsNotFound(err) {
                panic(err)
            }
        } else {
            fmt.Printf("Deleted pod %q for CRD %q\n", pod.Name, custom.Name)
            cnt--;
        }
    }
    //return nil
}

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
        c.updatePods(oldCustom,newCustom,cnt)
    }
    c.handleObject(newObj)
}

func (c *controller) handleDel(obj interface{}) {
    key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
    if err != nil {
        runtime.HandleError(fmt.Errorf("failed to get key for object %+v: %v", obj, err))
        return
    }
    c.workqueue.Add(key)
}
