// Copyright DataStax, Inc.
// Please see the included license file for details.

package multi_dc_cluster

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName  = "Cluster resource cleanup after termination"
	namespace = "test-multi-dc-cluster"

	dcNames = []string{"dc1", "dc2"}
	dcYamls = []string{"../testdata/dse-multi-dc-1-rack-1-node-dc1.yaml",
		"../testdata/dse-multi-dc-1-rack-1-node-dc2.yaml"}
	dcResources = []string{fmt.Sprintf("CassandraDatacenter/%s", dcNames[0]),
		fmt.Sprintf("CassandraDatacenter/%s", dcNames[1])}
	dcLabels = []string{fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcNames[0]),
		fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcNames[1])}

	operatorYaml     = "../testdata/operator.yaml"
	ns               = ginkgo_util.NewWrapper(testName, namespace)
	defaultResources = []string{
		"../../operator/deploy/role.yaml",
		"../../operator/deploy/role_binding.yaml",
		"../../operator/deploy/service_account.yaml",
		"../../operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml",
	}
)

func TestLifecycle(t *testing.T) {
	AfterSuite(func() {
		logPath := fmt.Sprintf("%s/aftersuite", ns.LogDir)
		kubectl.DumpAllLogs(logPath).ExecV()
		fmt.Printf("\n\tPost-run logs dumped at: %s\n\n", logPath)
		_ = ns.Terminate()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, testName)
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator can stand up a multi-dc cluster", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "creating default resources"
			k := kubectl.ApplyFiles(defaultResources...)
			ns.ExecAndLog(step, k)

			step = "creating the cass-operator resource"
			k = kubectl.ApplyFiles(operatorYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the operator to become ready"
			json := "jsonpath={.items[0].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel("name=cass-operator").
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 120)

			step = "creating the first dc resource with 1 racks/1 nodes"
			k = kubectl.ApplyFiles(dcYamls[0])
			ns.ExecAndLog(step, k)

			step = "waiting for the first dc node to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabels[0]).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 1200)

			step = "checking the cassandra operator progress status is set to Ready"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResources[0]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "creating the second dc resource with 1 racks/1 nodes"
			k = kubectl.ApplyFiles(dcYamls[1])
			ns.ExecAndLog(step, k)

			step = "waiting for the second dc node to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabels[1]).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 1200)

			step = "checking the cassandra operator progress status is set to Ready"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResources[1]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "deleting the dcs"
			k = kubectl.DeleteFromFiles(dcYamls...)
			ns.ExecAndLog(step, k)

			step = "checking that the first dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabels[0]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that the second dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabels[1]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no pods remain for the first dc"
			json = "jsonpath={.items}"
			k = kubectl.Get("pods").
				WithLabel(dcLabels[0]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no pods remain for the second dc"
			json = "jsonpath={.items}"
			k = kubectl.Get("pods").
				WithLabel(dcLabels[1]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no services for the first dc"
			json = "jsonpath={.items}"
			k = kubectl.Get("services").
				WithLabel(dcLabels[0]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no services for the second dc"
			json = "jsonpath={.items}"
			k = kubectl.Get("services").
				WithLabel(dcLabels[1]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no stateful sets remain for the first dc"
			json = "jsonpath={.items}"
			k = kubectl.Get("statefulsets").
				WithLabel(dcLabels[0]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no stateful sets remain for the second dc"
			json = "jsonpath={.items}"
			k = kubectl.Get("statefulsets").
				WithLabel(dcLabels[1]).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)
		})
	})
})
