// Copyright DataStax, Inc.
// Please see the included license file for details.

package kind

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
	operatorImage              = "datastax/cass-operator:latest"
	operatorInitContainerImage = "datastax/cass-operator-init:latest"
	envLoadDevImages           = "M_LOAD_DEV_IMAGES"
)

func deleteCluster() error {
	return shutil.RunV("kind", "delete", "cluster")
}

func clusterExists() bool {
	out := shutil.OutputPanic("kind", "get", "clusters")
	return strings.TrimSpace(out) == "kind"
}

func createCluster() {
	// Kind can be flaky when starting up a new cluster
	// so let's give it a few chances to redeem itself
	// after failing
	retries := 5
	var err error
	for retries > 0 {
		// We explicitly request a kubernetes v1.15 cluster with --image
		err = shutil.RunV(
			"kind",
			"create",
			"cluster",
			"--config",
			"tests/testdata/kind/kind_config_6_workers.yaml",
			"--image",
			"kindest/node:v1.15.7@sha256:e2df133f80ef633c53c0200114fce2ed5e1f6947477dbc83261a6a921169488d")
		if err != nil {
			fmt.Printf("KIND failed to create the cluster. %v retries left.\n", retries)
			retries--
		} else {
			return
		}
	}
	mageutil.PanicOnError(err)
}

func loadImage(image string) {
	fmt.Printf("Loading image in kind: %s", image)
	shutil.RunVPanic("kind", "load", "docker-image", image)
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

// Currently there is no concept of "global tool install"
// with the go cli. With the new module system, your project's
// go.mod and go.sum files will be updated with new dependencies
// even though we only care about getting a tool installed.
//
// To get around this, we run the go cli from the /tmp directory
// and our project will remain untouched.
func Install() {
	cwd, err := os.Getwd()
	mageutil.PanicOnError(err)
	os.Chdir("/tmp")
	os.Setenv("GO111MODULE", "on")
	shutil.RunVPanic("go", "get", "sigs.k8s.io/kind@v0.7.0")
	os.Chdir(cwd)
}

// Load the latest copy of a local image into kind
func reloadLocalImage(image string) {
	fullImage := fmt.Sprintf("docker.io/%s", image)
	containers := dockerutil.GetAllContainersPanic()
	for _, c := range containers {
		if strings.HasPrefix(c.Image, "kindest") {
			fmt.Printf("Deleting old image from Docker container: %s\n", c.Id)
			execArgs := []string{"crictl", "rmi", fullImage}
			//TODO properly check for existing image before deleting..
			_ = dockerutil.Exec(c.Id, nil, false, "", "", execArgs).ExecV()
		}
	}
	fmt.Println("Loading new operator Docker image into KIND cluster")
	shutil.RunVPanic("kind", "load", "docker-image", image)
	fmt.Println("Finished loading new operator image into Kind.")
}

// Stand up an empty Kind cluster.
//
// This will also configure kubectl to point
// at the new cluster.
//
// Set M_LOAD_DEV_IMAGES to "true" to pull and
// load the dev images listed in buildsettings.yaml
// into the kind cluster.
func SetupEmptyCluster() {
	deleteCluster()
	createCluster()
	kubectl.ClusterInfoForContext("kind-kind").ExecVPanic()
	loadSettings()
	kubectl.ApplyFiles("operator/k8s-flavors/kind/rancher-local-path-storage.yaml").
		ExecVPanic()
	//TODO make this part optional
	operator.BuildDocker()
	loadImage(operatorInitContainerImage)
	loadImage(operatorImage)
}

// Bootstrap a KIND cluster, then run Ginkgo integration tests.
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

// Perform all the steps to stand up an example Kind cluster,
// except for applying the final cassandra yaml specification.
// This must either be applied manually or by calling SetupCassandraCluster
// or SetupDCECluster.
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

// Stand up an example kind cluster running Apache Cassandra.
// Loads all necessary resources to get a running Apache Cassandra data center and operator
// This target assumes that helm is installed and available on path.
func SetupCassandraCluster() {
	mg.Deps(SetupExampleCluster)
	kubectl.ApplyFiles(
		"operator/example-cassdc-yaml/cassandra-3.11.6/example-cassdc-minimal.yaml",
	).ExecVPanic()
	kubectl.WatchPods()
}

// Stand up an example kind cluster running DSE 6.8.
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
		reloadLocalImage(operatorInitContainerImage)
		reloadLocalImage(operatorImage)
	}

	//Find any lingering test namespaces and delete them
	output := kubectl.Get("namespaces").OutputPanic()
	rows := strings.Split(output, "\n")
	for _, row := range rows {
		name := strings.Fields(row)[0]
		if strings.HasPrefix(name, "test-") {
			fmt.Printf("Cleaning up namespace: %s\n", name)
			_ = kubectl.Delete("cassandradatacenter", "--all").ExecV()
			kubectl.DeleteByTypeAndName("namespace", name).ExecVPanic()
		}
	}
}

func ReloadOperator() {
	operator.BuildDocker()
	reloadLocalImage(operatorInitContainerImage)
	reloadLocalImage(operatorImage)
}
