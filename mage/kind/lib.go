package kind

import (
	"fmt"
	"os"
	"strings"

	"github.com/magefile/mage/mg"
	cfgutil "github.com/riptano/dse-operator/mage/config"
	dockerutil "github.com/riptano/dse-operator/mage/docker"
	"github.com/riptano/dse-operator/mage/kubectl"
	"github.com/riptano/dse-operator/mage/operator"
	shutil "github.com/riptano/dse-operator/mage/sh"
	mageutil "github.com/riptano/dse-operator/mage/util"
)

const (
	operatorImage = "datastax/dse-operator:latest"
)

func deleteCluster() error {
	return shutil.RunV("kind", "delete", "cluster")
}

func createCluster() {
	// We explicitly request a kubernetes v1.15 cluster with --image
	shutil.RunVPanic(
		"kind",
		"create",
		"cluster",
		"--config",
		"operator/deploy/kind/kind-example-config.yaml",
		"--image",
		"kindest/node:v1.15.7@sha256:e2df133f80ef633c53c0200114fce2ed5e1f6947477dbc83261a6a921169488d")
}

func loadImage(image string) {
	fmt.Printf("Loading image in kind: %s", image)
	shutil.RunVPanic("kind", "load", "docker-image", image)
}

// Install Kind.
//
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

// Load the latest operator docker image into a runing kind cluster.
func ReloadOperator() {
	containers := dockerutil.GetAllContainersPanic()
	for _, c := range containers {
		if strings.HasPrefix(c.Image, "kindest") {
			fmt.Printf("Deleting old operator image from Docker container: %s\n", c.Id)
			execArgs := []string{"crictl", "rmi", "docker.io/datastax/dse-operator:latest"}
			dockerutil.Exec(c.Id, nil, false, "", "", execArgs).ExecVPanic()
		}
	}
	fmt.Println("Loading new operator Docker image into KIND cluster")
	shutil.RunVPanic("kind", "load", "docker-image", "datastax/dse-operator:latest")
	fmt.Println("Finished loading new operator image into Kind.")
}

/// Stand up an empty Kind cluster.
///
/// This will also configure kubectl to point
/// at the new cluster.
func SetupEmptyCluster() {
	deleteCluster()
	createCluster()
	kubectl.ClusterInfoForContext("kind-kind").ExecVPanic()
}

/// Run all Ginkgo integration tests.
func RunIntegTests() {
	mg.Deps(SetupEmptyCluster)
	os.Setenv("CGO_ENABLED", "0")

	args := []string{"test", "-v", "./tests/..."}
	noColor := os.Getenv("GINKGO_NOCOLOR")
	if strings.ToLower(noColor) == "true" {
		args = append(args, "-ginkgo.noColor")
	}

	shutil.RunVPanic("go", args...)

	err := deleteCluster()
	mageutil.PanicOnError(err)
}

// Stand up a example Kind cluster.
//
// Loads all necessary resources to get
// a running DseDatacenter and operator
func SetupExampleCluster() {
	mg.Deps(SetupEmptyCluster)
	settings := cfgutil.ReadBuildSettings()
	loadImage(settings.Dev.DseImage)
	loadImage(settings.Dev.ConfigBuilderImage)
	operator.BuildDocker()
	loadImage(operatorImage)
	kubectl.CreateSecretLiteral("dse-superuser-secret", "devuser", "devpass").ExecVPanic()
	kubectl.ApplyFiles(
		"operator/deploy/kind/rancher-local-path-storage.yaml",
		"operator/deploy/role.yaml",
		"operator/deploy/role_binding.yaml",
		"operator/deploy/service_account.yaml",
		"operator/deploy/crds/datastax.com_dsedatacenters_crd.yaml",
		"operator/deploy/operator.yaml",
		"operator/deploy/kind/dsedatacenter-one-rack-example.yaml",
	).ExecVPanic()
	kubectl.WatchPods()
}

func DeleteCluster() {
	err := deleteCluster()
	mageutil.PanicOnError(err)
}
