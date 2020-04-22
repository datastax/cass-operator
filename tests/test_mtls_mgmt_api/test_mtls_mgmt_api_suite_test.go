// Copyright DataStax, Inc.
// Please see the included license file for details.

package test_mtls_mgmt_api

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "test mtls protecting mgmt api"
	namespace    = "test-mtls-for-mgmt-api"
	dcName       = "dc1"
	dcYaml       = "../testdata/dse-one-node-dc-with-mtls.yaml"
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
		Specify("the operator can start, scale up, and terminate a datacenter where the mgmt api is behind mtls", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			// jam in secrets
			step = "creating mtls secrets"
			k := kubectl.ApplyFiles(
				"../testdata/mtls-certs-server.yaml",
				"../testdata/mtls-certs-client.yaml",
			).InNamespace(namespace)
			ns.ExecAndLog(step, k)

			step = "creating a datacenter resource with 1 rack/1 node"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			step = "scale up to 2 nodes"
			json := "{\"spec\": {\"size\": 2}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterOperatorProgress(dcName, "Updating", 30)
			ns.WaitForDatacenterReady(dcName)

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
