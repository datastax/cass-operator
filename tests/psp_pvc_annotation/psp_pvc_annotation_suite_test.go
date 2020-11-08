// Copyright DataStax, Inc.
// Please see the included license file for details.

package psp_pvc_annotation

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName    = "Test PSP PVC Annotations"
	opNamespace = "test-psp-pvc-annotation"
	dc1Name     = "dc1"
	// This scenario requires RF greater than 2
	dc1Yaml      = "../testdata/psp-emm-dc.yaml"
	dc1Resource  = fmt.Sprintf("CassandraDatacenter/%s", dc1Name)
	pod1Name     = "cluster1-dc1-r1-sts-0"
	pod1Resource = fmt.Sprintf("pod/%s", pod1Name)
	ns           = ginkgo_util.NewWrapper(testName, opNamespace)
)

func TestLifecycle(t *testing.T) {
	t.Skip("Skip pending KO-536")

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
		Specify("the operator can respond to psp annotations on PVCs", func() {

			By("creating a namespace for the cass-operator")
			err := kubectl.CreateNamespace(opNamespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstallWithPSPEnabled("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating first datacenter resource"
			k := kubectl.ApplyFiles(dc1Yaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dc1Name)

			// Add an annotation to the pvc for the first pod

			json := "jsonpath={.spec.volumes[?(@.name==\"server-data\")].persistentVolumeClaim.claimName}"

			k = kubectl.Get(fmt.Sprintf("pod/%s", pod1Name)).FormatOutput(json)

			pvc1Name, _, err := ns.ExecVCapture(k)
			if err != nil {
				panic(err)
			}

			step = fmt.Sprintf("annotating pvc: %s", pvc1Name)
			k = kubectl.Annotate(
				"persistentvolumeclaim",
				pvc1Name,
				"volumehealth.storage.kubernetes.io/health",
				"inaccessible")
			ns.ExecAndLog(step, k)

			// Wait for a pod to no longer be ready

			ns.WaitForDatacenterReadyPodCount(dc1Name, 2)

			time.Sleep(1 * time.Minute)

			// Wait for the cluster to heal itself

			ns.WaitForDatacenterReady(dc1Name)

			// Make sure things look right in nodetool
			step = "verify in nodetool that we still have the right number of cassandra nodes"
			By(step)
			podNames := ns.GetDatacenterReadyPodNames(dc1Name)
			for _, podName := range podNames {
				nodeInfos := ns.RetrieveStatusFromNodetool(podName)
				Expect(len(nodeInfos)).To(Equal(len(podNames)), "Expect nodetool to return info on exactly %d nodes", len(podNames))
				for _, nodeInfo := range nodeInfos {
					Expect(nodeInfo.Status).To(Equal("up"), "Expected all nodes to be up, but node %s was down", nodeInfo.HostId)
					Expect(nodeInfo.State).To(Equal("normal"), "Expected all nodes to have a state of normal, but node %s was %s", nodeInfo.HostId, nodeInfo.State)
				}
			}
		})
	})
})
