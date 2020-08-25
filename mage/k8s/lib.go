// Copyright DataStax, Inc.
// Please see the included license file for details.

package k8sutil

import (
	"fmt"
	"os"
	"strings"
	"time"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	gcp "github.com/datastax/cass-operator/mage/gcloud"
	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	helm_util "github.com/datastax/cass-operator/mage/helm"
	integutil "github.com/datastax/cass-operator/mage/integ-tests"
	k3d "github.com/datastax/cass-operator/mage/k3d"
	kind "github.com/datastax/cass-operator/mage/kind"
	"github.com/datastax/cass-operator/mage/kubectl"
	"github.com/datastax/cass-operator/mage/operator"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
	"github.com/magefile/mage/mg"
)

const (
	OperatorImage    = "datastax/cass-operator:latest"
	OperatorImageUBI = "datastax/cass-operator:latest-ubi"
	envLoadDevImages = "M_LOAD_DEV_IMAGES"
	envK8sFlavor     = "M_K8S_FLAVOR"
)

var clusterActions *cfgutil.ClusterActions
var clusterType string

var supportedFlavors = map[string]cfgutil.ClusterActions{
	"kind": kind.ClusterActions,
	"k3d":  k3d.ClusterActions,
	"gke":  gcp.ClusterActions,
}

func getOperatorImage() string {
	var img string
	if baseOs := os.Getenv(operator.EnvBaseOs); baseOs != "" {
		img = "datastax/cass-operator:latest-ubi"
	} else {
		img = "datastax/cass-operator:latest"
	}
	return img
}

func loadImagesFromBuildSettings(cfg cfgutil.ClusterActions, settings cfgutil.BuildSettings) {
	for _, image := range settings.Dev.Images {
		// we likely don't always care if we fail to pull
		// because we could be testing local images
		_ = shutil.RunV("docker", "pull", image)
		cfg.LoadImage(image)
	}
}

func loadSettings(cfg cfgutil.ClusterActions) {
	loadDevImages := os.Getenv(envLoadDevImages)
	if strings.ToLower(loadDevImages) == "true" {
		fmt.Println("Pulling and loading images from buildsettings.yaml")
		settings := cfgutil.ReadBuildSettings()
		loadImagesFromBuildSettings(cfg, settings)
	}
}

// There is potential for globally scoped resources to be left
// over from a previous helm install
func cleanupLingeringHelmResources() {
	_ = kubectl.DeleteByTypeAndName("clusterrole", "cass-operator-cluster-role").ExecV()
	_ = kubectl.DeleteByTypeAndName("clusterrolebinding", "cass-operator").ExecV()
	_ = kubectl.DeleteByTypeAndName("validatingwebhookconfiguration", "cassandradatacenter-webhook-registration").ExecV()
	_ = kubectl.DeleteByTypeAndName("crd", "cassandradatacenters.cassandra.datastax.com").ExecV()
}

func loadClusterSettings() {
	if clusterType == "" {
		clusterType = mageutil.EnvOrDefault(envK8sFlavor, "k3d")
	}

	if clusterActions == nil {
		if cfg, ok := supportedFlavors[clusterType]; ok {
			clusterActions = &cfg
		} else {
			panic(fmt.Sprintf("Unsupported %s specified: %s", envK8sFlavor, clusterType))
		}
	}
}

// Stand up an empty cluster.
//
// This will also configure kubectl to point
// at the new cluster.
//
// Set M_LOAD_DEV_IMAGES to "true" to pull and
// load the dev images listed in buildsettings.yaml
// into the cluster.
func SetupEmptyCluster() {
	loadClusterSettings()
	clusterActions.DeleteCluster()
	clusterActions.CreateCluster()
	// some clusters need a few seconds before we can get kubeconfig info
	time.Sleep(10 * time.Second)
	clusterActions.SetupKubeconfig()
	loadSettings(*clusterActions)
	clusterActions.ApplyDefaultStorage()
	//TODO make this part optional
	operator.BuildDocker()
	operatorImg := getOperatorImage()
	clusterActions.LoadImage(operatorImg)
}

// Bootstrap a cluster, then run Ginkgo integration tests.
//
// Default behavior is to discover and run
// all test suites located under the ./tests/ directory.
//
// To run a subset of test suites, specify the name of the suite
// directories in env var M_INTEG_DIR, separated by a comma
//
// Example:
// M_INTEG_DIR=scale_up,stop_resume
//
// This target assumes that helm is installed and available on path.
func RunIntegTests() {
	loadClusterSettings()
	mg.Deps(SetupEmptyCluster)
	integutil.Run()
	noCleanup := os.Getenv(ginkgo_util.EnvNoCleanup)
	if strings.ToLower(noCleanup) != "true" {
		err := clusterActions.DeleteCluster()
		mageutil.PanicOnError(err)
	}
}

// Perform all the steps to stand up an example cluster,
// except for applying the final cassandra yaml specification.
// This must either be applied manually or by calling SetupCassandraCluster
// or SetupDSECluster.
// This target assumes that helm is installed and available on path.
func SetupExampleCluster() {
	mg.Deps(SetupEmptyCluster)
	kubectl.CreateSecretLiteral("cassandra-superuser-secret", "devuser", "devpass").ExecVPanic()

	overrides := map[string]string{"image": getOperatorImage()}
	var namespace = "default"
	err := helm_util.Install("./charts/cass-operator-chart", "cass-operator", namespace, overrides)
	mageutil.PanicOnError(err)

	// Wait for 15 seconds for the operator to come up
	// because the apiserver will call the webhook too soon and fail if we do not wait
	time.Sleep(time.Second * 15)
}

// Stand up an example cluster running Apache Cassandra.
// Loads all necessary resources to get a running Apache Cassandra data center and operator
// This target assumes that helm is installed and available on path.
func SetupCassandraCluster() {
	mg.Deps(SetupExampleCluster)
	kubectl.ApplyFiles(
		"operator/example-cassdc-yaml/cassandra-3.11.x/example-cassdc-minimal.yaml",
	).ExecVPanic()
	kubectl.WatchPods()
}

// Stand up an example cluster running DSE 6.8.
// Loads all necessary resources to get a running DCE data center and operator
// This target assumes that helm is installed and available on path.
func SetupDSECluster() {
	mg.Deps(SetupExampleCluster)
	kubectl.ApplyFiles(
		"operator/example-cassdc-yaml/dse-6.8.x/example-cassdc-minimal.yaml",
	).ExecVPanic()
	kubectl.WatchPods()
}

// Delete a running cluster.
func DeleteCluster() {
	loadClusterSettings()
	err := clusterActions.DeleteCluster()
	mageutil.PanicOnError(err)
}

// Cleanup potential previous integ test resources.
func EnsureEmptyCluster() {
	loadClusterSettings()
	if !clusterActions.ClusterExists() {
		SetupEmptyCluster()
	} else {
		// always load settings in case we have new images
		// that an existing cluster is missing
		loadSettings(*clusterActions)
		// make sure kubectl is pointing to our cluster
		clusterActions.SetupKubeconfig()
		// we should still ensure that the storage is set up
		// correctly every time
		clusterActions.ApplyDefaultStorage()
		// we still need to build and load an updated
		// set of our local operator images
		operator.BuildDocker()
		clusterActions.ReloadLocalImage(OperatorImage)
	}

	//Find any lingering test namespaces and delete them
	output := kubectl.Get("namespaces").OutputPanic()
	rows := strings.Split(output, "\n")
	for _, row := range rows {
		name := strings.Fields(row)[0]
		if strings.HasPrefix(name, "test-") {
			fmt.Printf("Cleaning up namespace: %s\n", name)
			// check if any cassdcs exist in the namespace.
			// kubectl will return an error if the crd has not been
			// applied first
			err := kubectl.Get("cassdc").InNamespace(name).ExecV()
			if err == nil {
				// safe to perform a delete cassdcs --all at this point
				err := kubectl.Delete("cassdcs", "--all").
					InNamespace(name).
					ExecV()

				// if we fail to delete a dc, then we cannot delete the
				// namespace because k8s will get stuck, so we need
				// to stop execution here
				mageutil.PanicOnError(err)
			}
			kubectl.DeleteByTypeAndName("namespace", name).ExecVPanic()
		}
	}
	cleanupLingeringHelmResources()
}

// Rebuild and load local operator image
// into cluster.
func ReloadOperator() {
	loadClusterSettings()
	operator.BuildDocker()
	clusterActions.ReloadLocalImage(OperatorImage)
}

// List k8s flavors that we support
// automating development workflows for
func ListSupportedFlavors() {
	fmt.Println("--------------------------------------------------------------")
	fmt.Printf("%s can be set to one of the following k8s flavors\n", envK8sFlavor)
	fmt.Println("--------------------------------------------------------------")
	for key := range supportedFlavors {
		fmt.Println(key)
	}
	fmt.Println("--------------------------------------------------------------")
}

// List environment variables that
// can be set for the specified cluster.
func Env() {
	loadClusterSettings()

	// Env available for all cluster actions
	fmt.Println("--------------------------------------------------------------")
	fmt.Println(" Environment variables to be used in any cluster")
	fmt.Println("--------------------------------------------------------------")
	fmt.Printf("%s - If set to true, will load local dev images into cluster\n", envLoadDevImages)
	fmt.Printf("%s - The type of k8s cluster to use. Run the listSupportedFlavors mage target for more info.\n", envK8sFlavor)

	fmt.Println("\n--------------------------------------------------------------")
	fmt.Printf(" Environment variables specific to the specific cluster: %s\n", clusterType)
	fmt.Println("--------------------------------------------------------------")
	clusterSpecific := clusterActions.DescribeEnv()

	if len(clusterSpecific) == 0 {
		fmt.Println("<None>")
	} else {
		for key := range clusterSpecific {
			fmt.Printf("%s - %s\n", key, clusterSpecific[key])
		}
	}
}

// If available, install the cli
// tool associated with the chosen k8s flavor
func InstallTool() {
	loadClusterSettings()
	clusterActions.InstallTool()
}

// Configure kubectl to point to the specified cluster
func Kubeconfig() {
	loadClusterSettings()
	clusterActions.SetupKubeconfig()
}
