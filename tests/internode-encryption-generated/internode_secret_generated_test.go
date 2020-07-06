// Copyright DataStax, Inc.
// Please see the included license file for details.

package internode_secret_generated

import (
	"fmt"
	"testing"


	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName          = "Internode Secret Generated"
	namespace         = "test-internode-secret-generated"
	defaultSecretName = "dc2-keystore"
	secretResource    = fmt.Sprintf("secret/%s", defaultSecretName)
	dcName            = "dc2"
	dcYaml            = "../testdata/encrypted-single-rack-2-node-dc.yaml"
	operatorYaml      = "../testdata/operator.yaml"
	dcResource        = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel           = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns                = ginkgo_util.NewWrapper(testName, namespace)
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
	Context("when in a new cluster where internodeSecretName is unspecified", func() {
		Specify("the operator generates an appropriate internode secret", func() {
			var step string
			var k kubectl.KCmd

			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step = "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 1 racks/2 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			// verify the secret was created
			step = "check that the internode secret was created"
			k = kubectl.Get(secretResource)
			ns.ExecAndLog(step, k)
		})
	})
})
