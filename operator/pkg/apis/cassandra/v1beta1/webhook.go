// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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
	return fmt.Errorf("CassandraDatacenter write rejected, attempted to %s", msg)
}

// ValidateSingleDatacenter checks that no values are improperly set on a CassandraDatacenter
func ValidateSingleDatacenter(dc CassandraDatacenter) error {
	// Ensure serverVersion and serverType are compatible

	var err error
	if dc.Spec.ServerType == "dse" {
		switch dc.Spec.ServerVersion {
		case "6.8.0":
			err = nil
		case "6.8.1":
			err = nil
		case "6.8.2":
			err = nil
		case "6.8.3":
			err = nil
		default:
			err = attemptedTo("use unsupported DSE version '%s'", dc.Spec.ServerVersion)
		}
	}
	if err != nil {
		return err
	}

	if dc.Spec.ServerType == "cassandra" && dc.Spec.DseWorkloads != nil {
		if dc.Spec.DseWorkloads.AnalyticsEnabled || dc.Spec.DseWorkloads.GraphEnabled || dc.Spec.DseWorkloads.SearchEnabled {
			return attemptedTo("enable DSE workloads if server type is Cassandra")
		}
	}

	if dc.Spec.ServerType == "cassandra" {
		switch dc.Spec.ServerVersion {
		case "3.11.6":
			err = nil
		case "3.11.7":
			err = nil
		case "4.0.0":
			err = nil
		default:
			err = attemptedTo("use unsupported Cassandra version '%s'", dc.Spec.ServerVersion)
		}
	}
	if err != nil {
		return err
	}

	isDse := dc.Spec.ServerType == "dse"
	isCassandra3 := dc.Spec.ServerType == "cassandra" && strings.HasPrefix(dc.Spec.ServerVersion, "3.")
	isCassandra4 := dc.Spec.ServerType == "cassandra" && strings.HasPrefix(dc.Spec.ServerVersion, "4.")

	var c map[string]interface{}
	_ = json.Unmarshal(dc.Spec.Config, &c)

	_, hasJvmOptions := c["jvm-options"]
	_, hasJvmServerOptions := c["jvm-server-options"]
	_, hasDseYaml := c["dse-yaml"]

	serverStr := fmt.Sprintf("%s-%s", dc.Spec.ServerType, dc.Spec.ServerVersion)
	if hasJvmOptions && (isDse || isCassandra4) {
		return attemptedTo("define config jvm-options with %s", serverStr)
	}
	if hasJvmServerOptions && isCassandra3 {
		return attemptedTo("define config jvm-server-options with %s", serverStr)
	}
	if hasDseYaml && (isCassandra3 || isCassandra4) {
		return attemptedTo("define config dse-yaml with %s", serverStr)
	}

	// if using multiple nodes per worker, requests and limits should be set for both cpu and memory
	if dc.Spec.AllowMultipleNodesPerWorker {
		if dc.Spec.Resources.Requests.Cpu().IsZero() ||
			dc.Spec.Resources.Limits.Cpu().IsZero() ||
			dc.Spec.Resources.Requests.Memory().IsZero() ||
			dc.Spec.Resources.Limits.Memory().IsZero() {

			return attemptedTo("use multiple nodes per worker without cpu and memory requests and limits")
		}
	}

	return nil
}

// ValidateDatacenterFieldChanges checks that no values are improperly changing while updating
// a CassandraDatacenter
func ValidateDatacenterFieldChanges(oldDc CassandraDatacenter, newDc CassandraDatacenter) error {

	if oldDc.Spec.ClusterName != newDc.Spec.ClusterName {
		return attemptedTo("change clusterName")
	}

	if oldDc.Spec.AllowMultipleNodesPerWorker != newDc.Spec.AllowMultipleNodesPerWorker {
		return attemptedTo("change allowMultipleNodesPerWorker")
	}

	if oldDc.Spec.SuperuserSecretName != newDc.Spec.SuperuserSecretName {
		return attemptedTo("change superuserSecretName")
	}

	if oldDc.Spec.ServiceAccount != newDc.Spec.ServiceAccount {
		return attemptedTo("change serviceAccount")
	}

	// StorageConfig changes are disallowed
	if !reflect.DeepEqual(oldDc.Spec.StorageConfig, newDc.Spec.StorageConfig) {
		return attemptedTo("change storageConfig")
	}

	if oldDc.Spec.Size > newDc.Spec.Size {
		return attemptedTo("decrease size")
	}

	// Topology changes - Racks
	// - Rack Name and Zone changes are disallowed.
	// - Removing racks is not supported.
	// - Reordering the rack list is not supported.
	// - Any new racks must be added to the end of the current rack list.

	oldRacks := oldDc.GetRacks()
	newRacks := newDc.GetRacks()

	if len(oldRacks) > len(newRacks) {
		return attemptedTo("remove rack")
	}

	newRackCount := len(newRacks) - len(oldRacks)
	if newRackCount > 0 {
		newSizeDifference := newDc.Spec.Size - oldDc.Spec.Size
		oldRackNodeSplit := SplitRacks(int(oldDc.Spec.Size), len(oldRacks))
		minNodesFromOldRacks := oldRackNodeSplit[len(oldRackNodeSplit)-1]
		minSizeAdjustment := minNodesFromOldRacks * newRackCount

		if newSizeDifference <= 0 {
			return attemptedTo("add rack without increasing size")
		}

		if int(newSizeDifference) < minSizeAdjustment {
			return attemptedTo(
				fmt.Sprintf("add racks without increasing size enough to prevent existing"+
					" nodes from moving to new racks to maintain balance.\n"+
					"New racks added: %d, size increased by: %d. Expected size increase to be at least %d",
					newRackCount, newSizeDifference, minSizeAdjustment))
		}
	}

	for index, oldRack := range oldRacks {
		newRack := newRacks[index]
		if oldRack.Name != newRack.Name {
			return attemptedTo("change rack name from '%s' to '%s'",
				oldRack.Name,
				newRack.Name)
		}
		if oldRack.Zone != newRack.Zone {
			return attemptedTo("change rack zone from '%s' to '%s'",
				oldRack.Zone,
				newRack.Zone)
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-cassandradatacenter,mutating=false,failurePolicy=ignore,groups=cassandra.datastax.com,resources=cassandradatacenters,verbs=create;update,versions=v1beta1,name=validate-cassandradatacenter-webhook
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
