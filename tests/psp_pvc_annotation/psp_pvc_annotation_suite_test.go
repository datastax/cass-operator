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
	dc1Yaml      = "../testdata/tolerations-dc.yaml"
	dc1Resource  = fmt.Sprintf("CassandraDatacenter/%s", dc1Name)
	pod1Name     = "cluster1-dc1-r1-sts-0"
	pod1Resource = fmt.Sprintf("pod/%s", pod1Name)
	ns           = ginkgo_util.NewWrapper(testName, opNamespace)
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

			i := 1
			for i < 300 {
				time.Sleep(1 * time.Second)
				i += 1

				names := ns.GetDatacenterReadyPodNames(dc1Name)
				if len(names) < 3 {
					break
				}
			}

			time.Sleep(1 * time.Minute)

			// Wait for the cluster to heal itself

			ns.WaitForDatacenterReady(dc1Name)
		})
	})
})
