// Copyright DataStax, Inc.
// Please see the included license file for details.

package k3d

import (
	"fmt"
	"os"
	"strings"
	"time"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	dockerutil "github.com/datastax/cass-operator/mage/docker"
	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	helm_util "github.com/datastax/cass-operator/mage/helm"
	integutil "github.com/datastax/cass-operator/mage/integ-tests"
	"github.com/datastax/cass-operator/mage/kubectl"
	"github.com/datastax/cass-operator/mage/operator"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
	"github.com/magefile/mage/mg"
)

const (
	operatorImage    = "datastax/cass-operator:latest"
	envLoadDevImages = "M_LOAD_DEV_IMAGES"
)

func deleteCluster() error {
	return shutil.RunV("k3d", "delete")
}

func clusterExists() bool {
	err := shutil.RunV("k3d", "ls")
	return err == nil
}

func createCluster() {
	// just incase we get some flakiness with docker
	// while creating a cluster, we want to
	// give it a few chances to redeem itself
	// after failing
	retries := 5
	var err error
	for retries > 0 {
		// We explicitly request a kubernetes v1.15 cluster with --image
		err = shutil.RunV(
			"k3d",
			"create",
			"-w", "6",
			"-image",
			"rancher/k3s:v1.16.9-rc1-k3s1",
		)
		if err != nil {
			fmt.Printf("k3d failed to create the cluster. %v retries left.\n", retries)
			retries--
		} else {
			return
		}
	}
	mageutil.PanicOnError(err)
}

func loadImage(image string) {
	fmt.Printf("Loading image in k3d: %s", image)
	shutil.RunVPanic("k3d", "i", image)
}

func loadImagesFromBuildSettings(settings cfgutil.BuildSettings) {
	for _, image := range settings.Dev.Images {
		shutil.RunVPanic("docker", "pull", image)
		loadImage(image)
	}
}

func loadSettings() {
	loadDevImages := os.Getenv(envLoadDevImages)
	if strings.ToLower(loadDevImages) == "true" {
		fmt.Println("Pulling and loading images from buildsettings.yaml")
		settings := cfgutil.ReadBuildSettings()
		loadImagesFromBuildSettings(settings)
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

func Install() {
	cwd, err := os.Getwd()
	mageutil.PanicOnError(err)
	os.Chdir("/tmp")
	shutil.RunVPanic("curl",
		"https://raw.githubusercontent.com/rancher/k3d/master/install.sh",
		"-o", "k3d_install.sh",
	)
	os.Setenv("TAG", "v1.3.4")
	os.Chmod("./k3d_install.sh", 0755)
	shutil.RunVPanic("./k3d_install.sh")
	os.Chdir(cwd)
}

// Load the latest copy of a local image into k3d
func reloadLocalImage(image string) {
	fullImage := fmt.Sprintf("docker.io/%s", image)
	containers := dockerutil.GetAllContainersPanic()
	for _, c := range containers {
		if strings.HasPrefix(c.Image, "k3d-k3s") {
			fmt.Printf("Deleting old image from Docker container: %s\n", c.Id)
			execArgs := []string{"crictl", "rmi", fullImage}
			//TODO properly check for existing image before deleting..
			_ = dockerutil.Exec(c.Id, nil, false, "", "", execArgs).ExecV()
		}
	}
	fmt.Println("Loading new operator Docker image into k3d cluster")
	shutil.RunVPanic("k3d", "i", image)
	fmt.Println("Finished loading new operator image into k3d.")
}

// Stand up an empty k3d cluster.
//
// This will also configure kubectl to point
// at the new cluster.
//
// Set M_LOAD_DEV_IMAGES to "true" to pull and
// load the dev images listed in buildsettings.yaml
// into the k3d cluster.
func SetupEmptyCluster() {
	deleteCluster()
	createCluster()
	time.Sleep(5 * time.Second)
	KubeConfig()
	loadSettings()
	kubectl.ApplyFiles("operator/k8s-flavors/kind/rancher-local-path-storage.yaml").
		ExecVPanic()
	//TODO make this part optional
	operator.BuildDocker()
	loadImage(operatorImage)
}

// Bootstrap a k3d cluster, then run Ginkgo integration tests.
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
	mg.Deps(SetupEmptyCluster)
	integutil.Run()
	noCleanup := os.Getenv(ginkgo_util.EnvNoCleanup)
	if strings.ToLower(noCleanup) != "true" {
		err := deleteCluster()
		mageutil.PanicOnError(err)
	}
}

// Perform all the steps to stand up an example k3d cluster,
// except for applying the final cassandra yaml specification.
// This must either be applied manually or by calling SetupCassandraCluster
// or SetupDSECluster.
// This target assumes that helm is installed and available on path.
func SetupExampleCluster() {
	mg.Deps(SetupEmptyCluster)
	kubectl.CreateSecretLiteral("cassandra-superuser-secret", "devuser", "devpass").ExecVPanic()

	var namespace = "default"
	var overrides = map[string]string{"image": "datastax/cass-operator:latest"}
	err := helm_util.Install("./charts/cass-operator-chart", "cass-operator", namespace, overrides)
	mageutil.PanicOnError(err)

	// Wait for 15 seconds for the operator to come up
	// because the apiserver will call the webhook too soon and fail if we do not wait
	time.Sleep(time.Second * 15)
}

// Stand up an example k3d cluster running Apache Cassandra.
// Loads all necessary resources to get a running Apache Cassandra data center and operator
// This target assumes that helm is installed and available on path.
func SetupCassandraCluster() {
	mg.Deps(SetupExampleCluster)
	kubectl.ApplyFiles(
		"operator/example-cassdc-yaml/cassandra-3.11.6/example-cassdc-minimal.yaml",
	).ExecVPanic()
	kubectl.WatchPods()
}

// Stand up an example k3d cluster running DSE 6.8.
// Loads all necessary resources to get a running DCE data center and operator
// This target assumes that helm is installed and available on path.
func SetupDSECluster() {
	mg.Deps(SetupExampleCluster)
	kubectl.ApplyFiles(
		"operator/example-cassdc-yaml/dse-6.8.0/example-cassdc-minimal.yaml",
	).ExecVPanic()
	kubectl.WatchPods()
}

// Delete a running cluster
func DeleteCluster() {
	err := deleteCluster()
	mageutil.PanicOnError(err)
}

func EnsureEmptyCluster() {
	if !clusterExists() {
		SetupEmptyCluster()
	} else {
		// we should still ensure that the storage is set up
		// correctly every time
		kubectl.ApplyFiles("operator/k8s-flavors/kind/rancher-local-path-storage.yaml").
			ExecVPanic()
		// we still need to build and load an updated
		// set of our local operator images
		operator.BuildDocker()
		reloadLocalImage(operatorImage)
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

func ReloadOperator() {
	operator.BuildDocker()
	reloadLocalImage(operatorImage)
}

func KubeConfig() {
	fmt.Println("BEFORE CONFIG")
	config := shutil.OutputPanic("k3d", "get-kubeconfig")
	fmt.Println("BEFORE ENV")
	os.Setenv("KUBECONFIG", config)
}
