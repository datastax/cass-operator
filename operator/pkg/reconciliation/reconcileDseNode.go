package reconciliation

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
)

func refreshSeeds(rc *dsereconciliation.ReconciliationContext) error {
	rc.ReqLogger.Info("reconcileDseNode::refreshSeeds")
	if rc.DseDatacenter.Spec.Parked {
		rc.ReqLogger.Info("cluster is parked, skipping refreshSeeds")
		return nil
	}

	selector := rc.DseDatacenter.GetClusterLabels()
	selector[datastaxv1alpha1.DseNodeState] = "Started"

	podList, err := listPods(rc, selector)
	if err != nil {
		rc.ReqLogger.Error(err, "error listing pods during refreshSeeds")
		return err
	}

	for _, pod := range podList.Items {
		if err := rc.NodeMgmtClient.CallReloadSeedsEndpoint(&pod); err != nil {
			return err
		}
	}

	return nil
}

func listPods(rc *dsereconciliation.ReconciliationContext, selector map[string]string) (*corev1.PodList, error) {
	rc.ReqLogger.Info("reconcileDseNode::listPods")

	listOptions := &client.ListOptions{
		Namespace:     rc.DseDatacenter.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	podList := &corev1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}

	return podList, rc.Client.List(rc.Ctx, podList, listOptions)
}
