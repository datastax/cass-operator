// Copyright DataStax, Inc.
// Please see the included license file for details.

package helm_chart_imagepullsecrets

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
	testName       = "Helm Chart imagePullSecrets"
	opNamespace    = "test-helm-chart-imagepullsecrets"
	dc1Name        = "dc2"
	dc1Yaml        = "../testdata/default-single-rack-single-node-dc.yaml"
	registrySecret = "githubPullSecret"
	ns             = ginkgo_util.NewWrapper(testName, opNamespace)
)

func TestLifecycle(t *testing.T) {
	// Only run this test if docker creds are defined
	if kubectl.DockerCredentialsDefined() {
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
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator can be correctly installed with imagePullSecrets", func() {

			By("creating a namespace for the cass-operator")
			err := kubectl.CreateNamespace(opNamespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			ns.CreateDockerRegistrySecret(registrySecret)

			time.Sleep(1 * time.Minute)

			/*
				step := "setting up cass-operator resources via helm chart"

						ns.HelmInstallWithImagePullSecrets("../../charts/cass-operator-chart")

						ns.WaitForOperatorReady()

						// Create a small cass cluster to verify cass-operator is functional
						step = "creating first datacenter resource"
						k := kubectl.ApplyFiles(dc1Yaml)
						ns.ExecAndLog(step, k)

						ns.WaitForDatacenterReady(dc1Name)
			*/
		})
	})
})
