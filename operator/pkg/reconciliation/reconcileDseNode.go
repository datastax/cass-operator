package reconciliation

import (
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/google/uuid"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/httphelper"
)

var reconcileDseNodeLogger = logf.Log.WithName("dsenode_controller")

type ReconcileDseNode struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	httpClient httphelper.HttpClient
}

// newReconciler returns a new reconcile.Reconciler
func NewDseNodeReconciler(mgr manager.Manager, client httphelper.HttpClient) reconcile.Reconciler {
	return &ReconcileDseNode{
		client:     mgr.GetClient(),
		httpClient: http.DefaultClient,
	}
}

func (r *ReconcileDseNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := reconcileDseNodeLogger.
		WithValues("requestNamespace", request.Namespace).
		WithValues("requestName", request.Name).
		// loopID is used to tie all events together that are spawned by the same reconciliation loop
		WithValues("loopID", uuid.New().String())

	reqLogger.Info("======== reconcileDseNode::Reconcile has been called")

	rc, err := dsereconciliation.CreateReconciliationContext(
		&request,
		r.client,
		nil,
		reqLogger)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("DseDatacenter resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get DseDatacenter.")
		return reconcile.Result{Requeue: true}, err
	}

	if err := refreshSeeds(rc, r.httpClient); err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

func buildPodHostFromPod(pod corev1.Pod) string {
	return httphelper.GetPodHost(
		pod.Name,
		pod.Labels[datastaxv1alpha1.CLUSTER_LABEL],
		pod.Labels[datastaxv1alpha1.DATACENTER_LABEL],
		pod.Namespace)
}

func refreshSeeds(rc *dsereconciliation.ReconciliationContext, client httphelper.HttpClient) error {
	rc.ReqLogger.Info("reconcileDseNode::refreshSeeds")

	selector := map[string]string{
		datastaxv1alpha1.CLUSTER_LABEL: rc.DseDatacenter.Spec.ClusterName,
	}
	podList, err := listPods(rc, selector)
	if err != nil {
		rc.ReqLogger.Error(err, "No pods found for DseDatacenter")
		return err
	}

	for _, pod := range podList.Items {
		rc.ReqLogger.Info("reloading seeds for pod from DSE Node Management API",
			"pod", pod.Name)

		request := httphelper.NodeMgmtRequest{
			Endpoint: "/api/v0/ops/seeds/reload",
			Host:     buildPodHostFromPod(pod),
			Client:   client,
			Method:   http.MethodPost,
		}

		if err := httphelper.CallNodeMgmtEndpoint(rc.ReqLogger, request); err != nil {
			return err
		}
	}

	return nil
}

func listPods(rc *dsereconciliation.ReconciliationContext, selector map[string]string) (*corev1.PodList, error) {
	rc.ReqLogger.Info("reconcileDseNode::listPods")

	listOptions := &client.ListOptions{
		Namespace:     rc.DseDatacenter.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	podList := &corev1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}

	return podList, rc.Client.List(rc.Ctx, listOptions, podList)
}
