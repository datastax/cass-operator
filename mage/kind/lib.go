package kind

import (
	"fmt"
	"os"
	"strings"

	"github.com/riptano/dse-operator/mage/config"
	"github.com/riptano/dse-operator/mage/docker"
	"github.com/riptano/dse-operator/mage/operator"
	"github.com/riptano/dse-operator/mage/sh"
)

const (
	operatorImage = "datastax/dse-operator:latest"
)

func deleteCluster() {
	// Will error out if we don't already have a running cluster
	_ = shutil.RunV("kind", "delete", "cluster")
}

func createCluster() {
	shutil.RunVPanic("kind", "create", "cluster", "--config", "operator/deploy/kind/kind-example-config.yaml")
}

func exportKubeConfig() {
	path := shutil.OutputPanic("kind", "get", "kubeconfig-path", "--name=kind")
	os.Setenv("KUBECONFIG", strings.TrimSpace(path))
}

func loadImage(image string) {
	fmt.Printf("Loading image in kind: %s", image)
	shutil.RunVPanic("kind", "load", "docker-image", image)
}

func createSecret(user string, pw string) {
	u := fmt.Sprintf("--from-literal=username=%s", user)
	p := fmt.Sprintf("--from-literal=password=%s", pw)
	shutil.RunVPanic("kubectl", "create", "secret", "generic", "dse-superuser-secret", u, p)
}

func applyFile(path string) {
	shutil.RunVPanic("kubectl", "apply", "-f", path)
}

func watchPods() {
	shutil.RunVPanic("watch", "-n1", "kubectl", "get", "pods")
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
	os.Chdir("/tmp")
	os.Setenv("GO111MODULE", "on")
	shutil.RunVPanic("go", "get", "sigs.k8s.io/kind@v0.5.1")
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

// Stand up a Kind cluster.
//
// Loads all necessary yaml files to get
// a running DseDatacenter and operator
func Setup() {
	settings := cfgutil.ReadBuildSettings()
	deleteCluster()
	createCluster()
	exportKubeConfig()
	loadImage(settings.Dev.DseImage)
	loadImage(settings.Dev.ConfigBuilderImage)
	operator.BuildDocker()
	loadImage(operatorImage)
	createSecret("devuser", "devpass")
	applyFile("operator/deploy/kind/rancher-local-path-storage.yaml")
	applyFile("operator/deploy/role.yaml")
	applyFile("operator/deploy/role_binding.yaml")
	applyFile("operator/deploy/service_account.yaml")
	applyFile("operator/deploy/crds/datastax.com_dsedatacenters_crd.yaml")
	applyFile("operator/deploy/operator.yaml")
	applyFile("operator/deploy/kind/dsedatacenter-one-rack-example.yaml")
	watchPods()
}
