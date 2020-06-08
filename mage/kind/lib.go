// Copyright DataStax, Inc.
// Please see the included license file for details.

package kind

import (
	"fmt"
	"os"
	"strings"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	dockerutil "github.com/datastax/cass-operator/mage/docker"
	"github.com/datastax/cass-operator/mage/kubectl"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

var ClusterActions = cfgutil.NewClusterActions(
	deleteCluster,
	clusterExists,
	createCluster,
	loadImage,
	install,
	reloadLocalImage,
	applyDefaultStorage,
	setupKubeconfig,
	describeEnv,
)

func describeEnv() map[string]string {
	return make(map[string]string)
}

func applyDefaultStorage() {
	kubectl.ApplyFiles("operator/k8s-flavors/kind/rancher-local-path-storage.yaml").
		ExecVPanic()
}

func setupKubeconfig() {
	kubectl.ClusterInfoForContext("kind-kind").ExecVPanic()
}

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
			"kindest/node:v1.17.5@sha256:ab3f9e6ec5ad8840eeb1f76c89bb7948c77bbf76bcebe1a8b59790b8ae9a283a")
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

// Currently there is no concept of "global tool install"
// with the go cli. With the new module system, your project's
// go.mod and go.sum files will be updated with new dependencies
// even though we only care about getting a tool installed.
//
// To get around this, we run the go cli from the /tmp directory
// and our project will remain untouched.
func install() {
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
