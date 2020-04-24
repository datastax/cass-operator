// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
	"errors"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = logf.Log.WithName("api")

func attemptedTo(action string, actionStrArgs ...interface{}) error {
	var msg string
	if actionStrArgs != nil {
		msg = fmt.Sprintf(action, actionStrArgs...)

	} else {
		msg = action
	}
	return fmt.Errorf("CassandraDatacenter attempted to %s", msg)
}

// Ensure that no values are improperly set
func ValidateSingleDatacenter(dc CassandraDatacenter) error {
	// Ensure serverVersion and serverType are compatible

	if dc.Spec.ServerType == "dse" && dc.Spec.ServerVersion != "6.8.0" {
		return attemptedTo("use unsupported DSE version '%s'", dc.Spec.ServerVersion)
	}

	if dc.Spec.ServerType == "cassandra" && dc.Spec.ServerVersion != "3.11.6" && dc.Spec.ServerVersion != "4.0.0" {
		return attemptedTo("use unsupported Cassandra version '%s'", dc.Spec.ServerVersion)
	}

	return nil
}

// Ensure that no values are improperly set
func ValidateDatacenterFieldChanges(oldDc CassandraDatacenter, newDc CassandraDatacenter) error {

	if oldDc.Spec.ClusterName != newDc.Spec.ClusterName {
		return attemptedTo("change ClusterName")
	}

	if oldDc.Spec.AllowMultipleNodesPerWorker != newDc.Spec.AllowMultipleNodesPerWorker {
		return attemptedTo("change AllowMultipleNodesPerWorker")
	}

	if oldDc.Spec.SuperuserSecretName != newDc.Spec.SuperuserSecretName {
		return attemptedTo("change SuperuserSecretName")
	}

	if oldDc.Spec.ServiceAccount != newDc.Spec.ServiceAccount {
		return attemptedTo("change ServiceAccount")
	}

	// StorageConfig changes are disallowed
	if !reflect.DeepEqual(oldDc.Spec.StorageConfig, newDc.Spec.StorageConfig) {
		return attemptedTo("change StorageConfig")
	}

	// Topology changes - Racks
	// - Rack Name and Zone changes are disallowed.
	// - Removing racks is not supported.
	// - Reordering the rack list is not supported.
	// - Any new racks must be added to the end of the current rack list.

	oldRacks := oldDc.GetRacks()
	newRacks := newDc.GetRacks()

	if len(oldRacks) > len(newRacks) {
		return attemptedTo("remove Rack")
	}

	newRackCount := len(newRacks) - len(oldRacks)
	if newRackCount > 0 {
		newSizeDifference := newDc.Spec.Size - oldDc.Spec.Size

		if newSizeDifference <= 0 {
			return attemptedTo("add Rack without increasing Size")
		}

		if int(newSizeDifference) < newRackCount {
			return attemptedTo(
				fmt.Sprintf("add Racks without increasing Size enough to populate each rack.\n"+
					"New racks added: %d, Size increased by: %d", newRackCount, newSizeDifference))
		}
	}

	for index, oldRack := range oldRacks {
		newRack := newRacks[index]
		if oldRack.Name != newRack.Name {
			return attemptedTo("change Rack Name from '%s' to '%s'",
				oldRack.Name,
				newRack.Name)
		}
		if oldRack.Zone != newRack.Zone {
			return attemptedTo("change Rack Zone from '%s' to '%s'",
				oldRack.Zone,
				newRack.Zone)
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-cassandradatacenter,mutating=false,failurePolicy=ignore,groups=cassandra.datastax.com,resources=cassandradatacenters,verbs=create;update;delete,versions=v1beta1,name=validate-cassandradatacenter-webhook
var _ webhook.Validator = &CassandraDatacenter{}

func (dc *CassandraDatacenter) ValidateCreate() error {
	log.Info("Validating webhook called for create")
	err := ValidateSingleDatacenter(*dc)
	if err != nil {
		return err
	}

	return nil
}

func (dc *CassandraDatacenter) ValidateUpdate(old runtime.Object) error {
	log.Info("Validating webhook called for update")
	oldDc, ok := old.(*CassandraDatacenter)
	if !ok {
		return errors.New("old object in ValidateUpdate cannot be cast to CassandraDatacenter")
	}

	err := ValidateSingleDatacenter(*dc)
	if err != nil {
		return err
	}

	return ValidateDatacenterFieldChanges(*oldDc, *dc)
}

func (dc *CassandraDatacenter) ValidateDelete() error {
	return nil
}
