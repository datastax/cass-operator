// Copyright DataStax, Inc.
// Please see the included license file for details.

package stop_resume

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "Stop and Resume"
	namespace    = "test-stop-resume"
	dcName       = "dc1"
	dcYaml       = "../testdata/default-three-rack-three-node-dc.yaml"
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
		Specify("the operator can stop, resume, and terminate a datacenter", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			step = "stopping the dc"
			json := "{\"spec\": {\"stopped\": true}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			// Ensure conditions set correctly for stopped
			ns.WaitForDatacenterCondition(dcName, "Stopped", string(corev1.ConditionTrue)) 
			ns.WaitForDatacenterCondition(dcName, "Ready", string(corev1.ConditionFalse))

			step = "checking the spec size hasn't changed"
			json = "jsonpath={.spec.size}"
			k = kubectl.Get(dcResource).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "3", 20)

			ns.WaitForDatacenterToHaveNoPods(dcName)

			step = "resume the dc"
			json = "{\"spec\": {\"stopped\": false}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			// Ensure conditions set correctly for resuming
			ns.WaitForDatacenterCondition(dcName, "Stopped", string(corev1.ConditionFalse))
			ns.WaitForDatacenterCondition(dcName, "Resuming", string(corev1.ConditionTrue))

			ns.WaitForDatacenterReady(dcName)
			ns.WaitForDatacenterCondition(dcName, "Ready", string(corev1.ConditionTrue))

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
