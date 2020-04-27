// Copyright DataStax, Inc.
// Please see the included license file for details.

package delete_node_terminated_container

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName     = "Delete Node where Cassandra container terminated, restarted, and isn't becoming ready"
	namespace    = "test-delete-node-terminated-container"
	dcName       = "dc2"
	dcYaml       = "../testdata/default-single-rack-single-node-dc.yaml"
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
		Specify("the operator can detect a node where Cassandra container terminated, restarted, and is hanging, and delete the pod", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 1 rack/1 node"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the pod to start up"
			json := `jsonpath={.items[0].metadata.labels.cassandra\.datastax\.com/node-state}`
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Starting", 1200)

			// give the cassandra container some time to get created
			time.Sleep(10 * time.Second)

			step = "finding name of the pod"
			json = "jsonpath={.items[0].metadata.name}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				FormatOutput(json)
			podName := ns.OutputAndLog(step, k)

			step = "finding mgmt api PID on the pod"
			execArgs := []string{"-c", "cassandra",
				"--", "bash", "-c",
				"ps aux | grep [m]anagement-api",
			}
			k = kubectl.ExecOnPod(podName, execArgs...)
			ps := ns.OutputAndLog(step, k)
			pid := strings.Fields(ps)[1]

			step = "killing mgmt api process on the pod"
			execArgs = []string{"-c", "cassandra",
				"--", "bash", "-c",
				fmt.Sprintf("kill %s", pid),
			}
			k = kubectl.ExecOnPod(podName, execArgs...)
			ns.ExecAndLog(step, k)

			step = "waiting for the operator to terminate the pod"
			json = "jsonpath={.metadata.deletionTimestamp}"
			k = kubectl.GetByTypeAndName("pod", podName).
				FormatOutput(json)
			ns.WaitForOutputContainsAndLog(step, k, "-", 700)

			step = "waiting for the terminated pod to come back"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 600)

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
