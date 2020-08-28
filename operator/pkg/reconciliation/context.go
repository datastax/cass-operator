// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/dynamicwatch"
	"github.com/datastax/cass-operator/operator/pkg/events"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/psp"
	"github.com/datastax/cass-operator/operator/pkg/utils"
)

// ReconciliationContext contains all of the input necessary to calculate a list of ReconciliationActions
type ReconciliationContext struct {
	Request          *reconcile.Request
	Client           runtimeClient.Client
	Scheme           *runtime.Scheme
	Datacenter       *api.CassandraDatacenter
	NodeMgmtClient   httphelper.NodeMgmtClient
	Recorder         record.EventRecorder
	ReqLogger        logr.Logger
	PSPHealthUpdater psp.HealthStatusUpdater
	SecretWatches    dynamicwatch.DynamicWatches

	// According to golang recommendations the context should not be stored in a struct but given that
	// this is passed around as a parameter we feel that its a fair compromise. For further discussion
	// see: golang/go#22602
	Ctx context.Context

	Services               []*corev1.Service
	Endpoints              *corev1.Endpoints
	desiredRackInformation []*RackInformation
	statefulSets           []*appsv1.StatefulSet
	dcPods                 []*corev1.Pod
	clusterPods            []*corev1.Pod
}

// CreateReconciliationContext gathers all information needed for computeReconciliationActions into a struct.
func CreateReconciliationContext(
	req *reconcile.Request,
	cli runtimeClient.Client,
	scheme *runtime.Scheme,
	rec record.EventRecorder,
	secretWatches dynamicwatch.DynamicWatches,
	reqLogger logr.Logger) (*ReconciliationContext, error) {
	
	rc := &ReconciliationContext{}
	rc.Request = req
	rc.Client = cli
	rc.Scheme = scheme
	rc.Recorder = &events.LoggingEventRecorder{EventRecorder: rec, ReqLogger: reqLogger}
	rc.SecretWatches = secretWatches
	rc.ReqLogger = reqLogger
	rc.Ctx = context.Background()

	rc.ReqLogger = rc.ReqLogger.
		WithValues("namespace", req.Namespace)

	rc.ReqLogger.Info("handler::CreateReconciliationContext")

	if utils.IsPSPEnabled() {
		// Add PSP health status updater
		// TODO: Feature gate this
		operatorNs, err := k8sutil.GetOperatorNamespace()
		if err != nil {
			return nil, err
		}
		rc.PSPHealthUpdater = psp.NewHealthStatusUpdater(cli, operatorNs)
	} else {
		// Use no-op updater if PSP is disabled
		rc.PSPHealthUpdater = &psp.NoOpUpdater{}
	}

	// Fetch the datacenter resource
	dc := &api.CassandraDatacenter{}
	if err := retrieveDatacenter(rc, req, dc); err != nil {
		rc.ReqLogger.Error(err, "error in retrieveDatacenter")
		return nil, err
	}
	rc.Datacenter = dc

	// workaround for kubernetes having problems with zero-value and nil Times
	if rc.Datacenter.Status.SuperUserUpserted.IsZero() {
		rc.Datacenter.Status.SuperUserUpserted = metav1.Unix(1, 0)
	}
	if rc.Datacenter.Status.LastServerNodeStarted.IsZero() {
		rc.Datacenter.Status.LastServerNodeStarted = metav1.Unix(1, 0)
	}
	if rc.Datacenter.Status.LastRollingRestart.IsZero() {
		rc.Datacenter.Status.LastRollingRestart = metav1.Unix(1, 0)
	}

	httpClient, err := httphelper.BuildManagementApiHttpClient(dc, cli, rc.Ctx)
	if err != nil {
		rc.ReqLogger.Error(err, "error in BuildManagementApiHttpClient")
		return nil, err
	}

	rc.ReqLogger = rc.ReqLogger.
		WithValues("datacenterName", dc.Name).
		WithValues("clusterName", dc.Spec.ClusterName)

	protocol, err := httphelper.GetManagementApiProtocol(dc)
	if err != nil {
		rc.ReqLogger.Error(err, "error in GetManagementApiProtocol")
		return nil, err
	}

	rc.NodeMgmtClient = httphelper.NodeMgmtClient{
		Client:   httpClient,
		Log:      rc.ReqLogger,
		Protocol: protocol,
	}

	return rc, nil
}

func retrieveDatacenter(rc *ReconciliationContext, request *reconcile.Request, dc *api.CassandraDatacenter) error {
	err := rc.Client.Get(
		rc.Ctx,
		request.NamespacedName,
		dc)
	return err
}
