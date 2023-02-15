package main

import (
	"fmt"
	//"os"
	//"os/signal"
	//"syscall"
	"time"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	//"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/Senjuti256/customcluster/pkg/client/clientset/versioned"
	informers "github.com/Senjuti256/customcluster/pkg/client/informers/externalversions"
	//v1alpha1 "github.com/Senjuti256/customcluster/pkg/apis/sde.dev/v1alpha1"
)

const controllerAgentName = "customcontroller"

type controller struct {
    kubeClientset     kubernetes.Interface
    customClientset   clientset.Interface
    customInformer    cache.SharedIndexInformer
    workqueue         workqueue.RateLimitingInterface
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
    }

    customInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    controller.handleAdd,
        UpdateFunc: controller.handleUpdate,
        DeleteFunc: controller.handleDelete,
    })

    return controller, nil
}

func (c *controller) Run(stopCh <-chan struct{}) {
    defer c.workqueue.ShutDown()

    go c.customInformer.Run(stopCh)

    if !cache.WaitForCacheSync(stopCh, c.customInformer.HasSynced) {
        runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
        return
    }

    go c.runWorker()

    <-stopCh
}

func (c *controller) runWorker() {
    for c.processNextItem() {
    }
}

func (c *controller) processNextItem() bool {
    key, quit := c.workqueue.Get()
    if quit {
        return false
    }
    defer c.workqueue.Done(key)

    err := c.syncHandler(key.(string))
    if err != nil {
        c.workqueue.AddRateLimited(key)
        runtime.HandleError(fmt.Errorf("failed to process item with key %q: %v", key, err))
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

    custom, err := c.customClientset.controller().CustomClusters(namespace).Get(name, metav1.GetOptions{})            //*
    if err != nil {
        if errors.IsNotFound(err) {
            glog.Infof("CustomCluster %s/%s has been deleted", namespace, name)
            return nil
        }
        return fmt.Errorf("failed to retrieve CustomCluster %s/%s: %v", namespace, name, err)
    }

    count := custom.Spec.Count
    message := custom.Spec.Message

    labelSelector := fmt.Sprintf("customcluster=%s", name)

    pods, err := c.kubeClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})             //*
    if err != nil {
        return fmt.Errorf("failed to list pods: %v", err)
    }

    currentCount := len(pods.Items)

    if currentCount < count {
        for i := currentCount; i < count; i++ {
            podName := fmt.Sprintf("%s-%d", name, i)
            if err := c.createPod(namespace, podName, message); err != nil {
                return fmt.Errorf("failed to create pod %s/%s: %v", namespace, podName, err)
            }
        }
    } else if currentCount > count {
        for i := currentCount - 1; i >= count; i-- {
            pod := &pods.Items[i]
            if err := c.deletePod(pod); err != nil {
                return fmt.Errorf("failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
            }
        }
    }

    return nil
}

func (c *controller) createPod(namespace, podName, message string) error {
    labels := map[string]string{"customcluster": podName}

    pod := &v1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: namespace,
            Name:      podName,
            Labels:    labels,
        },
        Spec: v1.PodSpec{
            Containers: []v1.Container{
                {
                    Name:  "main",
                    Image: "busybox",
                    Args:  []string{"sh", "-c", fmt.Sprintf("echo %s; sleep 3600", message)},
                },
            },
        },
    }

    _, err := c.kubeClientset.CoreV1().Pods(namespace).Create(pod,metav1.CreateOptions{
    	TypeMeta:        metav1.TypeMeta{},
    	DryRun:          []string{},
    	FieldManager:    "",
    	FieldValidation: "",
    })                                                 //*
    if err != nil {
        return err
    }

    return nil
}

func (c *controller) deletePod(pod *v1.Pod) error {
    err := c.kubeClientset.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})   //*
    if err != nil && !errors.IsNotFound(err) {
    return err
    }

    return nil
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
    oldCustom := oldObj.(*v1alpha1.CustomCluster)                         //*
    newCustom := newObj.(*v1alpha1.CustomCluster)                         //*
    if oldCustom.ResourceVersion == newCustom.ResourceVersion {
        // Periodic resync will send update events for all known CustomClusters.
        // Two different versions of the same CustomCluster will always have different RVs.
        return
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

func (c *controller) run(stopCh <-chan struct{}) {
    defer c.workqueue.ShutDown()

    glog.Info("Starting CustomCluster controller")

    if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {                    //*
        runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
        return
    }

    glog.Info("CustomCluster controller synced and ready")

    // infinite loop until stopCh is closed
    wait.Until(c.runWorker, time.Second, stopCh)
}
