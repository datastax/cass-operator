// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = logf.Log.WithName("api")

// +kubebuilder:webhook:path=/validate-cassandradatacenter,mutating=false,failurePolicy=ignore,groups=cassandra.datastax.com,resources=cassandradatacenters,verbs=create;update;delete,versions=v1beta1,name=validate-cassandradatacenter-webhook
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
		return errors.New("CassandraDatacenter attempted to change ClusterName")
	}

	if dc.Spec.AllowMultipleNodesPerWorker != oldDc.Spec.AllowMultipleNodesPerWorker {
		return errors.New("CassandraDatacenter attempted to change AllowMultipleNodesPerWorker")
	}

	// Topology changes - Racks
	// - Rack Name and Zone changes are disallowed.
	// - Removing racks is not supported.
	// - Reordering the rack list is not supported.
	// - Any new racks must be added to the end of the current rack list.

	if len(oldDc.Spec.Racks) > len(dc.Spec.Racks) {
		return fmt.Errorf("CassandraDatacenter attempted to remove Rack")
	}

	for index, oldRack := range oldDc.Spec.Racks {
		newRack := dc.Spec.Racks[index]
		if oldRack.Name != newRack.Name {
			return fmt.Errorf("CassandraDatacenter attempted to change Rack Name from '%s' to '%s'",
				oldRack.Name,
				newRack.Name)
		}
		if oldRack.Zone != newRack.Zone {
			return fmt.Errorf("CassandraDatacenter attempted to change Rack Zone from '%s' to '%s'",
				oldRack.Zone,
				newRack.Zone)
		}
	}

	// TODO - no allowMultipleNodesPerWorker changes

	// TODO - no storage config changes

	// TODO ensure serverVersion and serverType are compatible

	return nil
}

func (dc *CassandraDatacenter) ValidateDelete() error {
	return nil
}
