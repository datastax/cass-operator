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
		want DseConfigMap
	}{
		{
			name: "Happy Path",
			args: args{
				seeds:       []string{"seed0", "seed1", "seed2"},
				clusterName: "cluster-name",
				dcName:      "dc-name",
			},
			want: DseConfigMap{
				"cluster-info": DseConfigMap{
					"name":  "cluster-name",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": DseConfigMap{
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
			want: DseConfigMap{
				"cluster-info": DseConfigMap{
					"name":  "cluster-name",
					"seeds": "",
				},
				"datacenter-info": DseConfigMap{
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
			want: DseConfigMap{
				"cluster-info": DseConfigMap{
					"name":  "",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": DseConfigMap{
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
			want: DseConfigMap{
				"cluster-info": DseConfigMap{
					"name":  "cluster-name",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": DseConfigMap{
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
			want: DseConfigMap{
				"cluster-info": DseConfigMap{
					"name":  "",
					"seeds": "",
				},
				"datacenter-info": DseConfigMap{
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
