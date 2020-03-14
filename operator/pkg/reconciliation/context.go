package reconciliation

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/httphelper"

	api "github.com/riptano/dse-operator/operator/pkg/apis/cassandra/v1alpha2"
)

// ReconciliationContext contains all of the input necessary to calculate a list of ReconciliationActions
type ReconciliationContext struct {
	Request        *reconcile.Request
	Client         client.Client
	Scheme         *runtime.Scheme
	Datacenter     *api.CassandraDatacenter
	NodeMgmtClient httphelper.NodeMgmtClient
	Recorder       record.EventRecorder
	ReqLogger      logr.Logger
	// According to golang recommendations the context should not be stored in a struct but given that
	// this is passed around as a parameter we feel that its a fair compromise. For further discussion
	// see: golang/go#22602
	Ctx context.Context

	Services               []*corev1.Service
	desiredRackInformation []*RackInformation
	statefulSets           []*appsv1.StatefulSet
}

// CreateReconciliationContext gathers all information needed for computeReconciliationActions into a struct.
func CreateReconciliationContext(
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	ReqLogger logr.Logger) (*ReconciliationContext, error) {

	rc := &ReconciliationContext{}
	rc.Request = request
	rc.Client = client
	rc.Scheme = scheme
	rc.Recorder = recorder
	rc.ReqLogger = ReqLogger
	rc.Ctx = context.Background()

	rc.ReqLogger = rc.ReqLogger.
		WithValues("namespace", request.Namespace)

	rc.ReqLogger.Info("handler::CreateReconciliationContext")

	// Fetch the datacenter resource
	dc := &api.CassandraDatacenter{}
	if err := retrieveDatacenter(rc, request, dc); err != nil {
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

	httpClient, err := httphelper.BuildManagementApiHttpClient(dc, client, rc.Ctx)
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
