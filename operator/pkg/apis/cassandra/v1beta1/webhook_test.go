// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ValidateSingleDatacenter(t *testing.T) {
	tests := []struct {
		name      string
		dc        *CassandraDatacenter
		errString string
	}{
		{
			name: "Dse Valid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "dse",
					ServerVersion: "6.8.0",
				},
			},
			errString: "",
		},
		{
			name: "Dse 6.8.3 Valid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "dse",
					ServerVersion: "6.8.3",
				},
			},
			errString: "",
		},
		{
			name: "Dse Invalid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "dse",
					ServerVersion: "4.8.0",
				},
			},
			errString: "use unsupported DSE version '4.8.0'",
		},
		{
			name: "Cassandra valid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "cassandra",
					ServerVersion: "3.11.7",
				},
			},
			errString: "",
		},
		{
			name: "Cassandra valid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "cassandra",
					ServerVersion: "4.0.0",
				},
			},
			errString: "",
		},
		{
			name: "Cassandra Invalid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "cassandra",
					ServerVersion: "6.8.0",
				},
			},
			errString: "use unsupported Cassandra version '6.8.0'",
		},
		{
			name: "Dse Workloads in Cassandra Invalid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "cassandra",
					ServerVersion: "6.8.0",
					DseWorkloads: &DseWorkloads{
						AnalyticsEnabled: true,
					},
				},
			},
			errString: "CassandraDatacenter write rejected, attempted to enable DSE workloads if server type is Cassandra",
		},
		{
			name: "Dse Workloads in Dse valid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "dse",
					ServerVersion: "6.8.3",
					DseWorkloads: &DseWorkloads{
						AnalyticsEnabled: true,
					},
				},
			},
			errString: "",
		},
		{
			name: "Cassandra 3.11 invalid config file dse-yaml",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "cassandra",
					ServerVersion: "3.11.7",
					Config: json.RawMessage(`
					{
						"cassandra-yaml": {},
						"dse-yaml": {
							"key1": "value1"
						}
					}
					`),
				},
			},
			errString: "attempted to define config dse-yaml with cassandra-3.11.7",
		},
		{
			name: "Cassandra 3.11 invalid config file jvm-server-options",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "cassandra",
					ServerVersion: "3.11.7",
					Config: json.RawMessage(`
					{
						"cassandra-yaml": {},
						"jvm-server-options": {
							"key1": "value1"
						}
					}
					`),
				},
			},
			errString: "attempted to define config jvm-server-options with cassandra-3.11.7",
		},
		{
			name: "DSE 6.8 invalid config file jvm-options",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "dse",
					ServerVersion: "6.8.3",
					Config: json.RawMessage(`
					{
						"cassandra-yaml": {},
						"jvm-options": {
							"key1": "value1"
						}
					}
					`),
				},
			},
			errString: "attempted to define config jvm-options with dse-6.8.3",
		},
		{
			name: "Allow multiple nodes per worker requires resource requests",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:                  "dse",
					ServerVersion:               "6.8.3",
					Config:                      json.RawMessage(`{}`),
					AllowMultipleNodesPerWorker: true,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
			errString: "",
		},
		{
			name: "Allow multiple nodes per worker requires resource requests",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:                  "dse",
					ServerVersion:               "6.8.3",
					Config:                      json.RawMessage(`{}`),
					AllowMultipleNodesPerWorker: true,
				},
			},
			errString: "use multiple nodes per worker without cpu and memory requests and limits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSingleDatacenter(*tt.dc)
			if err == nil {
				if tt.errString != "" {
					t.Errorf("ValidateSingleDatacenter() err = %v, want %v", err, tt.errString)
				}
			} else {
				if !strings.HasSuffix(err.Error(), tt.errString) {
					t.Errorf("ValidateSingleDatacenter() err = %v, want suffix %v", err, tt.errString)
				}
			}
		})
	}
}

func Test_ValidateDatacenterFieldChanges(t *testing.T) {
	storageSize := resource.MustParse("1Gi")
	storageName := "server-data"

	tests := []struct {
		name      string
		oldDc     *CassandraDatacenter
		newDc     *CassandraDatacenter
		errString string
	}{
		{
			name: "No significant changes",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ClusterName:                 "oldname",
					AllowMultipleNodesPerWorker: false,
					SuperuserSecretName:         "hush",
					ServiceAccount:              "admin",
					StorageConfig: StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &storageName,
							AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{"storage": storageSize},
							},
						},
					},
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ClusterName:                 "oldname",
					AllowMultipleNodesPerWorker: false,
					SuperuserSecretName:         "hush",
					ServiceAccount:              "admin",
					StorageConfig: StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &storageName,
							AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{"storage": storageSize},
							},
						},
					},
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			errString: "",
		},
		{
			name: "Clustername changed",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ClusterName: "oldname",
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ClusterName: "newname",
				},
			},
			errString: "change clusterName",
		},
		{
			name: "AllowMultipleNodesPerWorker changed",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					AllowMultipleNodesPerWorker: false,
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					AllowMultipleNodesPerWorker: true,
				},
			},
			errString: "change allowMultipleNodesPerWorker",
		},
		{
			name: "SuperuserSecretName changed",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					SuperuserSecretName: "hush",
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					SuperuserSecretName: "newsecret",
				},
			},
			errString: "change superuserSecretName",
		},
		{
			name: "ServiceAccount changed",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServiceAccount: "admin",
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServiceAccount: "newadmin",
				},
			},
			errString: "change serviceAccount",
		},
		{
			name: "StorageConfig changes",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					StorageConfig: StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &storageName,
							AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{"storage": storageSize},
							},
						},
					},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					StorageConfig: StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &storageName,
							AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteMany"},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{"storage": storageSize},
							},
						},
					},
				},
			},
			errString: "change storageConfig",
		},
		{
			name: "Removing a rack",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			errString: "remove rack",
		},
		{
			name: "Scaling down",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}},
					Size: 6,
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}},
					Size: 3,
				},
			},
			errString: "decrease size",
		},
		{
			name: "Changed a rack name",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0-changed",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			errString: "change rack name from 'rack0' to 'rack0-changed'",
		},
		{
			name: "Changed a rack zone",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2-changed",
					}},
				},
			},
			errString: "change rack zone from 'zone2' to 'zone2-changed'",
		},
		{
			name: "Adding a rack is allowed if size increases",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Size: 3,
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Size: 4,
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}, {
						Name: "rack3",
						Zone: "zone2",
					}},
				},
			},
			errString: "",
		},
		{
			name: "Adding a rack is not allowed if size doesn't increase",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}, {
						Name: "rack3",
						Zone: "zone2",
					}},
				},
			},
			errString: "add rack without increasing size",
		},
		{
			name: "Adding a rack is not allowed if size doesn't increase enough to prevent moving nodes from existing racks",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Size: 9,
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Size: 11,
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}},
				},
			},
			errString: "add racks without increasing size enough to prevent existing nodes from moving to new racks to maintain balance.\nNew racks added: 1, size increased by: 2. Expected size increase to be at least 4",
		},
		{
			name: "Adding multiple racks is not allowed if size doesn't increase enough to prevent moving nodes from existing racks",
			oldDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Size: 9,
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}},
				},
			},
			newDc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					Size: 16,
					Racks: []Rack{{
						Name: "rack0",
						Zone: "zone0",
					}, {
						Name: "rack1",
						Zone: "zone1",
					}, {
						Name: "rack2",
						Zone: "zone2",
					}, {
						Name: "rack3",
						Zone: "zone3",
					}},
				},
			},
			errString: "add racks without increasing size enough to prevent existing nodes from moving to new racks to maintain balance.\nNew racks added: 2, size increased by: 7. Expected size increase to be at least 8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDatacenterFieldChanges(*tt.oldDc, *tt.newDc)
			if err == nil {
				if tt.errString != "" {
					t.Errorf("ValidateDatacenterFieldChanges() err = %v, want %v", err, tt.errString)
				}
			} else {
				if !strings.HasSuffix(err.Error(), tt.errString) {
					t.Errorf("ValidateDatacenterFieldChanges() err = %v, want suffix %v", err, tt.errString)
				}
			}
		})
	}
}
