package dsedatacenter

import (
	"context"

	logr "github.com/go-logr/logr"
	datastaxv1alpha1 "github.com/riptano/dse-operator/pkg/apis/datastax/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_dsedatacenter")

// Add creates a new DseDatacenter Controller and adds it to the Manager.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDseDatacenter{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("dsedatacenter-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DseDatacenter
	err = c.Watch(&source.Kind{Type: &datastaxv1alpha1.DseDatacenter{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO: list all secondary resources we want to monitor here
	//
	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner DseDatacenter
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &datastaxv1alpha1.DseDatacenter{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDseDatacenter{}

// ReconcileDseDatacenter reconciles a DseDatacenter object
type ReconcileDseDatacenter struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DseDatacenter object and makes changes based on the state read
// and what is in the DseDatacenter.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDseDatacenter) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DseDatacenter")

	// Fetch the DseDatacenter dseDatacenter
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{}
	err := r.client.Get(context.TODO(), request.NamespacedName, dseDatacenter)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	//
	// Set up a headless service for the cluster
	//

	service := newServiceForDseDatacenter(dseDatacenter)

	// Set DseDatacenter dseDatacenter as the owner and controller
	if err := controllerutil.SetControllerReference(dseDatacenter, service, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if the service already exists
	serviceFound := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, serviceFound)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
		err = r.client.Create(context.TODO(), service)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	//
	// Set up a statefulset for the cluster
	//

	nodeCount := int(dseDatacenter.Spec.Size)
	rackCount := len(dseDatacenter.Spec.Racks)

	if rackCount == 0 {
		// Just reconcile the "default rack"
		err = reconcileRack("default", r, reqLogger, dseDatacenter, service, nodeCount)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else {
		// nodes_per_rack = total_size / rack_count + 1 if rack_index < remainder

		nodesPerRack, extraNodes := nodeCount/rackCount, nodeCount%rackCount

		for rackIndex, dseRack := range dseDatacenter.Spec.Racks {
			nodesForThisRack := nodesPerRack
			if rackIndex < extraNodes {
				nodesForThisRack += 1
			}

			// TODO in the future, handle labels and annotations for the rack
			err = reconcileRack(dseRack.Name, r, reqLogger, dseDatacenter, service, nodesForThisRack)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Don't requeue
	return reconcile.Result{}, nil
}

// This creates a headless service for the DSE Datacenter.
func newServiceForDseDatacenter(dseDatacenter *datastaxv1alpha1.DseDatacenter) *corev1.Service {
	// TODO adjust labels
	labels := map[string]string{
		"app": dseDatacenter.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dseDatacenter.Name + "-service",
			Namespace: dseDatacenter.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			// This MUST match a template pod label in the statefulset
			Selector:  labels,
			Type:      "ClusterIP",
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				// Note: Port Names cannot be more than 15 characters
				{
					Name:       "native",
					Port:       9042,
					TargetPort: intstr.FromInt(9042),
				},
				{
					Name:       "inter-node-msg",
					Port:       8609,
					TargetPort: intstr.FromInt(8609),
				},
				{
					Name:       "intra-node",
					Port:       7000,
					TargetPort: intstr.FromInt(7000),
				},
				{
					Name:       "tls-intra-node",
					Port:       7001,
					TargetPort: intstr.FromInt(7001),
				},
				{
					Name:       "jmx",
					Port:       7199,
					TargetPort: intstr.FromInt(7199),
				},
			},
		},
	}
}

// Ensure that the resources for a dse rack have been properly created
func reconcileRack(
	rackName string,
	r *ReconcileDseDatacenter,
	reqLogger logr.Logger,
	dseDatacenter *datastaxv1alpha1.DseDatacenter,
	service *corev1.Service,
	replicaCount int) error {

	statefulSet := newStatefulSetForDseDatacenter(rackName, dseDatacenter, service, replicaCount)

	// Set DseDatacenter dseDatacenter as the owner and controller
	if err := controllerutil.SetControllerReference(dseDatacenter, statefulSet, r.scheme); err != nil {
		return err
	}

	// Check if the statefulSet already exists
	statefulSetFound := &policyv1beta1.PodDisruptionBudget{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: statefulSet.Name, Namespace: statefulSet.Namespace}, statefulSetFound)
	if err != nil && errors.IsNotFound(err) {
		// Create the StatefulSet
		reqLogger.Info(
			"Creating a new StatefulSet",
			"StatefulSet.Namespace",
			statefulSet.Namespace,
			"StatefulSet.Name",
			statefulSet.Name)
		err = r.client.Create(context.TODO(), statefulSet)
		if err != nil {
			return err
		}
	}

	//
	// Create a PodDisruptionBudget for the StatefulSet
	//

	// TODO: decide if budget is for one statefulset or all pods in the dc
	budget := newPodDisruptionBudgetForStatefulSet(dseDatacenter, statefulSet, service)

	// Set DseDatacenter dseDatacenter as the owner and controller
	if err := controllerutil.SetControllerReference(dseDatacenter, budget, r.scheme); err != nil {
		return err
	}

	// Check if the budget already exists
	budgetFound := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: budget.Name, Namespace: budget.Namespace}, budgetFound)
	if err != nil && errors.IsNotFound(err) {
		// Create the Budget
		reqLogger.Info("Creating a new PodDisruptionBudget", "PodDisruptionBudget.Namespace", budget.Namespace, "PodDisruptionBudget.Name", budget.Name)
		err = r.client.Create(context.TODO(), budget)
		if err != nil {
			return err
		}
	}

	return nil
}

// This creates a statefulset for the DSE Datacenter.
func newStatefulSetForDseDatacenter(
	rackName string,
	dseDatacenter *datastaxv1alpha1.DseDatacenter,
	service *corev1.Service,
	replicaCount int) *appsv1.StatefulSet {
	replicaCountInt32 := int32(replicaCount)
	labels := map[string]string{
		"app":  dseDatacenter.Name,
		"rack": rackName,
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dseDatacenter.Name + "-" + rackName + "-stateful-set",
			Namespace: dseDatacenter.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			// TODO adjust this
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas:    &replicaCountInt32,
			ServiceName: service.Name,

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						// TODO FIXME
						Name: "dse",
						// TODO make this dynamic
						Image: "datastax/dse-server:6.0.2",
						Env: []corev1.EnvVar{
							{
								Name:  "DS_LICENSE",
								Value: "accept",
							},
							// TODO FIXME - use custom seed handling
							{
								Name:  "SEEDS",
								Value: "example-dsedatacenter-default-stateful-set-0.example-dsedatacenter-service.default.svc.cluster.local",
							},
							{
								Name:  "NUM_TOKENS",
								Value: "32",
							},
						},
						Ports: []corev1.ContainerPort{
							// Note: Port Names cannot be more than 15 characters
							{
								Name:          "native",
								ContainerPort: 9042,
							},
							{
								Name:          "inter-node-msg",
								ContainerPort: 8609,
							},
							{
								Name:          "intra-node",
								ContainerPort: 7000,
							},
							{
								Name:          "tls-intra-node",
								ContainerPort: 7001,
							},
							{
								Name:          "jmx",
								ContainerPort: 7199,
							},
						},
					}},
				},
			},
		},
	}
}

// This creates a statefulset for the DSE Datacenter.
func newPodDisruptionBudgetForStatefulSet(dseDatacenter *datastaxv1alpha1.DseDatacenter, statefulSet *appsv1.StatefulSet, service *corev1.Service) *policyv1beta1.PodDisruptionBudget {
	// Right now, we will just have maxUnavailable at 1
	maxUnavailable := intstr.FromInt(1)
	labels := map[string]string{
		"app": dseDatacenter.Name,
	}
	return &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSet.Name + "-pdb",
			Namespace: statefulSet.Namespace,
			Labels:    labels,
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			// TODO figure selector policy this out
			// right now this is matching ALL pods for a given datacenter
			// across all racks
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			MaxUnavailable: &maxUnavailable,
		},
	}
}
