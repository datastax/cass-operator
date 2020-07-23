// Copyright DataStax, Inc.
// Please see the included license file for details.

package serverconfig

import (
	"reflect"
	"testing"
)

func TestGetModelValues(t *testing.T) {
	type args struct {
		seeds        []string
		clusterName  string
		dcName       string
		graphEnabled int
		solrEnabled  int
		sparkEnabled int
	}
	tests := []struct {
		name string
		args args
		want NodeConfig
	}{
		{
			name: "Happy Path",
			args: args{
				seeds:        []string{"seed0", "seed1", "seed2"},
				clusterName:  "cluster-name",
				dcName:       "dc-name",
				graphEnabled: 1,
				solrEnabled:  0,
				sparkEnabled: 0,
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "cluster-name",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": NodeConfig{
					"graph-enabled": 1,
					"name":          "dc-name",
					"solr-enabled":  0,
					"spark-enabled": 0,
				}},
		},
		{
			name: "Empty seeds",
			args: args{
				seeds:        []string{},
				clusterName:  "cluster-name",
				dcName:       "dc-name",
				graphEnabled: 0,
				solrEnabled:  1,
				sparkEnabled: 0,
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "cluster-name",
					"seeds": "",
				},
				"datacenter-info": NodeConfig{
					"graph-enabled": 0,
					"name":          "dc-name",
					"solr-enabled":  1,
					"spark-enabled": 0,
				}},
		},
		{
			name: "Missing cluster name",
			args: args{
				seeds:        []string{"seed0", "seed1", "seed2"},
				clusterName:  "",
				dcName:       "dc-name",
				graphEnabled: 1,
				solrEnabled:  1,
				sparkEnabled: 1,
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": NodeConfig{
					"graph-enabled": 1,
					"name":          "dc-name",
					"solr-enabled":  1,
					"spark-enabled": 1,
				}},
		},
		{
			name: "Missing dc name",
			args: args{
				seeds:        []string{"seed0", "seed1", "seed2"},
				clusterName:  "cluster-name",
				dcName:       "",
				graphEnabled: 0,
				solrEnabled:  0,
				sparkEnabled: 1,
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "cluster-name",
					"seeds": "seed0,seed1,seed2",
				},
				"datacenter-info": NodeConfig{
					"graph-enabled": 0,
					"name":          "",
					"solr-enabled":  0,
					"spark-enabled": 1,
				}},
		},
		{
			name: "Empty args",
			args: args{
				seeds:        nil,
				clusterName:  "",
				dcName:       "",
				graphEnabled: 0,
				solrEnabled:  0,
				sparkEnabled: 0,
			},
			want: NodeConfig{
				"cluster-info": NodeConfig{
					"name":  "",
					"seeds": "",
				},
				"datacenter-info": NodeConfig{
					"graph-enabled": 0,
					"name":          "",
					"solr-enabled":  0,
					"spark-enabled": 0,
				}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetModelValues(tt.args.seeds, tt.args.clusterName, tt.args.dcName, tt.args.graphEnabled, tt.args.solrEnabled, tt.args.sparkEnabled); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetModelValues() = %v, want %v", got, tt.want)
			}
		})
	}
}
