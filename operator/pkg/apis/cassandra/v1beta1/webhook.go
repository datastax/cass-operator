// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
	"errors"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = logf.Log.WithName("api")

// +kubebuilder:webhook:path=/validate-cassandradatacenter,mutating=false,failurePolicy=ignore,groups=cassandra.datastax.com,resources=cassandradatacenters,verbs=create;update;delete,versions=v1alpha2,name=validate-cassandradatacenter-webhook
var _ webhook.Validator = &CassandraDatacenter{}

func (dc *CassandraDatacenter) ValidateCreate() error {
	return nil
}

func (dc *CassandraDatacenter) ValidateUpdate(old runtime.Object) error {
	log.Info("validating webhook called")
	oldDc, ok := old.(*CassandraDatacenter)
	if !ok {
		log.Info("validating webhook could not cast")
		return errors.New("old object in ValidateUpdate cannot be cast to CassandraDatacenter")
	}

	if dc.Spec.ClusterName != oldDc.Spec.ClusterName {
		log.Info("attempt to change clustername")
		return errors.New("CassandraDatacenter attempted to change ClusterName")
	}
	log.Info("no attempt to change clustername")

	return nil
}

func (dc *CassandraDatacenter) ValidateDelete() error {
	return nil
}
