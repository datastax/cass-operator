// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
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
			errString: "CassandraDatacenter attempted to use unsupported DSE version '4.8.0'",
		},
		{
			name: "Cassandra valid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "cassandra",
					ServerVersion: "3.11.6",
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
			errString: "CassandraDatacenter attempted to use unsupported Cassandra version '6.8.0'",
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
				if err.Error() != tt.errString {
					t.Errorf("ValidateSingleDatacenter() err = %v, want %v", err, tt.errString)
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
			errString: "CassandraDatacenter attempted to change ClusterName",
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
			errString: "CassandraDatacenter attempted to change AllowMultipleNodesPerWorker",
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
			errString: "CassandraDatacenter attempted to change SuperuserSecretName",
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
			errString: "CassandraDatacenter attempted to change ServiceAccount",
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
			errString: "CassandraDatacenter attempted to change StorageConfig",
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
			errString: "CassandraDatacenter attempted to remove Rack",
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
			errString: "CassandraDatacenter attempted to change Rack Name from 'rack0' to 'rack0-changed'",
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
			errString: "CassandraDatacenter attempted to change Rack Zone from 'zone2' to 'zone2-changed'",
		},
		{
			name: "Adding a rack is allowed",
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
			errString: "",
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
				if err.Error() != tt.errString {
					t.Errorf("ValidateDatacenterFieldChanges() err = %v, want %v", err, tt.errString)
				}
			}
		})
	}
}
