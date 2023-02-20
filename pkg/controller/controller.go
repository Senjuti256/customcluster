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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const controllerAgentName = "custom-controller"

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

    //CustomclusterInformer informers.CustomclusterInformer) *Controller {

		// Create event broadcaster
		// Add sample-controller types to the default Kubernetes Scheme so Events can be
		// logged for sample-controller types.
		/*utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
		klog.V(4).Info("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		eventBroadcaster.StartStructuredLogging(0)
		eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
		recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	
	*/
	
	c := &Controller{
		kubeClient: kubeClient,
		cpodClient: cpodClient,
		cpodSync:   cpodInformer.Informer().HasSynced,
		cpodlister: cpodInformer.Lister(),
		wq:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Customcluster"),
	}
    klog.Info("NewController made\n")
    klog.Info("Setting up event handlers")

	/*CustomclusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleAdd,
		UpdateFunc: func(old,new interface{}){
			c.handleAdd(new)
		},
	})*/

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
*/

func (c *Controller) processNextItem() bool {
	klog.Info("Inside processNextItem method")
	obj, shutdown := c.wq.Get()
	if shutdown {
		klog.Info("Shutting down")
		return false
	}
   /*
	err := func(obj interface{}) error{
	    defer c.wq.Forget(item)

	

    if err := c.syncHandler(item.(string)); err != nil {

	   klog.Fatalf("Error syncing the current and desired state--%s",err.Error())
	   return false
    }
    klog.Info("successfully synced ",item.(string))

    return true*/
	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.wq.Done(obj)
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
			c.wq.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.wq.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.wq.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Foo resource with this namespace/name
	cpod, err := c.cpodlister.Customclusters(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"controller": cpod.Name, 
		},
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}

	pList, err := c.kubeClient.CoreV1().Pods(cpod.Namespace).List(context.TODO(), listOptions)

	klog.Info("In the syncHandler to sync pods")
	
	if err := c.syncPods(cpod, pList); err != nil {
		klog.Fatalf("Error while syncing the current vs desired state for App %v: %v\n", cpod.Name, err.Error())
		
	}
	
	klog.Info("synced pods successfully\n")

	err = c.waitForPods(cpod, pList)
	if err != nil {
		klog.Fatalf("error %s, waiting for pods to meet the expected state", err.Error())
		
	}
	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world

	klog.Info("successfully waited for pods")

    klog.Info("\n-------Message---------",cpod.Spec.Message)
	klog.Info("\n-------Count---------",cpod.Spec.Count)


	err = c.updateStatus(cpod, cpod.Spec.Message, pList)
	if err != nil {
		return err
	}

	klog.Info("successfully updated status")

	return nil
}

func (c *Controller) syncPods(cpod *v1alpha1.Customcluster, pList *corev1.PodList) error {
	newPods := cpod.Spec.Count
	klog.Info("\n-------Count inside syncPods--------",cpod.Spec.Count)

	currentPods := c.totalRunningPods(cpod)
	klog.Info("\n-------Current cnt of running pods-------",currentPods)

	newMessage := cpod.Spec.Message
    klog.Info("\n-------Message inside syncPods--------",cpod.Spec.Message)

	currentMessage := cpod.Status.Message
	klog.Info("\n-------Message inside syncPods--------",cpod.Status.Message)

	var ifDelete, ifCreate bool
	numCreate := newPods             //int(*&newPods)
	numDelete := 0

	if newPods != currentPods || newMessage != currentMessage {
		if newMessage != currentMessage {
			ifDelete = true
			ifCreate = true
			numCreate = newPods             //&
			numDelete = currentPods
		} else {
			if currentPods < newPods {            //&
				ifCreate = true
				numCreate = newPods - currentPods           //&
			} else if currentPods > newPods {               //&
				ifDelete = true
				numDelete = currentPods - newPods           //&
			}
		}
	}

	if ifDelete {
		klog.Info("pods are getting deletes ", numDelete)
		
		for i:= 0; i < numDelete ; i++ {
			err := c.kubeClient.CoreV1().Pods(cpod.Namespace).Delete(context.TODO(), pList.Items[i].Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Fatalf("error while deleting the pods %v", cpod.Name)
				return err
			}
		}
		klog.Info("pods deleted successfully")
		
	}

	if ifCreate {
		klog.Info("pods are getting created ", numCreate)
		for i := 0; i < numCreate ; i++ {
			newcpod, err := c.kubeClient.CoreV1().Pods(cpod.Namespace).Create(context.TODO(), newPod(cpod), metav1.CreateOptions{})
			if err != nil {
				if errors.IsAlreadyExists(err) {
					numCreate++
				} else {
					klog.Fatalf("error in creating pods %v", cpod.Name)
					return err
				}
			} 
			if newcpod.Name != "" {
				klog.Info("Created!")
			}
		}
	}

	return nil
}

func (c *Controller) waitForPods(cpod *v1alpha1.Customcluster, psList *corev1.PodList) error {
	klog.Info("waiting for pods to be in running state")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	return poll.Wait(ctx, func(ctx context.Context) (bool, error) {
		currentPods := c.totalRunningPods(cpod)

		if currentPods == int(cpod.Spec.Count) {                                   //& added
			return true, nil
		}
		return false, nil
	})
}

func (c *Controller) totalRunningPods(cr *v1alpha1.Customcluster) int {
	klog.Info("calculating total number of running pods")
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"controller": cr.Name,
		},
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	psList, _ := c.kubeClient.CoreV1().Pods(cr.Namespace).List(context.TODO(), listOptions)
	
	currentPods := 0

	for _,pod := range psList.Items {
		if pod.ObjectMeta.DeletionTimestamp.IsZero() && pod.Status.Phase == "Running" {
			currentPods++
		}
	}

	klog.Info("Total number of running pods are ", currentPods)

	return currentPods
}

func (c *Controller) updateStatus(cp *v1alpha1.Customcluster, message string, psList *corev1.PodList) error {

	cpCopy, err := c.cpodClient.SamplecontrollerV1alpha1().Customclusters(cp.Namespace).Get(context.TODO(), cp.Name, metav1.GetOptions{})
	currentPods := c.totalRunningPods(cp)
	if err != nil {
		return err
	}

	cpCopy.Status.Count = currentPods
	cpCopy.Status.Message = message

	klog.Info("updating status")
	_, err = c.cpodClient.SamplecontrollerV1alpha1().Customclusters(cp.Namespace).UpdateStatus(context.TODO(), cpCopy, metav1.UpdateOptions{})

	klog.Info("updates status with error = ", err.Error())

	return err


}

func newPod(app *v1alpha1.Customcluster) *corev1.Pod {
	klog.Info("new pods creation function")
	labels := map[string]string{
		"controller": app.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
			Name: fmt.Sprintf(app.Name + "-" + strconv.Itoa(rand.Intn(100000000))),
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, v1alpha1.SchemeGroupVersion.WithKind("App")),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "nginx",
					Image: "nginx:latest",
					Env: []corev1.EnvVar{
						{
							Name: "MESSAGE",
							Value: app.Spec.Message,
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


func (c *Controller) handleAdd(obj interface{}) {
	klog.Info("In the createHandler")
	
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.wq.Add(key)
}

func (c *Controller) handleDel(obj interface{}) {
	klog.Info("In the deleteHandler")
	
	c.wq.Done(obj)
}