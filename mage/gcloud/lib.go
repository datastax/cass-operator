// Copyright DataStax, Inc.
// Please see the included license file for details.

package gcloud

import (
	"fmt"
	"strconv"

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

func createCluster(name string, cfg ClusterConfig) {
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

func setupKubeConfig(name string, cfg ClusterConfig) {
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

// Creates an empty k8s cluster in GKE.
//
// Requires the following env vars:
// M_GC_PROJECT
// M_GC_CLUSTER_NAME
//
// Optional env vars:
// M_GC_NODES
// M_GC_MACHINE
// M_GC_REGION
// M_GC_VERSION
// M_GC_BOOT_DISK
// M_GC_BOOT_SIZE
// M_GC_IMAGE_TYPE
func SetupEmptyCluster() {
	proj := mageutil.RequireEnv(envProject)
	name := mageutil.RequireEnv(envClusterName)
	cfg := buildClusterConfig(proj)
	createCluster(name, cfg)
	setupKubeConfig(name, cfg)
}

// Configures kubectl to point to a GKE cluster.
//
// Requires the following env vars:
// M_GC_PROJECT
// M_GC_CLUSTER_NAME
//
// Optional env vars:
// M_GC_REGION
//
// Region will default to us-central1
func KubeConfig() {
	proj := mageutil.RequireEnv(envProject)
	name := mageutil.RequireEnv(envClusterName)
	cfg := buildClusterConfig(proj)
	setupKubeConfig(name, cfg)
}
