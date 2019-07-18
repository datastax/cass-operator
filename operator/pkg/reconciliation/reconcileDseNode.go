package reconciliation

import (
	"fmt"
	"io/ioutil"
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

func refreshSeeds(rc *dsereconciliation.ReconciliationContext, client httphelper.HttpClient) error {
	rc.ReqLogger.Info("reconcileDseNode::refreshSeeds")

	podList, err := listPods(rc)
	if err != nil {
		rc.ReqLogger.Error(err, "No pods found for DseDatacenter")
		return err
	}

	for _, pod := range podList.Items {
		rc.ReqLogger.Info("Reloading seeds for pod",
			"pod", pod.Name)

		if err := callNodeMgmtEndpoint(rc, client, pod, "/api/v0/ops/seeds/reload"); err != nil {
			return err
		}
	}

	return nil
}

func listPods(rc *dsereconciliation.ReconciliationContext) (*corev1.PodList, error) {
	rc.ReqLogger.Info("reconcileDseNode::listPods")

	selector := map[string]string{
		datastaxv1alpha1.CLUSTER_LABEL: rc.DseDatacenter.Spec.ClusterName,
	}

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

func callNodeMgmtEndpoint(rc *dsereconciliation.ReconciliationContext, client httphelper.HttpClient, pod corev1.Pod, apiEndpoint string) error {
	rc.ReqLogger.Info("reconcileDseNode::callNodeMgmtEndpoint")

	nodeServicePattern := "%s.%s-%s-service.%s"

	podHost := fmt.Sprintf(nodeServicePattern, pod.Name, rc.DseDatacenter.Spec.ClusterName, rc.DseDatacenter.Name, rc.DseDatacenter.Namespace)

	url := "http://" + podHost + ":8080" + apiEndpoint
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		rc.ReqLogger.Error(err, "unable to create request")
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		rc.ReqLogger.Error(err, "unable to do request")
		return err
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			rc.ReqLogger.Error(err, "unable to close response body")
		}
	}()

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		rc.ReqLogger.Error(err, "unable to read response body")
		return err
	}

	if res.StatusCode != http.StatusOK {
		rc.ReqLogger.Info("incorrect status code when reloading seeds",
			"statusCode", res.StatusCode,
			"pod", podHost)

		return fmt.Errorf("incorrect status code of %d when reloading seeds", res.StatusCode)
	}

	return nil
}
