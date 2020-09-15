// Copyright DataStax, Inc.
// Please see the included license file for details.

package scale_down_not_enough_space

import (
	"fmt"
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
	"github.com/google/uuid"
)

var (
	testName   = "Scale down datacenter but not enough space"
	namespace  = "test-scale-down-not-enough-space"
	dcName     = "dc1"
	dcYaml     = "../testdata/default-three-rack-four-node-limited-storage-dc.yaml"
	dcResource = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel    = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns         = ginkgo_util.NewWrapper(testName, namespace)
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
		Specify("scaling down fails when there is not enough space to absorb data", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 3 racks/4 nodes"
			k := kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			podToDecommission := "cluster1-dc1-r1-sts-1"
			podPvcName := "server-data-cluster1-dc1-r1-sts-1"

			user, pw := ns.RetrieveSuperuserCreds("cluster1")
			ns.CqlExecute(podToDecommission, "create keyspace", "CREATE KEYSPACE IF NOT EXISTS my_key WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1}", user, pw)

			ns.CqlExecute(podToDecommission, "create table", "CREATE TABLE IF NOT EXISTS my_key.my_table (id uuid, data text, PRIMARY KEY(id))", user, pw)

			randStr := genRandString(100000)
			for i := 0; i < 500; i++ {
				uuid := uuid.New()

				cql := fmt.Sprintf("INSERT INTO my_key.my_table (id, data) VALUES (%s, '%s')", uuid, randStr)
				ns.CqlExecute(podToDecommission, "Insert random data", cql, user, pw)
			}

			step = "scale down to 3 nodes"
			json := "{\"spec\": {\"size\": 3}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterConditionWithReason(dcName, "DatacenterConditionValid", string(corev1.ConditionFalse), "notEnoughSpaceToScaleDown")

			step = "check node status is not set to decommissioning"
			json = "jsonpath={.items}"
			k = kubectl.Get("pod").
				WithLabel("cassandra.datastax.com/node-state=Decommissioning").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 30)

			step = "check that the pod did not get terminated"
			json = "jsonpath={.items[*].metadata.name}"
			k = kubectl.Get("pod").
				WithFlag("field-selector", fmt.Sprintf("metadata.name=%s", podToDecommission)).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, podToDecommission, 30)

			step = "check that the pod's PVCs did not get terminated"
			json = "jsonpath={.items[*].metadata.name}"
			k = kubectl.Get("pvc").
				WithFlag("field-selector", fmt.Sprintf("metadata.name=%s", podPvcName)).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, podPvcName, 30)

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

func genRandString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
