package dsereconciliation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
)

// ReconciliationContext contains all of the input necessary to calculate a list of ReconciliationActions
type ReconciliationContext struct {
	Request       *reconcile.Request
	Client        client.Client
	Scheme        *runtime.Scheme
	DseDatacenter *datastaxv1alpha1.DseDatacenter
	// Note that logr.Logger is an interface,
	// so we do not want to store a pointer to it
	// see: https://stackoverflow.com/a/44372954
	ReqLogger logr.Logger
	// According to golang recommendations the context should not be stored in a struct but given that
	// this is passed around as a parameter we feel that its a fair compromise. For further discussion
	// see: golang/go#22602
	Ctx context.Context
}

//
// Gather all information needed for computeReconciliationActions into a struct.
//
func CreateReconciliationContext(
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme,
	ReqLogger logr.Logger) (*ReconciliationContext, error) {

	rc := &ReconciliationContext{}
	rc.Request = request
	rc.Client = client
	rc.Scheme = scheme
	rc.ReqLogger = ReqLogger
	rc.Ctx = context.Background()

	rc.ReqLogger = rc.ReqLogger.
		WithValues("namespace", request.Namespace)

	rc.ReqLogger.Info("handler::CreateReconciliationContext")

	// Fetch the DseDatacenter dseDatacenter
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{}
	if err := retrieveDseDatacenter(rc, request, dseDatacenter); err != nil {
		return nil, err
	}
	rc.DseDatacenter = dseDatacenter

	rc.ReqLogger = rc.ReqLogger.
		WithValues("dseDatacenterName", dseDatacenter.Name).
		WithValues("dseDatacenterClusterName", dseDatacenter.ClusterName)

	return rc, nil
}

func retrieveDseDatacenter(rc *ReconciliationContext, request *reconcile.Request, dseDatacenter *datastaxv1alpha1.DseDatacenter) error {
	err := rc.Client.Get(
		rc.Ctx,
		request.NamespacedName,
		dseDatacenter)
	if err != nil {
		if errors.IsNotFound(err) {
			// Chance this was a pod event so get the DseDatacenter via the pod
			if innerErr := retrieveDseDatacenterByPod(rc, request, dseDatacenter); innerErr != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func retrieveDseDatacenterByPod(rc *ReconciliationContext, request *reconcile.Request, dseDatacenter *datastaxv1alpha1.DseDatacenter) error {
	pod := &corev1.Pod{}
	err := rc.Client.Get(
		rc.Ctx,
		request.NamespacedName,
		pod)
	if err != nil {
		rc.ReqLogger.Info("Unable to get pod",
			"podName", request.Name)
		return err
	}

	// Its entirely possible that a pod could be missing OwnerReferences even though it should be owned by a
	// statefulset. The most likely scenario for this would be if a pod label was modified, causing the selector on
	// the statefulset to no longer find the pod. Once the pod has been reconciled and we've fixed the label its OwnerReferences
	// should show back up and everything will be fine.
	if len(pod.OwnerReferences) == 0 {
		rc.ReqLogger.Info("OwnerReferences missing for pod",
			"podName",
			pod.Name)
		return fmt.Errorf("pod=%s missing OwnerReferences", pod.Name)
	}

	statefulSet := &appsv1.StatefulSet{}
	err = rc.Client.Get(
		rc.Ctx,
		types.NamespacedName{
			Name:      pod.OwnerReferences[0].Name,
			Namespace: pod.Namespace},
		statefulSet)
	if err != nil {
		rc.ReqLogger.Info("Unable to get statefulset",
			"statefulsetName", pod.OwnerReferences[0].Name)
		return err
	}

	err = rc.Client.Get(
		rc.Ctx,
		types.NamespacedName{
			Name:      statefulSet.OwnerReferences[0].Name,
			Namespace: pod.Namespace},
		dseDatacenter)
	if err != nil {
		rc.ReqLogger.Info("Unable to get DseDatacenter",
			"dseDatacenterName", statefulSet.OwnerReferences[0].Name)
		return err
	}
	return nil
}
