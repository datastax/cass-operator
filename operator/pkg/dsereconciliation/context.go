package dsereconciliation

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/httphelper"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
)

// ReconciliationContext contains all of the input necessary to calculate a list of ReconciliationActions
type ReconciliationContext struct {
	Request        *reconcile.Request
	Client         client.Client
	Scheme         *runtime.Scheme
	DseDatacenter  *datastaxv1alpha1.DseDatacenter
	NodeMgmtClient httphelper.NodeMgmtClient
	// Note that logr.Logger is an interface,
	// so we do not want to store a pointer to it
	// see: https://stackoverflow.com/a/44372954
	ReqLogger logr.Logger
	// According to golang recommendations the context should not be stored in a struct but given that
	// this is passed around as a parameter we feel that its a fair compromise. For further discussion
	// see: golang/go#22602
	Ctx context.Context
}

// CreateReconciliationContext gathers all information needed for computeReconciliationActions into a struct.
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
		rc.ReqLogger.Error(err, "error in retrieveDseDatacenter")
		return nil, err
	}
	rc.DseDatacenter = dseDatacenter

	// workaround for kubernetes having problems with zero-value and nil Times
	if rc.DseDatacenter.Status.SuperUserUpserted.IsZero() {
		rc.DseDatacenter.Status.SuperUserUpserted = metav1.Unix(1, 0)
	}
	if rc.DseDatacenter.Status.LastDseNodeStarted.IsZero() {
		rc.DseDatacenter.Status.LastDseNodeStarted = metav1.Unix(1, 0)
	}

	httpClient, err := httphelper.BuildManagementApiHttpClient(dseDatacenter, client, rc.Ctx)
	if err != nil {
		rc.ReqLogger.Error(err, "error in BuildManagementApiHttpClient")
		return nil, err
	}

	rc.ReqLogger = rc.ReqLogger.
		WithValues("dseDatacenterName", dseDatacenter.Name).
		WithValues("dseDatacenterClusterName", dseDatacenter.Spec.DseClusterName)

	protocol, err := httphelper.GetManagementApiProtocol(dseDatacenter)
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

func retrieveDseDatacenter(rc *ReconciliationContext, request *reconcile.Request, dseDatacenter *datastaxv1alpha1.DseDatacenter) error {
	err := rc.Client.Get(
		rc.Ctx,
		request.NamespacedName,
		dseDatacenter)
	return err
}
