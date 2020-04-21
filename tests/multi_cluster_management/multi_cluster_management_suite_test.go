// Copyright DataStax, Inc.
// Please see the included license file for details.

package multi_cluster_management

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "Multi-cluster Management"
	namespace    = "test-multi-cluster-management"
	dcNames      = [2]string{"dc1", "dc2"}
	dcYamls      = [2]string{"../testdata/default-three-rack-three-node-dc.yaml", "../testdata/default-single-rack-single-node-dc.yaml"}
	operatorYaml = "../testdata/operator.yaml"
	ns           = ginkgo_util.NewWrapper(testName, namespace)
)

func dcResourceForName(dcName string) string {
	return fmt.Sprintf("CassandraDatacenter/%s", dcName)
}

func dcLabelForName(dcName string) string {
	return fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
}

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
		Specify("the operator manages multiple clusters in the same namespace", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			step = "waiting for the operator to become ready"
			json := "jsonpath={.items[0].status.containerStatuses[0].ready}"
			k := kubectl.Get("pods").
				WithLabel("name=cass-operator").
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 120)

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k = kubectl.ApplyFiles(dcYamls[0])
			ns.ExecAndLog(step, k)

			step = "creating another datacenter resource with 1 rack/1 node"
			k = kubectl.ApplyFiles(dcYamls[1])
			ns.ExecAndLog(step, k)

			step = "waiting for the node to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabelForName(dcNames[0])).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true true true", 1200)

			step = "checking the cassandra operator progress status is set to Ready for first dc"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResourceForName(dcNames[0])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "waiting for the nodes to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("Pods").
				WithLabel(dcLabelForName(dcNames[1])).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 1200)

			step = "checking the cassandra operator progress status is set to Ready for second dc"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResourceForName(dcNames[1])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "deleting the first dc"
			k = kubectl.DeleteFromFiles(dcYamls[0])
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabelForName(dcNames[0])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no dc pods remain"
			json = "jsonpath={.items}"
			k = kubectl.Get("pods").
				WithLabel(dcLabelForName(dcNames[0])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "deleting the second dc"
			k = kubectl.DeleteFromFiles(dcYamls[1])
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabelForName(dcNames[1])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no dc pods remain"
			json = "jsonpath={.items}"
			k = kubectl.Get("pods").
				WithLabel(dcLabelForName(dcNames[1])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)
		})
	})
})
