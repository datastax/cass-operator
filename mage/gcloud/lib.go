// Copyright DataStax, Inc.
// Please see the included license file for details.

package gcloud

import (
	"fmt"
	"strconv"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	"github.com/datastax/cass-operator/mage/kubectl"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

const (
	envProject        = "M_GC_PROJECT"
	envNumNodes       = "M_GC_NODES"
	envMachineType    = "M_GC_MACHINE"
	envRegion         = "M_GC_REGION"
	envClusterVersion = "M_GC_VERSION"
	envBootDisk       = "M_GC_BOOT_DISK"
	envBootSize       = "M_GC_BOOT_SIZE"
	envImageType      = "M_GC_IMAGE_TYPE"
	envClusterName    = "M_GC_CLUSTER_NAME"
)

type ClusterConfig struct {
	Project        string
	NumNodes       string
	MachineType    string
	ImageType      string
	Region         string
	ClusterVersion string
	BootDisk       string
	BootSize       string
}

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
	return map[string]string{
		"M_GC_PROJECT":      "GCP project name. No default, required.",
		"M_GC_NODES":        "Number of worker nodes for the cluster. Must be divisible by 3. Defaults to 9",
		"M_GC_MACHINE":      "Machine type for the nodes",
		"M_GC_REGION":       "Region. Defaults to us-central1",
		"M_GC_VERSION":      "k8s version. If not set, stable channel will be used.",
		"M_GC_BOOT_DISK":    "Boot disk type. Defaults to pd-standard",
		"M_GC_BOOT_SIZE":    "Boot disk size. Defaults to 50",
		"M_GC_IMAGE_TYPE":   "Image type. Defaults to COS",
		"M_GC_CLUSTER_NAME": "Cluster name. No default, required.",
	}
}

func deleteCluster() error {
	// TODO Support this
	fmt.Println("A mage target has attempted to delete a running cluster,")
	fmt.Println("but this action is currently not implemented for gcp.")
	return nil
}

func clusterExists() bool {
	// TODO Support this
	fmt.Println("A mage target has attempted to check if a cluster exists or not,")
	fmt.Println("but this action is currently not implemented for gcp.")
	return false
}

func install() {
	// TODO install gcloud here
	panic("install not yet implemented for gcp")
}

func loadImage(image string) {
	fmt.Println("A mage target has attempted to load a docker image into a running cluster,")
	fmt.Println("but this action is not supported for gcp.")
	fmt.Println("Please ensure any images that you need for your cluster are available")
	fmt.Println("in either GCR or a public docker repository.")
}

func reloadLocalImage(image string) {
	fmt.Println("A mage target has attempted to load a docker image into a running cluster,")
	fmt.Println("but this action is not supported for gcp.")
	fmt.Println("Please ensure any images that you need for your cluster are available")
	fmt.Println("in either GCR or a public docker repository.")
}

func applyDefaultStorage() {
	kubectl.ApplyFiles("./operator/k8s-flavors/gke/storage.yaml").
		ExecVPanic()
}

func calculateNumNodes() int {
	numNodes := mageutil.EnvOrDefault(envNumNodes, "9")
	numParsed, err := strconv.Atoi(numNodes)
	// GKE will give you numNodes * 3 total nodes
	// so we should let users specify in terms of units
	// that they expect.
	if err != nil || numParsed%3 != 0 {
		msg := "Must specify an integer that is divisible by 3 in M_GC_NODES."
		msg = fmt.Sprintf("%s\n\tSpecified value is invalid: %s", msg, numNodes)
		panic(msg)
	}
	return numParsed / 3
}

func buildClusterConfig(project string) ClusterConfig {
	// TODO extract out cluster config into a singleton style global variable
	// since this will potentially get called multiple times from a single
	// high level target
	return ClusterConfig{
		ClusterVersion: mageutil.EnvOrDefault(envClusterVersion, ""),
		Project:        project,
		NumNodes:       fmt.Sprintf("%d", calculateNumNodes()),
		MachineType:    mageutil.EnvOrDefault(envMachineType, "n1-standard-2"),
		Region:         mageutil.EnvOrDefault(envRegion, "us-central1"),
		BootDisk:       mageutil.EnvOrDefault(envBootDisk, "pd-standard"),
		BootSize:       mageutil.EnvOrDefault(envBootSize, "50"),
		ImageType:      mageutil.EnvOrDefault(envImageType, "COS"),
	}

}

func (cfg ClusterConfig) ToCliArgs() []string {
	args := []string{
		"--project", cfg.Project,
		"--region", cfg.Region,
		"--machine-type", cfg.MachineType,
		"--image-type", cfg.ImageType,
		"--disk-type", cfg.BootDisk,
		"--disk-size", cfg.BootSize,
		"--num-nodes", cfg.NumNodes,
	}

	// always default to stable channel, but allow the user to specify a cluster
	// version if needed
	if cfg.ClusterVersion != "" {
		args = append(args, "--cluster-version", cfg.ClusterVersion)
	} else {
		args = append(args, "--release-channel", "stable")
	}

	return args
}

func createCluster() {
	proj := mageutil.RequireEnv(envProject)
	name := mageutil.RequireEnv(envClusterName)
	cfg := buildClusterConfig(proj)
	args := []string{
		"beta",
		"container",
		"clusters",
		"create",
		name,
		"--no-enable-basic-auth",
		"--enable-stackdriver-kubernetes",
		"--enable-ip-alias",
		"--network", "projects/gcp-dmc/global/networks/default",
		"--subnetwork", "projects/gcp-dmc/regions/us-central1/subnetworks/default",
		"--default-max-pods-per-node", "110",
		"--no-enable-master-authorized-networks",
		"--addons", "HorizontalPodAutoscaling,HttpLoadBalancing",
		"--enable-autoupgrade",
		"--enable-autorepair",
		"--labels", "cass-operator-testing=true",
		"--node-labels", "cass-operator-testing=true",
		"--metadata", "disable-legacy-endpoints=true,cass-operator-testing=true",
	}

	args = append(args, cfg.ToCliArgs()...)
	shutil.RunVPanic("gcloud", args...)
}

func setupKubeconfig() {
	proj := mageutil.RequireEnv(envProject)
	name := mageutil.RequireEnv(envClusterName)
	cfg := buildClusterConfig(proj)
	args := []string{
		"container",
		"clusters",
		"get-credentials",
		name,
		"--region", cfg.Region,
		"--project", cfg.Project,
	}
	shutil.RunVPanic("gcloud", args...)
}
