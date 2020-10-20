// Copyright DataStax, Inc.
// Please see the included license file for details.

package delete_node_allow_disable

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "Disable deletion of node that lost readiness and isn't becoming ready"
	namespace    = "test-disable-delete-node-lost-readiness"
	dcName       = "dc1"
	dcYaml       = "../testdata/default-three-rack-three-node-disable-delete-stuck-nodes.yaml"
	operatorYaml = "../testdata/operator.yaml"
	dcResource   = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel      = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns           = ginkgo_util.NewWrapper(testName, namespace)
)

func TestLifecycle(t *testing.T) {
	AfterSuite(func() {
		logPath := fmt.Sprintf("%s/aftersuite", ns.LogDir)
		kubectl.DumpAllLogs(logPath).ExecV()
		fmt.Printf("\n\tPost-run logs dumped at: %s\n\n", logPath)
		ns.Terminate()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, testName)
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator can detect a node that lost readiness and is hanging, and doesn't delete the pod when is configured not to", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			podNames := ns.GetDatacenterPodNames(dcName)
			podName := podNames[0]

			step = "verifying that the pod is labeled as Started"
			json := `jsonpath={.metadata.labels.cassandra\.datastax\.com/node-state}`
			k = kubectl.GetByTypeAndName("pod", podName).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Started", 120)

			ns.DisableGossip(podName)

			step = "verifying that the pod lost readiness"
			json = "jsonpath={.status.containerStatuses[0].ready}"
			k = kubectl.GetByTypeAndName("pod", podName).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "false", 60)

			// allow a total of 11 minutes to pass to verify that the pod doesn't get deleted
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)
			waitOneMinuteAndVerifyPodNeverTerminates(podName)

			step = "deleting the dc"
			k = kubectl.DeleteFromFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabel).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)
		})
	})
})

func waitOneMinuteAndVerifyPodNeverTerminates(podName string) {
	time.Sleep(time.Second * 60)
	By("waiting one minute and checking that pod didn't terminate")
	json := "jsonpath={.metadata.deletionTimestamp}"
	k := kubectl.GetByTypeAndName("pod", podName).
		FormatOutput(json)
	ns.WaitForOutput(k, "", 5)
}
