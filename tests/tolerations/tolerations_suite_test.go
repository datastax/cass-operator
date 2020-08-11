// Copyright DataStax, Inc.
// Please see the included license file for details.

package tolerations

import (
	"fmt"
	"testing"
	//"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "Tolerations"
	opNamespace  = "test-tolerations"
	dc1Name      = "dc2"
	dc1Yaml      = "../testdata/default-single-rack-2-node-dc.yaml"
	dc1Resource  = fmt.Sprintf("CassandraDatacenter/%s", dc1Name)
	pod1Name     = "cluster2-dc2-r1-sts-0"
	pod1Resource = fmt.Sprintf("pod/%s", pod1Name)
	ns           = ginkgo_util.NewWrapper(testName, opNamespace)
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
		Specify("the operator can build pods with tolerations", func() {

			By("creating a namespace for the cass-operator")
			err := kubectl.CreateNamespace(opNamespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating first datacenter resource"
			k := kubectl.ApplyFiles(dc1Yaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dc1Name)

			/*
				// Add a taint to the node
				k = kubectl.GetNodeNameForPod(pod1Name)
				node1Name, _, err := ns.ExecVCapture(k)
				if err != nil {
					panic(err)
				}
				node1Resource := fmt.Sprintf("node/%s", node1Name)

				// node.vmware.com/drain=planned-downtime:NoSchedule
				step = fmt.Sprintf("tainting node: %s", node1Name)
				k = kubectl.Taint(
					node1Name,
					"node.vmware.com/drain",
					"planned-downtime",
					"NoSchedule")
				ns.ExecAndLog(step, k)

				time.Sleep(5 * time.Second)

				json := `
				{
					"spec": {
						"taints": null
					}
				}`
				k = kubectl.PatchMerge(node1Resource, json)
				err = k.ExecV()
				if err != nil {
					panic(err)
				}
			*/
		})
	})
})
