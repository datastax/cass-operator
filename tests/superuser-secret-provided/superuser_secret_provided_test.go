// Copyright DataStax, Inc.
// Please see the included license file for details.

package superuser_secret_provided

import (
	"fmt"
	"testing"
	"strings"
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName            = "Superuser Secret Provided"
	namespace           = "test-superuser-secret-provided"
	superuserSecretName = "my-superuser-secret"
	defaultSecretName   = "cluster2-superuser"
	superuserName       = "bob"
	superuserPass       = "bobber"
	secretResource      = fmt.Sprintf("secret/%s", superuserSecretName)
	dcName              = "dc2"
	dcYaml              = "../testdata/default-single-rack-2-node-dc-with-superuser-secret.yaml"
	operatorYaml        = "../testdata/operator.yaml"
	dcResource          = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel             = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns                  = ginkgo_util.NewWrapper(testName, namespace)
	defaultResources    = []string{
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

func retrievePodNames(ns ginkgo_util.NsWrapper, dcName string) []string {
	json := "jsonpath={.items[*].metadata.name}"
	k := kubectl.Get("pods").
		WithFlag("selector", fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)).
		FormatOutput(json)

	output := ns.OutputPanic(k)
	podNames := strings.Split(output, " ")
	sort.Sort(sort.StringSlice(podNames))

	return podNames
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

			step = "creating default resources"
			k = kubectl.ApplyFiles(defaultResources...)
			ns.ExecAndLog(step, k)

			step = "creating the cass-operator resource"
			k = kubectl.ApplyFiles(operatorYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the operator to become ready"
			json = "jsonpath={.items[0].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel("name=cass-operator").
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 120)

			step = "create superuser secret"
			k = kubectl.CreateSecretLiteral(superuserSecretName, superuserName, superuserPass)
			ns.ExecAndLog(step, k)

			step = "creating a datacenter resource with 1 racks/2 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "checking the cassandra operator progress status is set to Ready"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResource).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 1200)

			podNames := retrievePodNames(ns, dcName)

			step = "check superuser credentials work"
			k = kubectl.ExecOnPod(
				podNames[0], "--", "cqlsh", 
				"--user", superuserName, 
				"--password", superuserPass, 
				"-e", "select * from system_schema.keyspaces;").
				WithFlag("container", "cassandra")
			ns.ExecAndLog(step, k)

			step = "check that bad credentials don't work"
			k = kubectl.ExecOnPod(
				podNames[0], "--", "cqlsh", 
				"--user", superuserName, 
				"--password", "notthepassword", 
				"-e", "select * from system_schema.keyspaces;").
				WithFlag("container", "cassandra")
			By(step)
			err = ns.ExecV(k)
			Expect(err).To(HaveOccurred())

			// It wouldn't be the end of the world if the default secret were
			// unnecessarily created (so long as it isn't used), but it would
			// be confusing.
			step = "check that the default superuser secret was not generated"
			k = kubectl.Get("secret", defaultSecretName)
			By(step)
			err = ns.ExecV(k)
			Expect(err).To(HaveOccurred())
		})
	})
})
