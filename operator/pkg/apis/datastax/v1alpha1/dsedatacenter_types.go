package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultRepository = "datastax/dse-server"
	// TODO discuss this before release to a non-CaaS customer
	defaultVersion = "6.7.3"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DseDatacenterSpec defines the desired state of DseDatacenter
// +k8s:openapi-gen=true
type DseDatacenterSpec struct {
	// The number of the DSE server nodes
	Size int32 `json:"size"`
	// DSE Version
	Version string `json:"version"`
	// Repository to grab the DSE image from
	Repository string `json:"repository,omitempty"`
	// Annotations
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels
	Labels map[string]string `json:"labels,omitempty"`
	// Definition file config
	Config string `json:"config,omitempty"`
	// Resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Racks is an exported field, BUT it is highly recommended that GetRacks() be used for accessing in order to handle
	// the case where no rack is defined
	Racks []DseRack `json:"racks,omitempty"`
	// StorageClaim
	StorageClaim []DseStorageClaim `json:"storageclaim,omitempty"`
}

func (s *DseDatacenterSpec) GetRacks() []DseRack {
	if len(s.Racks) >= 1 {
		return s.Racks
	}

	return []DseRack{{
		Name: "default",
	}}
}

type DseRack struct {
	// The rack name
	Name string `json:"name"`
	// Annotations
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels
	Labels map[string]string `json:"labels,omitempty"`
}

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
func (in *DseDatacenter) GetSeedList() []string {
	var seeds []string
	nodeServicePattern := "%s-%s-stateful-set-%d.%s-service.%s.svc.cluster.local" // e.g. "example-dsedatacenter-default-stateful-set-0.example-dsedatacenter-service.default.svc.cluster.local"

	if in.Spec.Size == 0 {
		return []string{}
	}

	for _, dseRack := range in.Spec.GetRacks() {
		seeds = append(seeds, fmt.Sprintf(nodeServicePattern, in.Name, dseRack.Name, 0, in.Name, in.Namespace))
	}

	// ensure that each Datacenter has at least 2 seeds
	if len(in.Spec.GetRacks()) == 1 && in.Spec.Size > 1 {
		seeds = append(seeds, fmt.Sprintf(nodeServicePattern, in.Name, in.Spec.GetRacks()[0].Name, 1, in.Name, in.Namespace))
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
