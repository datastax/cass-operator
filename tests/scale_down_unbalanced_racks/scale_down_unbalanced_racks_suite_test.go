// Copyright DataStax, Inc.
// Please see the included license file for details.

package scale_down_unbalanced_racks

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName   = "Scale down datacenter with unbalanced racks"
	namespace  = "test-scale-down-unbalanced-racks"
	dcName     = "dc1"
	dcYaml     = "../testdata/default-two-rack-four-node-dc.yaml"
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
		Specify("a datacenter can be scaled down with unbalanced racks", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 2 racks/4 nodes"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			step = "scale up second rack to 3 nodes"
			json := "{\"spec\": {\"replicas\": 3}}"
			k = kubectl.PatchMerge("sts/cluster1-dc1-r2-sts", json)
			ns.ExecAndLog(step, k)

			// because we scale up the rack before scaling down the dc,
			// the operator will wait to scale down until the extra rack
			// nodes are  ready, so we can safetly update the dc size
			// at this point
			step = "scale down dc to 2 nodes"
			json = "{\"spec\": {\"size\": 2}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			extraPod := "cluster1-dc1-r2-sts-2"

			step = "check that the extra pod is ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pod").
				WithFlag("field-selector", fmt.Sprintf("metadata.name=%s", extraPod)).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 360)

			// The rack with an extra node should get a decommission request
			// first, despite being the last rack and the first rack also needing
			// to eventually decommission nodes
			expectedRemainingPods := []string{
				"cluster1-dc1-r1-sts-0", "cluster1-dc1-r1-sts-1",
				"cluster1-dc1-r2-sts-0", "cluster1-dc1-r2-sts-1",
			}
			ensurePodGetsDecommissionedNext(extraPod, expectedRemainingPods)

			expectedRemainingPods = []string{
				"cluster1-dc1-r1-sts-0", "cluster1-dc1-r1-sts-1",
				"cluster1-dc1-r2-sts-0",
			}
			ensurePodGetsDecommissionedNext("cluster1-dc1-r2-sts-1", expectedRemainingPods)

			expectedRemainingPods = []string{
				"cluster1-dc1-r1-sts-0",
				"cluster1-dc1-r2-sts-0",
			}
			ensurePodGetsDecommissionedNext("cluster1-dc1-r1-sts-1", expectedRemainingPods)

			ns.WaitForDatacenterCondition(dcName, "ScalingDown", string(corev1.ConditionFalse))
			ns.WaitForDatacenterOperatorProgress(dcName, "Ready", 120)

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

func ensurePodGetsDecommissionedNext(podName string, expectedRemainingPods []string) {
	step := fmt.Sprintf("check that pod %s status set to decommissioning", podName)
	json := "jsonpath={.items[*].metadata.name}"
	k := kubectl.Get("pod").
		WithLabel("cassandra.datastax.com/node-state=Decommissioning").
		FormatOutput(json)
	ns.WaitForOutputAndLog(step, k, podName, 120)

	step = "check the remaining pods haven't been decommissioned yet"
	json = "jsonpath={.items[*].metadata.name}"
	k = kubectl.Get("pod").
		WithLabel("cassandra.datastax.com/node-state=Started").
		FormatOutput(json)
	ns.WaitForOutputAndLog(step, k, strings.Join(expectedRemainingPods, " "), 10)

	step = fmt.Sprintf("check that pod %s got terminated", podName)
	json = "jsonpath={.items}"
	k = kubectl.Get("pod").
		WithFlag("field-selector", fmt.Sprintf("metadata.name=%s", podName)).
		FormatOutput(json)
	ns.WaitForOutputAndLog(step, k, "[]", 360)
}
