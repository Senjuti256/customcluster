package controller

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	v1alpha1 "github.com/Senjuti256/customcluster/pkg/apis/sde.dev/v1alpha1"
	cClientSet "github.com/Senjuti256/customcluster/pkg/client/clientset/versioned"
	cInformer "github.com/Senjuti256/customcluster/pkg/client/informers/externalversions/sde.dev/v1alpha1"
	cLister "github.com/Senjuti256/customcluster/pkg/client/listers/sde.dev/v1alpha1"
	"github.com/kanisterio/kanister/pkg/poll"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by controller"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Customcluster synced successfully"
)

// Controller implementation for Customcluster resources
type Controller struct {
	// K8s clientset
	kubeClient kubernetes.Interface
	// things required for controller:
	// - clientset for interacting with custom resources
	cpodClient cClientSet.Interface
	// - resource (informer) cache has synced
	cpodSync cache.InformerSynced
	// - interface provided by informer
	cpodlister cLister.CustomclusterLister
	// - queue
	// stores the work that has to be processed, instead of performing
	// as soon as it's changed.This reduces overhead to the API server through repeated querying for updates on CR
	// Helps to ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	wq workqueue.RateLimitingInterface
}

// returns a new customcluster controller
func NewController(kubeClient kubernetes.Interface, cpodClient cClientSet.Interface, cpodInformer cInformer.CustomclusterInformer) *Controller {
	klog.Info("NewController is called\n")
	c := &Controller{
		kubeClient: kubeClient,
		cpodClient: cpodClient,
		cpodSync:   cpodInformer.Informer().HasSynced,
		cpodlister: cpodInformer.Lister(),
		wq:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Customcluster"),
	}
    klog.Info("NewController made\n")
    klog.Info("Setting up event handlers")
	// event handler when the custom resources are added/deleted/updated.
	cpodInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.handleAdd,
			UpdateFunc: func(old, new interface{}) {
				oldcpod := old.(*v1alpha1.Customcluster)
				newcpod := new.(*v1alpha1.Customcluster)
				if newcpod == oldcpod {
					return
				}
				c.handleAdd(new)
			},
			DeleteFunc: c.handleDel,
		},
	)
    klog.Info("Returning controller object \n")
	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until ch
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(ch chan struct{}) error {
	defer c.wq.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting the Customcluster controller\n")
	klog.Info("Waiting for informer caches to sync\n")

	// Wait for the caches to be synced before starting workers
	if ok := cache.WaitForCacheSync(ch, c.cpodSync); !ok {
		log.Println("failed to wait for cache to sync")
	}
	// Launch the goroutine for workers to process the CR
	klog.Info("Starting workers\n")
	go wait.Until(c.worker, time.Second, ch)
	klog.Info("Started workers\n")
	<-ch
	klog.Info("Shutting down the worker")

	return nil
}

// worker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue
func (c *Controller) worker() {
	for c.processNextItem() {
	}
}

// processNextWorkItem will read a single work item existing in the workqueue and
// attempt to process it, by calling the syncHandler.
/*
func (c *Controller) processNextItem() bool {
	klog.Info("Inside processNextItem method")
	item, shutdown := c.wq.Get()
	if shutdown {
		klog.Info("Shutting down")
		return false
	}

	defer c.wq.Forget(item)
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Errorf("error while calling Namespace Key func on cache for item %s: %s", item, err.Error())
		return false
	}
    
	klog.Info("Trying to get namespace and name")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("error while splitting key into namespace & name: %s", err.Error())
		return false
	}

	cpod, err := c.cpodlister.Customclusters(ns).Get(name)
	klog.Info("cpod is",cpod.Name)
	klog.Info("cpod is",cpod.Namespace)
	klog.Info("cpod is",cpod.Spec)
	klog.Info("cpod is",cpod.Spec.Count)
	c1,err:=c.cpodClient.SamplecontrollerV1alpha1().Customclusters(ns).Get(context.TODO(),name,metav1.GetOptions{})
	klog.Info(c1.Spec.Count)
	if err != nil {
		klog.Errorf("error %s, Getting the cpod resource from lister.", err.Error())
		return false
	}

	// filter out if required pods are already available or not:
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"controller": cpod.Name,
		},
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	pList, _ := c.kubeClient.CoreV1().Pods(cpod.Namespace).List(context.TODO(), listOptions)
    klog.Info("Count ",cpod.Spec.Count)
	if err := c.syncHandler(cpod, pList); err != nil {
		klog.Errorf("Error while syncing the current vs desired state for the resource of kind Customcluster %v: %v\n", cpod.Name, err.Error())
		return false
	}

	// wait for pods to be ready
	err = c.waitForPods(cpod, pList)
	if err != nil {
		klog.Errorf("error %s, waiting for pods to meet the expected state", err.Error())
	}

    fmt.Println("Calling update status again!!")
	err = c.updateStatus(cpod, cpod.Spec.Message, pList)
	if err != nil {
		klog.Errorf("error %s updating status after waiting for Pods", err.Error())              //**error
	}

	return true
}
*/

func (c *Controller) processNextItem() bool {
	klog.Info("Inside processNextItem method")
	item, shutdown := c.wq.Get()
	if shutdown {
		klog.Info("Shutting down")
		return false
	}

	defer c.wq.Forget(item)

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Errorf("error while calling Namespace Key func on cache for item %s: %s", item, err.Error())
		return false
	}
    klog.Info("Trying to get namespace and name")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("error while splitting key into namespace & name: %s", err.Error())
		return false
	}

	foo, err := c.cpodlister.Customclusters(ns).Get(name)
	klog.Info("cpod is ",foo.Name)
	klog.Info("cpod is ",foo.Namespace)
	//klog.Info("cpod is",foo.Spec)
	klog.Info("cpod is ",foo.Spec.Count)
	klog.Info("cpod is ",foo.Spec.Message)
	if err != nil {
		klog.Errorf("error %s, Getting the foo resource from lister.", err.Error())
		return false
	}

	// filter out if required pods are already available or not:
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"controller": foo.Name,
		},
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	// TODO: Prefer using podLister to reduce the call to K8s API.
	podsList, _ := c.kubeClient.CoreV1().Pods(foo.Namespace).List(context.TODO(), listOptions)

	if err := c.syncHandler(foo, podsList); err != nil {
		klog.Errorf("Error while syncing the current vs desired state for TrackPod %v: %v\n", foo.Name, err.Error())
		return false
	}
    // wait for pods to be ready
	err = c.waitForPods(foo, podsList)
	if err != nil {
		klog.Errorf("error %s, waiting for pods to meet the expected state", err.Error())
	}

    fmt.Println("Calling update status again!!")
	err = c.updateStatus(foo, foo.Spec.Message, podsList)
	if err != nil {
		klog.Errorf("error %s updating status after waiting for Pods", err.Error())              //**error
	}

	return true
}

// total number of 'Running' pods
func (c *Controller) totalRunningPods(cpod *v1alpha1.Customcluster) int {
	labelSelector := metav1.LabelSelector{
		MatchLabels:      map[string]string{"controller": cpod.Name},
		MatchExpressions: []metav1.LabelSelectorRequirement{},                               //*
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}

	pList, _ := c.kubeClient.CoreV1().Pods(cpod.Namespace).List(context.TODO(), listOptions)

	runningPods := 0
	for _, pod := range pList.Items {
		if pod.ObjectMeta.DeletionTimestamp.IsZero() && pod.Status.Phase == "Running" {
			runningPods++
		}
	}
	return runningPods
}

// syncHandler monitors the current state & if current != desired,
// tries to meet the desired state.
func (c *Controller) syncHandler(cpod *v1alpha1.Customcluster, pList *corev1.PodList) error {
	var Createpod, Deletepod bool
	itr := cpod.Spec.Count
	deleteItr := 0
	runningPods := c.totalRunningPods(cpod)
	klog.Info("Total pods running ",runningPods)
	klog.Info("Count of pods ",cpod.Spec.Count)

	if runningPods != cpod.Spec.Count || cpod.Spec.Message != cpod.Status.Message {
		if runningPods > 0 && cpod.Spec.Message != cpod.Status.Message {
			klog.Warningf("the message of Customcluster %v resource has been modified, recreating the pods\n", cpod.Name)
			Deletepod = true
			Createpod = true
			itr = cpod.Spec.Count
			deleteItr = runningPods
		} else {
			klog.Warningf("detected mismatch of replica count for CR %v >> expected: %v & have: %v\n\n", cpod.Name, cpod.Spec.Count, runningPods)
			if runningPods < cpod.Spec.Count {
				Createpod = true
				itr = cpod.Spec.Count - runningPods
				klog.Infof("Creating %v new pods\n", itr)
			} else if runningPods > cpod.Spec.Count {
				Deletepod = true
				deleteItr = runningPods - cpod.Spec.Count
				klog.Infof("Deleting %v extra pods\n", deleteItr)
			}
        }
	}

	//Detect the manually created pod, and delete that specific pod.
	if Deletepod {
		for i := 0; i < deleteItr; i++ {
			err := c.kubeClient.CoreV1().Pods(cpod.Namespace).Delete(context.TODO(), pList.Items[i].Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("Pod deletion failed for CR %v\n", cpod.Name)
				return err
			}
		}
	}

	// Creates pod
	if Createpod {
		for i := 0; i < itr; i++ {
			nPod, err := c.kubeClient.CoreV1().Pods(cpod.Namespace).Create(context.TODO(), newPod(cpod), metav1.CreateOptions{})
			klog.Info("Error ",err)
			if err != nil {

				if errors.IsAlreadyExists(err) {
					// retry (might happen when the same named pod is created again)
					itr++
				} else {
					klog.Errorf("Pod creation failed for CR %v\n", cpod.Name)
					return err
				}
			}
			if nPod.Name != "" {
				klog.Infof("Pod %v created successfully!\n", nPod.Name)
			}
		}
	}

	return nil
}

// Creates the new pod with the specified template
func newPod(cpod *v1alpha1.Customcluster) *corev1.Pod {
	labels := map[string]string{
		"controller": cpod.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      fmt.Sprintf(cpod.Name + "-" + strconv.Itoa(rand.Intn(10000000))),
			Namespace: cpod.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cpod, v1alpha1.SchemeGroupVersion.WithKind("Customcluster")),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "my-nginx",
					Image: "nginx:latest",
					Env: []corev1.EnvVar{
						{
							Name:  "MESSAGE",
							Value: cpod.Spec.Message,
						},
					},
					Command: []string{
						"/bin/sh",
					},
					Args: []string{
						"-c",
						"while true; do echo '$(MESSAGE)'; sleep 100; done",
					},
				},
			},
		},
	}
}

// If the pod doesn't switch to a running state within 10 minutes, shall report.
func (c *Controller) waitForPods(cpod *v1alpha1.Customcluster, pList *corev1.PodList) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return poll.Wait(ctx, func(ctx context.Context) (bool, error) {
		runningPods := c.totalRunningPods(cpod)
        klog.Info("Total pods running %v",runningPods)
		if runningPods == cpod.Spec.Count {
			return true, nil
		}
		return false, nil
	})
}

// Updates the status section of TrackPod
func (c *Controller) updateStatus(cpod *v1alpha1.Customcluster, progress string, pList *corev1.PodList) error {
	fmt.Println("Just entered the update status fn")
	t, err := c.cpodClient.SamplecontrollerV1alpha1().Customclusters(cpod.Namespace).Get(context.Background(), cpod.Name, metav1.GetOptions{})
	totrunningPods := c.totalRunningPods(cpod)
	if err != nil {
		return err
	}
    fmt.Println("Got the total pods running")
	klog.Info("Running pods=",totrunningPods)
	t.Status.Count = totrunningPods
	t.Status.Message = progress
	_, err = c.cpodClient.SamplecontrollerV1alpha1().Customclusters(cpod.Namespace).UpdateStatus(context.Background(), t, metav1.UpdateOptions{})

	return err
}

func (c *Controller) handleAdd(obj interface{}) {
	klog.Info("Inside handleAdd!!!")
	c.wq.Add(obj)
}

func (c *Controller) handleDel(obj interface{}) {
	klog.Info("Inside handleDel!!")
	c.wq.Done(obj)
}
