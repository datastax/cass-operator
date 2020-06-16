// Copyright DataStax, Inc.
// Please see the included license file for details.

package test_bad_config_and_fix

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName   = "test rolling out a bad config and fixing it"
	namespace  = "test-bad-config-and-fix"
	dcName     = "dc1"
	dcYaml     = "../testdata/oss-three-rack-three-node-dc.yaml"
	dcResource = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel    = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns         = ginkgo_util.NewWrapper(testName, namespace)
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
		Specify("the operator can scale up, stop, resume, and terminate a datacenter", func() {
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
			ns.WaitForDatacenterCondition(dcName, "Ready", string(corev1.ConditionTrue))
			ns.WaitForDatacenterCondition(dcName, "Initialized", string(corev1.ConditionTrue))

			step = "apply a bad image"
			json := "{\"spec\": {\"serverImage\": \"datastax/cassandra-v314159\"}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterOperatorProgress(dcName, "Updating", 30)

			time.Sleep(time.Minute * 6)
			ns.WaitForDatacenterReadyPodCount(dcName, 2)

			step = "apply a good image"
			json = "{\"spec\": {\"serverImage\": \"\"}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "set the forceUpgradeRacks config"
			json = "{\"spec\": {\"forceUpgradeRacks\": [\"r1\"]}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			step = "checking that statefulsets have the right owner reference"
			json = "jsonpath={.metadata.ownerReferences[0].name}"
			k = kubectl.Get("sts/cluster1-dc1-r1-sts").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "dc1", 30)

			step = "deleting the dc"
			k = kubectl.DeleteFromFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
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
		})
	})
})
