// Copyright DataStax, Inc.
// Please see the included license file for details.

package additional_serviceoptions

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName        = "Additional service options"
	namespace       = "test-additional-service-options"
	clusterName     = "cluster2"
	dcName          = "dc2"
	dcYaml          = "../testdata/additional-service-annotations-and-labels.yaml"
	operatorYaml    = "../testdata/operator.yaml"
	dcResource      = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel         = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	serviceResource = fmt.Sprintf("services/%s-%s-service", clusterName, dcName)
	ns              = ginkgo_util.NewWrapper(testName, namespace)
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
		Specify("the operator can create a datacenter where services have additional properties", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 1 rack/1 node"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReadyWithTimeouts(dcName, 600, 120)

			step = "verify service has additional label and annotation"
			k = kubectl.Get("svc").WithLabel("test=additional-serviceoptions").FormatOutput(`jsonpath={.items[0].metadata.annotations.external-dns\.alpha\.kubernetes\.io/hostname}`)
			ns.WaitForOutputAndLog(step, k, "localhost", 300)

			step = "deleting the dc"
			k = kubectl.DeleteFromFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
			json := "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabel).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)
		})
	})
})
