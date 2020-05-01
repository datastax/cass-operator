// Copyright DataStax, Inc.
// Please see the included license file for details.

package add_racks

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "Add racks"
	namespace    = "test-add-racks"
	dcName       = "dc2"
	dcYaml       = "../testdata/default-single-rack-single-node-dc.yaml"
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
		Specify("racks can be added if the size is increased accordingly", func() {
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

			step = "creating a datacenter resource with 1 racks/1 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the node to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 1200)

			step = "checking the cassandra operator progress status is set to Ready"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResource).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "add 1 rack and increase size by 1"
			json = `{"spec": { "size": 2, "racks": [{"name": "r1"}, {"name": "r2"}]}}`
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "checking there is 1 node on the new rack"
			json = "jsonpath={.items[*].metadata.name}"
			k = kubectl.Get("pod").
				WithLabel(`cassandra.datastax.com/rack=r2`).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "cluster2-dc2-r2-sts-0", 60)

			step = "add 2 more racks and increase size by 2"
			json = `{"spec": { "size": 4, "racks": [{"name": "r1"}, {"name": "r2"}, {"name": "r3"}, {"name": "r4"}]}}`
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "checking there is 1 node on rack 3"
			json = "jsonpath={.items[*].metadata.name}"
			k = kubectl.Get("pod").
				WithLabel("cassandra.datastax.com/rack=r3").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "cluster2-dc2-r3-sts-0", 60)

			step = "checking there is 1 node on rack 4"
			json = "jsonpath={.items[*].metadata.name}"
			k = kubectl.Get("pod").
				WithLabel("cassandra.datastax.com/rack=r4").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "cluster2-dc2-r4-sts-0", 60)

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
