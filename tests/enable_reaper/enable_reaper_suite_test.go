// Copyright DataStax, Inc.
// Please see the included license file for details.

package enable_reaper

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "Enable Reaper"
	namespace    = "test-enable-reaper"
	dcName       = "dc1"
	dcYaml       = "../testdata/oss-three-rack-three-node-dc.yaml"
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
		Specify("the operator can scale up and enable Reaper and then disable Reaper", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			step = "setting up reaper-operator"
			k := kubectl.ApplyFiles("../testdata/reaper-operator-bundle.yaml")
			ns.ExecAndLog(step, k)

			ns.WaitForOperatorReady()
			ns.WaitForReaperOperatorReady()

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForSuperUserUpserted(dcName, 600)

			step = "check recorded host IDs"
			nodeStatusesHostIds := ns.GetNodeStatusesHostIds(dcName)
			Expect(len(nodeStatusesHostIds), 3)

			ns.WaitForDatacenterReady(dcName)
			ns.WaitForDatacenterCondition(dcName, "Ready", string(corev1.ConditionTrue))
			ns.WaitForDatacenterCondition(dcName, "Initialized", string(corev1.ConditionTrue))

			// We need to enable reaper in the CassandraDatacenter before deploying Reaper because
			// cass-operator creates the JMX secret that Reaper uses. The Reaper object cannot be
			// created without that secret. Alternatively, we could create the secret instead of
			// letting cass-operator generate it. Then the order of steps is less important.
			step = "enable Reaper"
			json := `{"spec": {"reaper": {"enabled": true, "service": "reaper-cassdc-reaper-service"}}}`
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterOperatorProgress(dcName, "Updating", 120)
			ns.WaitForDatacenterOperatorProgress(dcName, "Ready", 1200)

			step = "deploy reaper"
			k = kubectl.ApplyFiles("../testdata/reaper.yaml")
			ns.ExecAndLog(step, k)

			ns.WaitForReaperReady("reaper-cassdc", 300)

			ns.WaitForDatacenterCondition(dcName, "Reaper", "True")

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
