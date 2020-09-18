// Copyright DataStax, Inc.
// Please see the included license file for details.

package canary_upgrade

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
	testName     = "OSS test canary upgrade"
	namespace    = "test-canary-upgrade"
	dcName       = "dc1"
	dcYaml       = "../testdata/oss-upgrade-dc.yaml"
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
		Specify("the operator can perform a canary upgrade", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForSuperUserUpserted(dcName, 600)

			step = "check recorded host IDs"
			nodeStatusesHostIds := ns.GetNodeStatusesHostIds(dcName)
			Expect(len(nodeStatusesHostIds), 3)

			ns.WaitForDatacenterReady(dcName)
			ns.WaitForDatacenterCondition(dcName, "Initialized", string(corev1.ConditionTrue))

			step = "prepare for canary upgrade"
			json := "{\"spec\": {\"canaryUpgrade\": true, \"canaryUpgradeCount\": 1}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "perform canary upgrade"
			json = "{\"spec\": {\"serverVersion\": \"3.11.7\"}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterOperatorProgress(dcName, "Updating", 30)
			ns.WaitForDatacenterReadyPodCount(dcName, 3)

			images := []string{
				"datastax/cassandra-mgmtapi-3_11_6:v0.1.5",
				"datastax/cassandra-mgmtapi-3_11_6:v0.1.5",
				"datastax/cassandra-mgmtapi-3_11_7:v0.1.12",
			}
			ns.WaitForCassandraImages(dcName, images, 300)
			ns.WaitForDatacenterReadyPodCount(dcName, 3)

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