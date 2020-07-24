// Copyright DataStax, Inc.
// Please see the included license file for details.

package nodeport_service

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "NodePort Service"
	namespace    = "test-node-port-service"
	dcName       = "dc1"
	dcYaml       = "../testdata/nodeport-service-dc.yaml"
	operatorYaml = "../testdata/operator.yaml"
	// dcResource   = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	// dcLabel      = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	//	additionalSeedServiceResource  = "services/cluster1-dc1-additional-seed-service"
	//additionalSeedEndpointResource = "endpoints/cluster1-dc1-additional-seed-service"
	ns = ginkgo_util.NewWrapper(testName, namespace)
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
		Specify("the operator can properly create a nodeport service", func() {
			var step string
			var k kubectl.KCmd

			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step = "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with a nodeport service"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)
		})
	})
})
