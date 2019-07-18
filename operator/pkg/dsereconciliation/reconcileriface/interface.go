package reconcileriface

import "sigs.k8s.io/controller-runtime/pkg/reconcile"

// Reconciler ...
type Reconciler interface {
	Apply() (reconcile.Result, error)
}
