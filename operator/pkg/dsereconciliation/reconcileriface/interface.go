package reconcileriface

import "sigs.k8s.io/controller-runtime/pkg/reconcile"

type Reconciler interface {
	Apply() (reconcile.Result, error)
}
