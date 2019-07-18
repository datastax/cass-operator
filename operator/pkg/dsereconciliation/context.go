package dsereconciliation

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
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
