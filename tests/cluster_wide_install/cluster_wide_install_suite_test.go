// Copyright DataStax, Inc.
// Please see the included license file for details.

package cluster_wide_install

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	helm_util "github.com/datastax/cass-operator/mage/helm"
	"github.com/datastax/cass-operator/mage/kubectl"
	mageutil "github.com/datastax/cass-operator/mage/util"
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
	dc2Resource  = fmt.Sprintf("CassandraDatacenter/%s", dc2Name)
	ns           = ginkgo_util.NewWrapper(testName, opNamespace)
	ns1          = ginkgo_util.NewWrapper(testName, dcNamespace1)
	ns2          = ginkgo_util.NewWrapper(testName, dcNamespace2)
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

			var overrides = map[string]string{
				"image":              cfgutil.GetOperatorImage(),
				"clusterWideInstall": "true",
			}
			chartPath := "../../charts/cass-operator-chart"
			err = helm_util.Install(chartPath, "cass-operator", ns.Namespace, overrides)
			mageutil.PanicOnError(err)

			ns.WaitForOperatorReady()

			By("creating a namespace for the first dc")
			err = kubectl.CreateNamespace(dcNamespace1).ExecV()
			Expect(err).ToNot(HaveOccurred())

			By("creating a namespace for the second dc")
			err = kubectl.CreateNamespace(dcNamespace2).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "creating first datacenter resource"
			k := kubectl.ApplyFiles(dc1Yaml)
			ns1.ExecAndLog(step, k)

			step = "creating second datacenter resource"
			k = kubectl.ApplyFiles(dc2Yaml)
			ns2.ExecAndLog(step, k)

			ns1.WaitForDatacenterReady(dc1Name)
			ns2.WaitForDatacenterReady(dc2Name)

			step = "scale first dc up to 2 nodes"
			json := `{"spec": {"size": 2}}`
			k = kubectl.PatchMerge(dc1Resource, json)
			ns1.ExecAndLog(step, k)

			ns1.WaitForDatacenterOperatorProgress(dc1Name, "Updating", 30)
			ns1.WaitForDatacenterReady(dc1Name)

			step = "scale second dc up to 2 nodes"
			json = `{"spec": {"size": 2}}`
			k = kubectl.PatchMerge(dc2Resource, json)
			ns2.ExecAndLog(step, k)

			ns2.WaitForDatacenterOperatorProgress(dc2Name, "Updating", 30)
			ns2.WaitForDatacenterReady(dc2Name)

			By("deleting a namespace for the first dc")
			err = kubectl.DeleteNamespace(dcNamespace1).ExecV()
			Expect(err).ToNot(HaveOccurred())

			By("deleting a namespace for the second dc")
			err = kubectl.DeleteNamespace(dcNamespace2).ExecV()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
