// Copyright DataStax, Inc.
// Please see the included license file for details.

package preserve_client_ips

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName   = "Preserve client source IPs"
	namespace  = "test-preserve-client-ips"
	dcName     = "dc1"
	dcYaml     = "../testdata/preserve-client-ips-dc.yaml"
	dcResource = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel    = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns         = ginkgo_util.NewWrapper(testName, namespace)
)

func TestLifecycle(t *testing.T) {
	AfterSuite(func() {
		logPath := fmt.Sprintf("%s/aftersuite", ns.LogDir)
		err := kubectl.DumpAllLogs(logPath).ExecV()
		if err != nil {
			fmt.Printf("\n\tError during dumping logs: %s\n\n", err.Error())
		}
		fmt.Printf("\n\tPost-run logs dumped at: %s\n\n", logPath)
		ns.Terminate()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, testName)
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator can scale up a datacenter", func() {
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

			step = "deleting the dc"
			k = kubectl.DeleteFromFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "checking that the cassdc no longer exists"
			json := "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 60)

			step = "checking that the statefulsets no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("StatefulSet").
				WithLabel(dcLabel).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 60)
		})
	})
})
