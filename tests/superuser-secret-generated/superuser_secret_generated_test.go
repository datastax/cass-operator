// Copyright DataStax, Inc.
// Please see the included license file for details.

package superuser_secret_generated

import (
	"fmt"
	"testing"

	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName          = "Superuser Secret Generated"
	namespace         = "test-superuser-secret-generated"
	defaultSecretName = "cluster2-superuser"
	secretResource    = fmt.Sprintf("secret/%s", defaultSecretName)
	dcName            = "dc2"
	dcYaml            = "../testdata/default-single-rack-2-node-dc.yaml"
	operatorYaml      = "../testdata/operator.yaml"
	dcResource        = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel           = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns                = ginkgo_util.NewWrapper(testName, namespace)
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
	Context("when in a new cluster where superuserSecretName is unspecified", func() {
		Specify("the operator generates an appropriate superuser secret", func() {
			var step string
			var json string
			var k kubectl.KCmd

			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step = "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 1 racks/2 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			// verify the secret was created
			step = "check that the superuser secret was created"
			k = kubectl.Get(secretResource)
			ns.ExecAndLog(step, k)

			// verify the secret credentials actually work because that would be nice
			step = "get superuser username"
			json = "jsonpath={.data.username}"
			k = kubectl.Get(secretResource).FormatOutput(json)
			usernameBase64 := ns.OutputAndLog(step, k)
			Expect(usernameBase64).ToNot(Equal(""), "Expected secret to specify a username")
			usernameDecoded, err := base64.StdEncoding.DecodeString(usernameBase64)
			Expect(err).ToNot(HaveOccurred())

			step = "get superuser password"
			json = "jsonpath={.data.password}"
			k = kubectl.Get(secretResource).FormatOutput(json)
			passwordBase64 := ns.OutputAndLog(step, k)
			Expect(passwordBase64).ToNot(Equal(""), "Expected secret to specify a password")
			passwordDecoded, err := base64.StdEncoding.DecodeString(passwordBase64)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(passwordDecoded) <= 55).To(BeTrue(), "bcrypt requires passwords to be 55 bytes or less in size")

			step = "check superuser credentials work"
			k = kubectl.ExecOnPod(
				"cluster2-dc2-r1-sts-0", "--", "cqlsh",
				"--user", string(usernameDecoded),
				"--password", string(passwordDecoded),
				"-e", "select * from system_schema.keyspaces;").
				WithFlag("container", "cassandra")
			ns.ExecAndLog(step, k)

			step = "check that bad credentials don't work"
			k = kubectl.ExecOnPod(
				"cluster2-dc2-r1-sts-0", "--", "cqlsh",
				"--user", string(usernameDecoded),
				"--password", "notthepassword",
				"-e", "select * from system_schema.keyspaces;").
				WithFlag("container", "cassandra")
			By(step)
			err = ns.ExecV(k)
			Expect(err).To(HaveOccurred())
		})
	})
})
