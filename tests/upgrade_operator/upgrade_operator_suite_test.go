// Copyright DataStax, Inc.
// Please see the included license file for details.

package upgrade_operator

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	helm_util "github.com/datastax/cass-operator/mage/helm"
	"github.com/datastax/cass-operator/mage/kubectl"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

var (
	testName         = "Upgrade Operator"
	namespace        = "test-upgrade-operator"
	oldOperatorChart = "../testdata/cass-operator-1.1.0-chart"
	dcName           = "dc1"
	dcYaml           = "../testdata/operator-1.1.0-oss-dc.yaml"
	podName          = fmt.Sprintf("cluster1-%s-r1-sts-0", dcName)
	podNameJson      = "jsonpath={.items[*].metadata.name}"
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

func InstallOldOperator() {
	step := "install old Cass Operator v1.1.0"
	By(step)
	err := helm_util.Install(oldOperatorChart, "cass-operator", ns.Namespace, map[string]string{})
	mageutil.PanicOnError(err)
}

func UpgradeOperator() {
	step := "upgrade Cass Operator"
	By(step)
	var overrides = map[string]string{"image": cfgutil.GetOperatorImage()}
	err := helm_util.Upgrade("../../charts/cass-operator-chart", "cass-operator", ns.Namespace, overrides)
	mageutil.PanicOnError(err)
}

var _ = Describe(testName, func() {
	Context("when upgrading the Cass Operator", func() {
		Specify("the managed-by label is set correctly", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			InstallOldOperator()

			ns.WaitForOperatorReady()

			step := "creating a datacenter resource with 1 racks/1 node"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			// sanity check
			step = "sanity check that we have resources with defunct managed-by label value"
			k = kubectl.Get("pods").WithFlag("selector", "app.kubernetes.io/managed-by=cassandra-operator")
			output := ns.OutputAndLog(step, k)
			Expect(output).ToNot(Equal(""), "Expected some resources to have managed-by value of 'cassandra-operator'")

			step = "get name of 1.1.0 operator pod"
			json := "jsonpath={.items[].metadata.name}"
			k = kubectl.Get("pods").WithFlag("selector", "name=cass-operator").FormatOutput(json)
			oldOperatorName := ns.OutputAndLog(step, k)

			UpgradeOperator()

			step = "wait for 1.1.0 operator pod to be removed"
			k = kubectl.Get("pods").WithFlag("field-selector", fmt.Sprintf("metadata.name=%s", oldOperatorName))
			ns.WaitForOutputAndLog(step, k, "", 60)

			ns.WaitForOperatorReady()

			// give the operator a minute to reconcile and update the datacenter
			time.Sleep(1 * time.Minute)

			ns.WaitForDatacenterReady(dcName)

			// check no longer using old managed-by value
			step = "ensure no resources using defunct managed-by value after operator upgrade"
			k = kubectl.Get("all,service").
				WithFlag("selector", "app.kubernetes.io/managed-by=cassandra-operator").
				FormatOutput(podNameJson)
			output = ns.OutputAndLog(step, k)
			Expect(output).To(Equal(""), "Expected no resources to have defunct managed-by value of 'cassandra-operator'")

			// check using new managed-by value
			step = "ensure resources using managed-by value of 'cass-operator'"
			k = kubectl.Get("pods").
				WithFlag("selector", "app.kubernetes.io/managed-by=cass-operator").
				FormatOutput(podNameJson)
			output = ns.OutputAndLog(step, k)
			Expect(output).To(Equal(podName), "Expected resources to have managed-by value of 'cass-operator'")
		})
	})
})
