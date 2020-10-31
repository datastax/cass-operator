// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	reapergo "github.com/jsanda/reaper-client-go/reaper"
)

// If Reaper support is enabled via .Spec.Reaper.Enabled, this function performs a couple
// checks. First it checks the CassandraDatacenter's conditions to see if the C* cluster
// has already been registered with Reaper. If it has been registered, it then attempts to
// query Reaper to see if the cluster is still registered. The CassandraDatacenter's status
// conditions are updated accordingly.
func (rc *ReconciliationContext) CheckReaperStatus() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_reaper::CheckReaperStatus")
	dc := rc.Datacenter

	if !dc.IsReaperEnabled() {
		result.Continue()
	}

	statusUpdated := false
	dcPatch := client.MergeFrom(dc.DeepCopy())
	defer func() {
		if statusUpdated {
			err := rc.Client.Status().Patch(rc.Ctx, dc, dcPatch)
			if err != nil {
				rc.ReqLogger.Error(err, "error patching datacenter status for Reaper check")
			}
		}
	}()

	if condition, found := dc.GetCondition(api.DatacenterReaperManage); found && condition.Status == corev1.ConditionTrue {
		reaperSvc := getReaperServiceName(dc)
		restClient, err := reapergo.NewReaperClient(fmt.Sprintf("http://%s:8080", reaperSvc))
		if err != nil {
			rc.ReqLogger.Error(err, "failed to create Reaper REST client", "ReaperService", reaperSvc)
			statusUpdated = rc.setCondition(api.NewDatacenterConditionWithReason(
				api.DatacenterReaperManage,
				corev1.ConditionUnknown,
				"",
				"Failed to query Reaper"))
			return result.Continue()
		}

		cluster, err := restClient.GetCluster(rc.Ctx, dc.Spec.ClusterName)
		if err != nil {
			rc.ReqLogger.Error(err, "failed to query Reaper for cluster status")
			statusUpdated = rc.setCondition(api.NewDatacenterConditionWithReason(
				api.DatacenterReaperManage,
				corev1.ConditionUnknown,
				"",
				"Failed to query Reaper"))
			return result.Continue()
		}

		if cluster == nil {
			statusUpdated = rc.setCondition(api.NewDatacenterCondition(api.DatacenterReaperManage, corev1.ConditionFalse))
		}
	}
	return result.Continue()
}

// Makes sure that the cluster is registered with Reaper. It is assumed that this cluster
// is also the backend storage for Reaper. Reaper won't be ready and won't be able to serve
// REST api calls until schema initialization in its backend storage is complete (chicken
// meet egg). To avoid a lot of thrashing this function first checks that Reaper is ready.
// It will requeue the reconciliation request until Reaper is ready.
func (rc *ReconciliationContext) CheckRegisteredWithReaper() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_reaper::CheckRegisteredWithReaper")

	dc := rc.Datacenter

	if !rc.Datacenter.IsReaperEnabled() {
		return result.Continue()
	}

	if condition, found := dc.GetCondition(api.DatacenterReaperManage); found && condition.Status == corev1.ConditionTrue {
		return result.Continue()
	}

	statusUpdated := false
	dcPatch := client.MergeFrom(dc.DeepCopy())
	defer func() {
		if statusUpdated {
			err := rc.Client.Status().Patch(rc.Ctx, dc, dcPatch)
			if err != nil {
				rc.ReqLogger.Error(err, "error patching datacenter status for Reaper check")
			}
		}
	}()

	reaperSvc := getReaperServiceName(dc)
	// TODO Do not hard code the port
	restClient, err := reapergo.NewReaperClient(fmt.Sprintf("http://%s:8080", reaperSvc))
	if err != nil {
		rc.ReqLogger.Error(err, "failed to create Reaper REST client", "ReaperService", reaperSvc)
		statusUpdated = rc.setCondition(api.NewDatacenterCondition(api.DatacenterReaperManage, corev1.ConditionFalse))
		return result.Error(err)
	}

	if isReady, err := restClient.IsReaperUp(rc.Ctx); err == nil {
		if !isReady {
			rc.ReqLogger.Info("waiting for reaper to become ready")
			statusUpdated = rc.setCondition(api.NewDatacenterCondition(api.DatacenterReaperManage, corev1.ConditionFalse))
			return result.RequeueSoon(10)
		}
	} else {
		rc.ReqLogger.Error(err, "reaper readiness check failed")
		// We return result.RequestSoon here instead of result.Error because Reaper does not
		// start serving requests, including for its health check point, until schema
		// initialization has completed.
		statusUpdated = rc.setCondition(api.NewDatacenterCondition(api.DatacenterReaperManage, corev1.ConditionFalse))
		return result.RequeueSoon(30)
	}

	if err := restClient.AddCluster(rc.Ctx, rc.Datacenter.Spec.ClusterName,rc.Datacenter.GetDatacenterServiceName()); err == nil {
		statusUpdated = rc.setCondition(api.NewDatacenterCondition(api.DatacenterReaperManage, corev1.ConditionTrue))
		return result.Continue()
	} else {
		rc.ReqLogger.Error(err,"failed to register cluster with Reaper")
		statusUpdated = rc.setCondition(api.NewDatacenterConditionWithReason(
			api.DatacenterReaperManage,
			corev1.ConditionFalse,
			"",
			err.Error()))
		return result.Error(err)
	}
}

func getReaperServiceName(dc *api.CassandraDatacenter) string {
	if len(dc.Spec.Reaper.Namespace) == 0 {
		return dc.Spec.Reaper.Name + "-reaper-service"
	}
	return dc.Spec.Reaper.Name + "-reaper-service" + "." + dc.Spec.Reaper.Namespace
}
