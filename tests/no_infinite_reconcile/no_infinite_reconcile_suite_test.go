// Copyright DataStax, Inc.
// Please see the included license file for details.

package no_infinite_reconcile

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName                       = "No Infinite Reconcile"
	namespace                      = "test-no-infinite-reconcile"
	dcName                         = "dc1"
	dcYaml                         = "../testdata/default-three-rack-three-node-dc.yaml"
	dcResource                     = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel                        = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns                             = ginkgo_util.NewWrapper(testName, namespace)
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
		Specify("the operator eventually stops reconciling", func() {
			var step string
			var k kubectl.KCmd

			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step = "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			ns.ExpectDoneReconciling(dcName)
		})
	})
})
