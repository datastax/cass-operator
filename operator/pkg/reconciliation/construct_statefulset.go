// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

// This file defines constructors for k8s statefulset-related objects

import (
	"fmt"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/images"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/pkg/psp"
	"github.com/datastax/cass-operator/operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const ZONE_LABEL = "failure-domain.beta.kubernetes.io/zone"

func usesDefunctPvcManagedByLabel(sts *appsv1.StatefulSet) bool {
	usesDefunct := false
	for _, pvc := range sts.Spec.VolumeClaimTemplates {
		value, ok := pvc.Labels[oplabels.ManagedByLabel]
		if ok && value == oplabels.ManagedByLabelDefunctValue {
			usesDefunct = true
			break
		}
	}

	return usesDefunct
}

func newNamespacedNameForStatefulSet(
	dc *api.CassandraDatacenter,
	rackName string) types.NamespacedName {

	name := dc.Spec.ClusterName + "-" + dc.Name + "-" + rackName + "-sts"
	ns := dc.Namespace

	return types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
}

// We have to account for the fact that they might use the old managed-by label value
// (oplabels.ManagedByLabelDefunctValue) for CassandraDatacenters originally
// created in version 1.1.0 or earlier.
func newStatefulSetForCassandraDatacenterWithDefunctPvcManagedBy(
	rackName string,
	dc *api.CassandraDatacenter,
	replicaCount int) (*appsv1.StatefulSet, error) {

	return newStatefulSetForCassandraDatacenterHelper(rackName, dc, replicaCount, true)
}

func newStatefulSetForCassandraDatacenter(
	rackName string,
	dc *api.CassandraDatacenter,
	replicaCount int) (*appsv1.StatefulSet, error) {

	return newStatefulSetForCassandraDatacenterHelper(rackName, dc, replicaCount, false)
}

// Check if we need to define a SecurityContext.
// If the user defines the DockerImageRunsAsCassandra field, we trust that.
// Otherwise if ServerType is "dse", the answer is true.
// Otherwise we use the logic in CalculateDockerImageRunsAsCassandra
// to calculate a reasonable answer.
func shouldDefineSecurityContext(dc *api.CassandraDatacenter) bool {
	// The override field always wins
	if dc.Spec.DockerImageRunsAsCassandra != nil {
		return *dc.Spec.DockerImageRunsAsCassandra
	}

	return dc.Spec.ServerType == "dse" || images.CalculateDockerImageRunsAsCassandra(dc.Spec.ServerVersion)
}

func rackNodeAffinitylabels(dc *api.CassandraDatacenter, rackName string) (nodeAffinityLabels map[string]string,
	Error error) {
	racks := dc.GetRacks()
	for _, rack := range racks {
		if rack.Name == rackName {
			nodeAffinityLabels = utils.MergeMap(emptyMapIfNil(rack.NodeAffinityLabels),
				emptyMapIfNil(dc.Spec.NodeAffinityLabels))
			if rack.Zone != "" {
				if _, found := nodeAffinityLabels[ZONE_LABEL]; found {
					err := fmt.Errorf("Deprecated parameter Zone is used and also defined in " +
						"NodeAffinityLabels. You should only define it in NodeAffinityLabels")
					return nil, err
				}
				nodeAffinityLabels = utils.MergeMap(
					emptyMapIfNil(nodeAffinityLabels), map[string]string{ZONE_LABEL: rack.Zone},
					)
			}
			break
		}
	}
	return
}

// Create a statefulset object for the Datacenter.
func newStatefulSetForCassandraDatacenterHelper(
	rackName string,
	dc *api.CassandraDatacenter,
	replicaCount int,
	useDefunctManagedByForPvc bool) (*appsv1.StatefulSet, error) {

	replicaCountInt32 := int32(replicaCount)

	// see https://github.com/kubernetes/kubernetes/pull/74941
	// pvc labels are ignored before k8s 1.15.0
	pvcLabels := dc.GetRackLabels(rackName)
	if useDefunctManagedByForPvc {
		oplabels.AddDefunctManagedByLabel(pvcLabels)
	} else {
		oplabels.AddManagedByLabel(pvcLabels)
	}

	statefulSetLabels := dc.GetRackLabels(rackName)
	oplabels.AddManagedByLabel(statefulSetLabels)

	statefulSetSelectorLabels := dc.GetRackLabels(rackName)

	var volumeClaimTemplates []corev1.PersistentVolumeClaim

	nodeAffinityLabels, nodeAffinityLabelsConfigurationError := rackNodeAffinitylabels(dc, rackName)
	if nodeAffinityLabelsConfigurationError != nil {
		return nil, nodeAffinityLabelsConfigurationError
	}

	// Add storage
	if dc.Spec.StorageConfig.CassandraDataVolumeClaimSpec == nil {
		err := fmt.Errorf("StorageConfig.cassandraDataVolumeClaimSpec is required")
		return nil, err
	}

	volumeClaimTemplates = []corev1.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: pvcLabels,
			Name:   PvcName,
		},
		Spec: *dc.Spec.StorageConfig.CassandraDataVolumeClaimSpec,
	}}

	nsName := newNamespacedNameForStatefulSet(dc, rackName)

	template, err := buildPodTemplateSpec(dc, nodeAffinityLabels, rackName)
	if err != nil {
		return nil, err
	}

	// if the dc.Spec has a nodeSelector map, copy it into each sts pod template
	if len(dc.Spec.NodeSelector) > 0 {
		template.Spec.NodeSelector = utils.MergeMap(map[string]string{}, dc.Spec.NodeSelector)
	}

	// workaround for https://cloud.google.com/kubernetes-engine/docs/security-bulletins#may-31-2019

	if shouldDefineSecurityContext(dc) {
		var userID int64 = 999
		template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser:  &userID,
			RunAsGroup: &userID,
			FSGroup:    &userID,
		}
	}

	_ = httphelper.AddManagementApiServerSecurity(dc, template)

	result := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsName.Name,
			Namespace: nsName.Namespace,
			Labels:    statefulSetLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: statefulSetSelectorLabels,
			},
			Replicas:             &replicaCountInt32,
			ServiceName:          dc.GetDatacenterServiceName(),
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			Template:             *template,
			VolumeClaimTemplates: volumeClaimTemplates,
		},
	}
	result.Annotations = map[string]string{}

	if utils.IsPSPEnabled() {
		result = psp.AddStatefulSetChanges(dc, result)
	}

	// add a hash here to facilitate checking if updates are needed
	utils.AddHashAnnotation(result)

	return result, nil
}
