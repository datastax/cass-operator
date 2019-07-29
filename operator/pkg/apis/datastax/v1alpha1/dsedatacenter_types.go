package v1alpha1

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/Jeffail/gabs"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/riptano/dse-operator/operator/pkg/dseconfig"
	"github.com/riptano/dse-operator/operator/pkg/utils"
)

const (
	defaultRepository = ""
	defaultVersion    = ""

	defaultConfigBuilderImage = "datastax-docker.jfrog.io/datastax/dse-server-config-builder:7.0.0-3e8847c"

	CLUSTER_LABEL    = "com.datastax.dse.cluster"
	DATACENTER_LABEL = "com.datastax.dse.datacenter"
	SEED_NODE_LABEL  = "com.datastax.dse.seednode"
	RACK_LABEL       = "com.datastax.dse.rack"

	// FIXME switch over to using these constants below, then get rid of the above ones
	// golint says ALL_CAPS is a no-no

	// ClusterLabel is the DSE operator's label for the DSE cluster name
	ClusterLabel = CLUSTER_LABEL

	// DatacenterLabel is the DSE operator's label for the DSE datacenter name
	DatacenterLabel = DATACENTER_LABEL

	// SeedNodeLabel is the DSE operator's label for the DSE seed node state
	SeedNodeLabel = SEED_NODE_LABEL

	// RackLabel is the DSE operator's label for the DSE rack name
	RackLabel = RACK_LABEL

	// RackLabel is the DSE operator's label for the DSE rack name
	DseOperatorProgressLabel = "com.datastax.dse.operator.progress"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DseDatacenterSpec defines the desired state of DseDatacenter
// +k8s:openapi-gen=true
type DseDatacenterSpec struct {
	// The desired number of DSE server pods
	Size int32 `json:"size"`
	// DSE Version
	Version string `json:"version"`
	// Repository to grab the DSE image from
	Repository string `json:"repository"`
	// Definition file config
	// Note that k8s will populate Spec.Config with a json version of the contents
	// of this field.  Somehow k8s converts the yaml fragment to json, which is bizarre
	// but useful for us.  We can use []byte(dseDatacenter.Spec.Config) to make
	// the data accessible for parsing.
	Config json.RawMessage `json:"config,omitempty"`
	// Resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Racks is an exported field, BUT it is highly recommended that GetRacks() be used for accessing in order to handle
	// the case where no rack is defined
	Racks []DseRack `json:"racks,omitempty"`
	// StorageClaim
	StorageClaim *DseStorageClaim `json:"storageClaim,omitempty"`
	// DSE ClusterName
	ClusterName string `json:"clusterName"`
	// Parked state means we do not want any DSE processes running
	Parked bool `json:"parked,omitempty"`
	// ConfigBuilderImage
	ConfigBuilderImage string `json:"configBuilderImage,omitempty"`
}

// GetRacks is a getter for the DseRack slice in the spec
// It ensures there is always at least one rack
func (s *DseDatacenterSpec) GetRacks() []DseRack {
	if len(s.Racks) >= 1 {
		return s.Racks
	}

	return []DseRack{{
		Name: "default",
	}}
}

// DseRack ...
type DseRack struct {
	// The rack name
	Name string `json:"name"`
}

// DseStorageClaim ...
type DseStorageClaim struct {
	StorageClassName string `json:"storageclassname"`
	// Resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// DseDatacenterStatus defines the observed state of DseDatacenter
// +k8s:openapi-gen=true
type DseDatacenterStatus struct {
	// The number of the DSE server nodes
	Nodes int32 `json:"nodes"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DseDatacenter is the Schema for the dsedatacenters API
// +k8s:openapi-gen=true
type DseDatacenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DseDatacenterSpec   `json:"spec,omitempty"`
	Status DseDatacenterStatus `json:"status,omitempty"`
}

// GetSeedList will create a list of seed nodes to satisfy the conditions of
// 1. Assign one seed for each datacenter / rack combination
// 2. Then ensure that each Datacenter has at least 2 seeds
//
// In the event that no seeds are found, an empty list will be returned.
func (dc *DseDatacenter) GetSeedList() []string {
	var seeds []string
	nodeServicePattern := "%s-%s-%s-sts-%d.%s-%s-service.%s.svc.cluster.local" // e.g. "example-cluster-example-dsedatacenter-default-sts-0.example-cluster-example-dsedatacenter-service.default.svc.cluster.local"

	if dc.Spec.Size == 0 {
		return []string{}
	}

	for _, dseRack := range dc.Spec.GetRacks() {
		seeds = append(seeds, fmt.Sprintf(nodeServicePattern, dc.Spec.ClusterName, dc.Name, dseRack.Name, 0, dc.Spec.ClusterName, dc.Name, dc.Namespace))
	}

	// ensure that each Datacenter has at least 2 seeds
	if len(dc.Spec.GetRacks()) == 1 && dc.Spec.Size > 1 {
		seeds = append(seeds, fmt.Sprintf(nodeServicePattern, dc.Spec.ClusterName, dc.Name, dc.Spec.GetRacks()[0].Name, 1, dc.Spec.ClusterName, dc.Name, dc.Namespace))
	}

	if seeds == nil {
		return []string{}
	}

	return seeds
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DseDatacenterList contains a list of DseDatacenter
type DseDatacenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DseDatacenter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DseDatacenter{}, &DseDatacenterList{})
}

// GetServerImage combines the Repository and Version into a fully qualified image to pull
// If they aren't specified the default is datastax/dse-server:6.7.3 from docker hub
func (dc *DseDatacenter) GetServerImage() string {
	return makeImage(dc.Spec.Repository, dc.Spec.Version)
}

func (dc *DseDatacenter) GetConfigBuilderImage() string {
	if dc.Spec.ConfigBuilderImage == "" {
		return defaultConfigBuilderImage
	}
	return dc.Spec.ConfigBuilderImage
}

// GetDseVersion returns a simple string version of the DSE version.
// Example:
//   If the Spec.Version is:    6.8.0-DSP-18785-management-api-20190624102615-180cc39
//   GetDseVersion will return: 6.8.0
func (dc *DseDatacenter) GetDseVersion() string {
	// Match from the start of the string until the first dash
	re := regexp.MustCompile(`^([^-]+)`)

	version := dc.Spec.Version
	if version == "" {
		version = defaultVersion
	}
	return re.FindString(version)
}

// makeImage takes the repository and version information from the spec, and returns DSE docker image
// repo should be an empty string, or [hostname[:port]/][path/with/repo]
// if repo is empty we use "datastax/dse-server" as a default
// version should be a tag on the image path pointed to by the repo
// if version is empty we use "6.7.3" as a default
func makeImage(repo, version string) string {
	if repo == "" {
		repo = defaultRepository
	}
	if version == "" {
		version = defaultVersion
	}
	return repo + ":" + version
}

// GetRackLabels ...
func (dc *DseDatacenter) GetRackLabels(rackName string) map[string]string {
	labels := map[string]string{
		RACK_LABEL: rackName,
	}

	utils.MergeMap(&labels, dc.GetDatacenterLabels())

	return labels
}

// GetDatacenterLabels ...
func (dc *DseDatacenter) GetDatacenterLabels() map[string]string {
	labels := map[string]string{
		DATACENTER_LABEL: dc.Name,
	}

	utils.MergeMap(&labels, dc.GetClusterLabels())

	return labels
}

// GetClusterLabels ...
func (dc *DseDatacenter) GetClusterLabels() map[string]string {
	return map[string]string{
		CLUSTER_LABEL: dc.Spec.ClusterName,
	}
}

// GetConfigAsJSON gets a JSON-encoded string suitable for passing to configBuilder
func (dc *DseDatacenter) GetConfigAsJSON() (string, error) {

	modelValues := dseconfig.GetModelValues(dc.GetSeedList(), dc.Spec.ClusterName, dc.Name)

	var modelBytes []byte

	modelBytes, err := json.Marshal(modelValues)
	if err != nil {
		return "", err
	}

	// Combine the model values with the user-specified values

	modelParsed, err := gabs.ParseJSON([]byte(modelBytes))
	if err != nil {
		return "", errors.Wrap(err, "Model information for DseDatacenter resource was not properly configured")
	}

	if dc.Spec.Config != nil {
		configParsed, err := gabs.ParseJSON([]byte(dc.Spec.Config))
		if err != nil {
			return "", errors.Wrap(err, "Error parsing Spec.Config for DseDatacenter resource")
		}

		if err := modelParsed.Merge(configParsed); err != nil {
			return "", errors.Wrap(err, "Error merging Spec.Config for DseDatacenter resource")
		}
	}

	return modelParsed.String(), nil
}

// GetContainerPorts will return the container ports for the pods in a statefulset based on the provided config
func (dc *DseDatacenter) GetContainerPorts() ([]corev1.ContainerPort, error) {
	ports := []corev1.ContainerPort{
		{
			// Note: Port Names cannot be more than 15 characters
			Name:          "native",
			ContainerPort: 9042,
		},
		{
			Name:          "inter-node-msg",
			ContainerPort: 8609,
		},
		{
			Name:          "intra-node",
			ContainerPort: 7000,
		},
		{
			Name:          "tls-intra-node",
			ContainerPort: 7001,
		},
		// jmx-port 7199 was here, seems like we no longer need to expose it
		{
			Name:          "mgmt-api-http",
			ContainerPort: 8080,
		},
	}

	config, err := dc.GetConfigAsJSON()
	if err != nil {
		return nil, err
	}

	var f interface{}
	err = json.Unmarshal([]byte(config), &f)
	if err != nil {
		return nil, err
	}

	m := f.(map[string]interface{})
	promConf := utils.SearchMap(m, "10-write-prom-conf")
	if _, ok := promConf["enabled"]; ok {
		ports = append(ports, corev1.ContainerPort{
			Name:          "prometheus",
			ContainerPort: 9103,
		})
	}

	return ports, nil
}