// Copyright DataStax, Inc.
// Please see the included license file for details.

package k3d

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

func install() {
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

func setupKubeconfig() {
	config := shutil.OutputPanic("k3d", "get-kubeconfig")
	os.Setenv("KUBECONFIG", config)
	fmt.Printf("To set up kubectl in your shell to use this cluster, set:\n\tKUBECONFIG=%s\n", config)
}

func applyDefaultStorage() {
	kubectl.ApplyFiles("operator/k8s-flavors/kind/rancher-local-path-storage.yaml").
		ExecVPanic()
}
