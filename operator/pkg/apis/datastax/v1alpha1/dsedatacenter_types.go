package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// Racks
	Racks []DseRack `json:"racks,omitempty"`
	// StorageClaim
	StorageClaim []DseStorageClaim `json:"storageclaim,omitempty"`
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
