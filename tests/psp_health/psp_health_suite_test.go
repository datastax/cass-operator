// Copyright DataStax, Inc.
// Please see the included license file for details.

package psp_health

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
	"gopkg.in/yaml.v2"
)

var (
	testName         = "PSP Health"
	namespace        = "test-psp-health"
	dcName           = "dc2"
	dcYaml           = "../testdata/default-single-rack-2-node-dc.yaml"
	dcResource       = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel          = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns               = ginkgo_util.NewWrapper(testName, namespace)
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

func getPspInstanceHealth() (map[interface{}]interface{}, error) {
	path := "jsonpath={.data.health}"
	k := kubectl.Get("ConfigMap", "health-check-0").FormatOutput(path)
	configText := ns.OutputPanic(k)

	config := map[interface{}]interface{}{}
	err := yaml.Unmarshal([]byte(configText), config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func getPath(obj interface{}, path... interface{}) interface{} {
	if len(path) == 0 {
		return obj
	}

	m, ok := obj.(map[interface{}]interface{})
	if ok {
		return getPath(m[path[0].(string)], path[1:]...)
	}

	l, ok := obj.([]interface{})
	if ok {
		return getPath(l[path[0].(int)], path[1:]...)
	}
	
	return nil
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator syncs PSP health status information", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstallWithPSPEnabled("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 1 rack/2 node"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			step = "check health status config map exists"
			path := "jsonpath={.items.*.metadata.name}"
			k = kubectl.Get("ConfigMap").
				WithLabel("vmware.vsphere.health=health").
				FormatOutput(path)
			ns.WaitForOutputAndLog(step, k, "health-check-0", 60)

			step = "check health catalog config map exists"
			path = "jsonpath={.items.*.metadata.name}"
			k = kubectl.Get("ConfigMap").
				WithLabel("vmware.vsphere.health=catalog").
				FormatOutput(path)
			ns.WaitForOutputAndLog(step, k, "health-catalog-0", 60)

			config, err := getPspInstanceHealth()
			Expect(err).ToNot(HaveOccurred())

			Expect(
				getPath(config, "status", "instancehealth", 0, "instance"),
				).To(Equal(dcName), "Expected instance name to be %s", dcName)

			Expect(
				getPath(config, "status", "instancehealth", 0, "namespace"),
				).To(Equal(namespace), "Expected instance namespace to be %s", namespace)

			Expect(
				getPath(config, "status", "instancehealth", 0, "health"),
				).To(Equal("green"), "Expected instance health value to be green")
		})
	})
})
