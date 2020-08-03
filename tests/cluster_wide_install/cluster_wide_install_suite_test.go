// Copyright DataStax, Inc.
// Please see the included license file for details.

package cluster_wide_install

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

// Note: the cass-operator itself will be installed in the test-cluster-wide-install namespace
// The two test dcs will be setup in test-cluster-wide-install-ns1 and test-cluster-wide-install-ns2

var (
	testName     = "Cluster-wide install"
	opNamespace  = "test-cluster-wide-install"
	dcNamespace1 = "test-cluster-wide-install-ns1"
	dcNamespace2 = "test-cluster-wide-install-ns2"
	dc1Name      = "dc1"
	dc1Yaml      = "../testdata/cluster-wide-install-dc1.yaml"
	dc2Name      = "dc2"
	dc2Yaml      = "../testdata/cluster-wide-install-dc2.yaml"
	dc1Resource  = fmt.Sprintf("CassandraDatacenter/%s", dc1Name)
	dc1Label     = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dc1Name)
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
		Specify("the operator can monitor multiple namespaces", func() {
			By("creating a namespace for the cass-operator")
			err := kubectl.CreateNamespace(opNamespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			//step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			By("creating a namespace for the first dc")
			err = kubectl.CreateNamespace(dcNamespace1).ExecV()
			Expect(err).ToNot(HaveOccurred())

			By("creating a namespace for the second dc")
			err = kubectl.CreateNamespace(dcNamespace2).ExecV()
			Expect(err).ToNot(HaveOccurred())

			By("deleting a namespace for the first dc")
			err = kubectl.DeleteNamespace(dcNamespace1).ExecV()
			Expect(err).ToNot(HaveOccurred())

			By("deleting a namespace for the second dc")
			err = kubectl.DeleteNamespace(dcNamespace2).ExecV()
			Expect(err).ToNot(HaveOccurred())

			/*
				step = "creating a datacenter resource with 3 racks/3 nodes"
				k := kubectl.ApplyFiles(dcYaml)
				ns.ExecAndLog(step, k)

				ns.WaitForDatacenterReady(dcName)

				step = "scale up to 6 nodes"
				json := `{"spec": {"size": 6}}`
				k = kubectl.PatchMerge(dcResource, json)
				ns.ExecAndLog(step, k)

				ns.WaitForDatacenterOperatorProgress(dcName, "Updating", 30)
				ns.WaitForDatacenterReady(dcName)

				step = "deleting the dc"
				k = kubectl.DeleteFromFiles(dcYaml)
				ns.ExecAndLog(step, k)

				step = "checking that the cassdc no longer exists"
				json = "jsonpath={.items}"
				k = kubectl.Get("CassandraDatacenter").
					FormatOutput(json)
				ns.WaitForOutputAndLog(step, k, "[]", 60)

				step = "checking that the statefulsets no longer exists"
				json = "jsonpath={.items}"
				k = kubectl.Get("StatefulSet").
					WithLabel(dcLabel).
					FormatOutput(json)
				ns.WaitForOutputAndLog(step, k, "[]", 60)
			*/
		})
	})
})
