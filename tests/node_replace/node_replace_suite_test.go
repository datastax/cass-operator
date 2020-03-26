package park_unpark

import (
	"fmt"
	"testing"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/riptano/dse-operator/mage/ginkgo"
	"github.com/riptano/dse-operator/mage/kubectl"
)

var (
	testName         = "Node Replace"
	namespace        = "node-replace"
	dcName           = "dc1"
	podNameToReplace = "cluster1-dc1-r3-sts-0"
	dcYaml           = "../testdata/default-three-rack-three-node-dc.yaml"
	operatorYaml     = "../testdata/operator.yaml"
	dcResource       = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel          = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns               = ginkgo_util.NewWrapper(testName, namespace)
	defaultResources = []string{
		"../../operator/deploy/role.yaml",
		"../../operator/deploy/role_binding.yaml",
		"../../operator/deploy/service_account.yaml",
		"../../operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml",
	}
)

func TestLifecycle(t *testing.T) {
	AfterSuite(func() {
		logPath := fmt.Sprintf("%s/aftersuite", ns.LogDir)
		kubectl.DumpAllLogs(logPath).ExecV()

		fmt.Printf("\n\tPost-run logs dumped at: %s\n\n", logPath)
		_ = ns.Terminate()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, testName)
}

type NodetoolNodeInfo struct {
	Status string
	State string
	Address string
	HostId string
	Rack string
}

func RetrieveStatusFromNodetool(podName string) []NodetoolNodeInfo {
	k := kubectl.KCmd{Command: "exec", Args: []string{podName, "-i", "-c", "cassandra", "--namespace", ns.Namespace, "--", "nodetool", "status"}}
	output, err := k.Output()
	Expect(err).ToNot(HaveOccurred())
	
	getFullName := func(s string) string {
		status, ok := map[string]string{
			"U": "up",
			"D": "down",
			"N": "normal",
			"L": "leaving",
			"J": "joining",
			"M": "moving",
			"S": "stopped",
		}[string(s)]
		
		if !ok {
			status = s
		}
		return status
	}

	nodeTexts := regexp.MustCompile(`(?m)^.*(([0-9a-fA-F]+-){4}([0-9a-fA-F]+)).*$`).FindAllString(output, -1)
	nodeInfo := []NodetoolNodeInfo{}
	for _, nodeText := range nodeTexts {
		comps := regexp.MustCompile(`[[:space:]]+`).Split(nodeText, -1)
		nodeInfo = append(nodeInfo,
			NodetoolNodeInfo{
				Status: getFullName(string(comps[0][0])),
				State: getFullName(string(comps[0][1])),
				Address: comps[1],
				HostId: comps[len(comps)-2],
				Rack: comps[len(comps)-1],
			})
	}
	return nodeInfo
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator can replace a defunct cassandra node on pod start", func() {
			var step string
			var json string
			var k kubectl.KCmd

			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step = "creating default resources"
			k = kubectl.ApplyFiles(defaultResources...)
			ns.ExecAndLog(step, k)

			step = "creating the dse operator resource"
			k = kubectl.ApplyFiles(operatorYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the operator to become ready"
			json = "jsonpath={.items[0].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel("name=dse-operator").
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 120)

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the dse nodes to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true true true", 1200)

			step = "checking the cassandra operator progress status is set to Ready"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResource).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "ensure we actually recorded the host IDs for our cassandra nodes"
			json = "jsonpath={.status.nodeStatuses['cluster1-dc1-r1-sts-0','cluster1-dc1-r2-sts-0','cluster1-dc1-r3-sts-0'].hostID}"
			k = kubectl.Get("cassandradatacenter", dcName).FormatOutput(json)
			ns.WaitForOutputPatternAndLog(step, k, `^[a-zA-Z0-9-]{36}\s+[a-zA-Z0-9-]{36}\s+[a-zA-Z0-9-]{36}$`, 30)

			step = "retrieve the persistent volume claim"
			json = "jsonpath={.spec.volumes[?(.name=='server-data')].persistentVolumeClaim.claimName}"
			k = kubectl.Get("pod", podNameToReplace).FormatOutput(json)
			pvcName := ns.OutputAndLog(step, k)

			step = "disabling gossip on pod to remove readiness"
			execArgs := []string{"-c", "cassandra",
				"--", "bash", "-c",
				"nodetool disablegossip",
			}
			k = kubectl.ExecOnPod(podNameToReplace, execArgs...)
			ns.ExecAndLog(step, k)

			step = "verifying that the pod lost readiness"
			json = "jsonpath={.status.containerStatuses[0].ready}"
			k = kubectl.GetByTypeAndName("pod", podNameToReplace).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "false", 60)

			step = "patch CassandraDatacenter with appropriate replaceNodes setting"
			patch := fmt.Sprintf(`{"spec":{"replaceNodes":["%s"]}}`, podNameToReplace)
			k = kubectl.PatchMerge("cassandradatacenter/"+dcName, patch)
			ns.ExecAndLog(step, k)

			step = "wait for the status to indicate we are replacing pods"
			json = "jsonpath={.status.nodeReplacements[0]}"
			k = kubectl.Get("cassandradatacenter", dcName).FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, podNameToReplace, 10)

			step = "kill the pod and its little persistent volume claim too"
			k = kubectl.Delete("pvc", pvcName).WithFlag("wait", "false")
			ns.ExecAndLog(step, k)
			k = kubectl.Delete("pod", podNameToReplace).WithFlag("wait", "false")
			ns.ExecAndLog(step, k)

			step = "wait for the pod to return to life"
			json = "jsonpath={.status.containerStatuses[?(.name=='cassandra')].ready}"
			k = kubectl.Get("pod", podNameToReplace).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 1200)

			// If we do this wrong and start the node we replaced normally (instead of setting the replace
			// flag), we will end up with an additional node in our cluster. This issue should be caught by
			// checking nodetool.
			step = "verify in nodetool that we still have the right number of cassandra nodes"
			By(step)
			for _, podName := range []string{"cluster1-dc1-r1-sts-0", "cluster1-dc1-r2-sts-0", "cluster1-dc1-r3-sts-0"} {
				nodeInfos := RetrieveStatusFromNodetool(podName)
				Expect(len(nodeInfos)).To(Equal(3), "Expect nodetool to return info on exactly 3 nodes")
				for _, nodeInfo := range nodeInfos {
					Expect(nodeInfo.Status).To(Equal("up"), "Expected all nodes to be up, but node %s was down", nodeInfo.HostId)
					Expect(nodeInfo.State).To(Equal("normal"), "Expected all nodes to have a state of normal, but node %s was %s", nodeInfo.HostId, nodeInfo.State)
				}
			}
		})
	})
})
