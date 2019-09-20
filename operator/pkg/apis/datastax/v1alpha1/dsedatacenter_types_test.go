package v1alpha1

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeImage(t *testing.T) {
        type args struct {
                dseImage   string
                dseVersion string
        }
        tests := []struct {
                name string
                args args
                want string
		errString string
        }{
                {
                        name: "test empty image",
                        args: args{
                                dseImage:    "",
                                dseVersion: "6.8.0",
                        },
                        want: "datastaxlabs/dse-k8s-server:6.8.0-20190822",
			errString: "",
                },
                {
                        name: "test private repo server",
                        args: args{
                                dseImage:   "datastax.jfrog.io/secret-debug-image/dse-server:6.8.0-test123",
                                dseVersion: "6.8.0",
                        },
                        want: "datastax.jfrog.io/secret-debug-image/dse-server:6.8.0-test123",
			errString: "",
                },
                {
                        name: "test unknown version",
                        args: args{
                                dseImage:    "",
                                dseVersion: "6.7.0",
                        },
                        want: "",
			errString: "The specified DSE version 6.7.0 does not map to a known container image.",
                },
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        got, err := makeImage(tt.args.dseVersion, tt.args.dseImage)
			if got != tt.want {
                                t.Errorf("makeImage() = %v, want %v", got, tt.want)
			}
			if err == nil {
				if tt.errString != "" {
					t.Errorf("makeImage() err = %v, want %v", err, tt.errString)
				}
			} else {
				if err.Error() != tt.errString {
					t.Errorf("makeImage() err = %v, want %v", err, tt.errString)
				}
			}
                })
        }
}

func TestDseDatacenter_GetServerImage(t *testing.T) {
        type fields struct {
                TypeMeta   metav1.TypeMeta
                ObjectMeta metav1.ObjectMeta
                Spec       DseDatacenterSpec

               Status     DseDatacenterStatus
        }
        tests := []struct {
                name      string
                fields    fields
                want      string
		errString string
        }{
                {
                        name: "explicit DSE image specified",
                        fields: fields{
                                Spec: DseDatacenterSpec{
                                        DseImage:   "jfrog.io:6789/dse-server-team/dse-server:6.8.0-123",
                                        DseVersion: "6.8.0",
                                },
                        },
                        want: "jfrog.io:6789/dse-server-team/dse-server:6.8.0-123",
			errString: "",
                },
                {
                        name: "invalid version specified",
                        fields: fields{
                                Spec: DseDatacenterSpec{
                                        DseImage:   "",
                                        DseVersion: "9000",
                                },
                        },
                        want: "",
			errString: "The specified DSE version 9000 does not map to a known container image.",
                },
        }
        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        dc := &DseDatacenter{
                                TypeMeta:   tt.fields.TypeMeta,
                                ObjectMeta: tt.fields.ObjectMeta,
                                Spec:       tt.fields.Spec,
                                Status:     tt.fields.Status,
                        }
                        got, err := dc.GetServerImage()
			if got != tt.want {
                                t.Errorf("DseDatacenter.GetServerImage() = %v, want %v", got, tt.want)
			}
			if err == nil {
				if tt.errString != "" {
					t.Errorf("DseDatacenter.GetServerImage() err = %v, want %v", err, tt.errString)
				}
			} else {
				if err.Error() != tt.errString {
					t.Errorf("DseDatacenter.GetServerImage() err = %v, want %v", err, tt.errString)
				}
			}


                })
        }
 }

func TestDseDatacenter_GetSeedList(t *testing.T) {
	type fields struct {
		TypeMeta   metav1.TypeMeta
		ObjectMeta metav1.ObjectMeta
		Spec       DseDatacenterSpec
		Status     DseDatacenterStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "1 DC, 1 Rack, 1 Node",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:      "example-dsedatacenter",
					Namespace: "default_ns",
				},
				Spec: DseDatacenterSpec{
					Size: 1,
					Racks: []DseRack{{
						Name: "rack0",
					}},
					DseClusterName: "example-cluster",
				},
			},
			want: []string{"example-cluster-example-dsedatacenter-rack0-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local"},
		}, {
			name: "1 DC, 2 Rack, 2 Node",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:      "example-dsedatacenter",
					Namespace: "default_ns",
				},
				Spec: DseDatacenterSpec{
					Size: 2,
					Racks: []DseRack{{
						Name: "rack0",
					}, {
						Name: "rack1",
					}},
					DseClusterName: "example-cluster",
				},
			},
			want: []string{"example-cluster-example-dsedatacenter-rack0-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local",
				"example-cluster-example-dsedatacenter-rack1-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local"},
		}, {
			name: "1 DC, 1 Rack, 2 Node",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:      "example-dsedatacenter",
					Namespace: "default_ns",
				},
				Spec: DseDatacenterSpec{
					Size: 2,
					Racks: []DseRack{{
						Name: "rack0",
					}},
					DseClusterName: "example-cluster",
				},
			},
			want: []string{"example-cluster-example-dsedatacenter-rack0-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local",
				"example-cluster-example-dsedatacenter-rack0-sts-1.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local"},
		}, {
			name: "1 DC, 3 Rack, 6 Node",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:      "example-dsedatacenter",
					Namespace: "default_ns",
				},
				Spec: DseDatacenterSpec{
					Size: 6,
					Racks: []DseRack{{
						Name: "rack0",
					}, {
						Name: "rack1",
					}, {
						Name: "rack2",
					}},
					DseClusterName: "example-cluster",
				},
			},
			want: []string{"example-cluster-example-dsedatacenter-rack0-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local",
				"example-cluster-example-dsedatacenter-rack1-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local",
				"example-cluster-example-dsedatacenter-rack2-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local"},
		}, {
			name: "1 DC, 0 Rack, 0 Node",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:      "example-dsedatacenter",
					Namespace: "default_ns",
				},
				Spec: DseDatacenterSpec{
					Size:           0,
					Racks:          []DseRack{},
					DseClusterName: "example-cluster",
				},
			},
			want: []string{},
		}, {
			name: "1 DC, 3 Rack, 3 Node",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:      "example-dsedatacenter",
					Namespace: "default_ns",
				},
				Spec: DseDatacenterSpec{
					Size: 3,
					Racks: []DseRack{{
						Name: "rack0",
					}, {
						Name: "rack1",
					}, {
						Name: "rack2",
					}},
					DseClusterName: "example-cluster",
				},
			},
			want: []string{"example-cluster-example-dsedatacenter-rack0-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local",
				"example-cluster-example-dsedatacenter-rack1-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local",
				"example-cluster-example-dsedatacenter-rack2-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local"},
		}, {
			name: "1 DC, 0 Rack, 1 Node",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:      "example-dsedatacenter",
					Namespace: "default_ns",
				},
				Spec: DseDatacenterSpec{
					Size:           1,
					DseClusterName: "example-cluster",
				},
			},
			want: []string{"example-cluster-example-dsedatacenter-default-sts-0.example-cluster-example-dsedatacenter-service.default_ns.svc.cluster.local"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &DseDatacenter{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			if got := in.GetSeedList(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DseDatacenter.GetSeedList() = %v, want %v", got, tt.want)
			}
		})
	}
}


func Test_GenerateBaseConfigString(t *testing.T) {
	tests := []struct {
		name          string
		dseDatacenter *DseDatacenter
		want          string
		errString     string
	}{
		{
			name: "Simple Test",
			dseDatacenter: &DseDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dseDC",
				},
				Spec: DseDatacenterSpec{
					DseClusterName: "dseCluster",
					Config:         []byte("{\"cassandra-yaml\":{\"authenticator\":\"AllowAllAuthenticator\",\"batch_size_fail_threshold_in_kb\":1280}}"),
				},
			},
			want:      `{"cassandra-yaml":{"authenticator":"AllowAllAuthenticator","batch_size_fail_threshold_in_kb":1280},"cluster-info":{"name":"dseCluster","seeds":"dseCluster-seed-service"},"datacenter-info":{"name":"dseDC"}}`,
			errString: "",
		},
		{
			name: "Simple Test for error",
			dseDatacenter: &DseDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dseDC",
				},
				Spec: DseDatacenterSpec{
					DseClusterName: "dseCluster",
					Config:         []byte("\"cassandra-yaml\":{\"authenticator\":\"AllowAllAuthenticator\",\"batch_size_fail_threshold_in_kb\":1280}}"),
				},
			},
			want:      "",
			errString: "Error parsing Spec.Config for DseDatacenter resource: invalid character ':' after top-level value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.dseDatacenter.GetConfigAsJSON()
			if got != tt.want {
				t.Errorf("GenerateBaseConfigString() got = %v, want %v", got, tt.want)
			}
			if err == nil {
				if tt.errString != "" {
					t.Errorf("GenerateBaseConfigString() err = %v, want %v", err, tt.errString)
				}
			} else {
				if err.Error() != tt.errString {
					t.Errorf("GenerateBaseConfigString() err = %v, want %v", err, tt.errString)
				}
			}
		})
	}
}

func TestDseDatacenter_GetContainerPorts(t *testing.T) {
	type fields struct {
		TypeMeta   metav1.TypeMeta
		ObjectMeta metav1.ObjectMeta
		Spec       DseDatacenterSpec
		Status     DseDatacenterStatus
	}
	tests := []struct {
		name    string
		fields  fields
		want    []corev1.ContainerPort
		wantErr bool
	}{
		{
			name: "Happy Path",
			fields: fields{
				Spec: DseDatacenterSpec{
					DseClusterName: "dseCluster",
					Config:         []byte("{\"cassandra-yaml\":{\"authenticator\":\"AllowAllAuthenticator\",\"batch_size_fail_threshold_in_kb\":1280}}"),
				},
			},
			want: []corev1.ContainerPort{
				{
					Name:          "native",
					ContainerPort: 9042,
				}, {
					Name:          "inter-node-msg",
					ContainerPort: 8609,
				}, {
					Name:          "intra-node",
					ContainerPort: 7000,
				}, {
					Name:          "tls-intra-node",
					ContainerPort: 7001,
				}, {
					Name:          "mgmt-api-http",
					ContainerPort: 8080,
				}},
			wantErr: false,
		},
		{
			name: "Expose Prometheus",
			fields: fields{
				Spec: DseDatacenterSpec{
					DseClusterName: "dseCluster",
					Config:         []byte("{\"cassandra-yaml\":{\"10-write-prom-conf\":{\"enabled\":true,\"port\":9103,\"staleness-delta\":300},\"authenticator\":\"AllowAllAuthenticator\",\"batch_size_fail_threshold_in_kb\":1280}}"),
				},
			},
			want: []corev1.ContainerPort{
				{
					Name:          "native",
					ContainerPort: 9042,
				}, {
					Name:          "inter-node-msg",
					ContainerPort: 8609,
				}, {
					Name:          "intra-node",
					ContainerPort: 7000,
				}, {
					Name:          "tls-intra-node",
					ContainerPort: 7001,
				}, {
					Name:          "mgmt-api-http",
					ContainerPort: 8080,
				}, {
					Name:          "prometheus",
					ContainerPort: 9103,
				}},
			wantErr: false,
		},
		{
			name: "Expose Prometheus - No config",
			fields: fields{
				Spec: DseDatacenterSpec{
					DseClusterName: "dseCluster",
				},
			},
			want: []corev1.ContainerPort{
				{
					Name:          "native",
					ContainerPort: 9042,
				}, {
					Name:          "inter-node-msg",
					ContainerPort: 8609,
				}, {
					Name:          "intra-node",
					ContainerPort: 7000,
				}, {
					Name:          "tls-intra-node",
					ContainerPort: 7001,
				}, {
					Name:          "mgmt-api-http",
					ContainerPort: 8080,
				}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := &DseDatacenter{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			got, err := dc.GetContainerPorts()
			if (err != nil) != tt.wantErr {
				t.Errorf("DseDatacenter.GetContainerPorts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DseDatacenter.GetContainerPorts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDseDatacenter_GetSeedServiceName(t *testing.T) {
	dc := &DseDatacenter{
		Spec: DseDatacenterSpec{
			DseClusterName: "bob",
		},
	}
	want := "bob-seed-service"
	got := dc.GetSeedServiceName()

	if want != got {
		t.Errorf("DseDatacenter.GetSeedService() = %v, want %v", got, want)
	}
}
