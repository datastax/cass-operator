// Copyright DataStax, Inc.
// Please see the included license file for details.

package upgrade_operator

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
	helm_util "github.com/datastax/cass-operator/mage/helm"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

var (
	testName         = "Upgrade Operator"
	namespace        = "test-upgrade-operator"
	oldOperatorChart = "../testdata/cass-operator-1.1.0-chart"
	dcName           = "dc1"
	dcYaml           = "../testdata/operator-1.1.0-oss-dc.yaml"
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
	var overrides = map[string]string{"image": ginkgo_util.OperatorImage}
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

			UpgradeOperator()

			ns.WaitForOperatorReady()

			time.Sleep(1 * time.Minute)

			ns.WaitForDatacenterReady(dcName)
		})
	})
})
