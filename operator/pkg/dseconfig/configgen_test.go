package dseconfig

import (
	"reflect"
	"testing"
)

func TestGetModelValues(t *testing.T) {
	type args struct {
		seeds       []string
		clusterName string
		dcName      string
	}
	tests := []struct {
		name string
		args args
		want NodeConfig
	}{
		{
			name: "Happy Path",
			args: args{
				seeds:       []string{"seed0", "seed1", "seed2"},
				clusterName: "cluster-name",
				dcName:      "dc-name",
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "cluster-name",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": NodeConfig{
					"name": "dc-name",
				}},
		},
		{
			name: "Empty seeds",
			args: args{
				seeds:       []string{},
				clusterName: "cluster-name",
				dcName:      "dc-name",
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "cluster-name",
					"seeds": "",
				},
				"datacenter-info": NodeConfig{
					"name": "dc-name",
				}},
		},
		{
			name: "Missing cluster name",
			args: args{
				seeds:       []string{"seed0", "seed1", "seed2"},
				clusterName: "",
				dcName:      "dc-name",
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": NodeConfig{
					"name": "dc-name",
				}},
		},
		{
			name: "Missing dc name",
			args: args{
				seeds:       []string{"seed0", "seed1", "seed2"},
				clusterName: "cluster-name",
				dcName:      "",
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "cluster-name",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": NodeConfig{
					"name": "",
				}},
		},
		{
			name: "Empty args",
			args: args{
				seeds:       nil,
				clusterName: "",
				dcName:      "",
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "",
					"seeds": "",
				},
				"datacenter-info": NodeConfig{
					"name": "",
				}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetModelValues(tt.args.seeds, tt.args.clusterName, tt.args.dcName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetModelValues() = %v, want %v", got, tt.want)
			}
		})
	}
}
