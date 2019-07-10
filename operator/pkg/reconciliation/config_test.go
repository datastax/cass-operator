package reconciliation

import (
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"reflect"
	"testing"
)

func Test_mergeToDepth(t *testing.T) {
	tests := []struct {
		name  string
		mapA  DseConfigMap
		mapB  DseConfigMap
		depth int32
		want  DseConfigMap
	}{
		{
			name: "basic merge",
			mapA: DseConfigMap{
				"a": 1,
				"b": 2,
			},
			mapB: DseConfigMap{
				"a": 3,
				"c": 4,
			},
			depth: 0,
			want: DseConfigMap{
				"a": 3,
				"b": 2,
				"c": 4,
			},
		},
		{
			name: "recurse on maps",
			mapA: DseConfigMap{
				"a": DseConfigMap{
					"d": 1,
					"e": 2,
				},
				"b": 2,
			},
			mapB: DseConfigMap{
				"a": DseConfigMap{
					"d": 3,
					"f": 4,
				},
				"c": 4,
			},
			depth: 1,
			want: DseConfigMap{
				"a": DseConfigMap{
					"d": 3,
					"e": 2,
					"f": 4,
				},
				"b": 2,
				"c": 4,
			},
		},
		{
			name: "respect depth",
			mapA: DseConfigMap{
				"a": DseConfigMap{
					"d": 1,
					"e": 2,
				},
				"b": 2,
			},
			mapB: DseConfigMap{
				"a": DseConfigMap{
					"d": 3,
					"f": 4,
				},
				"c": 4,
			},
			depth: 0,
			want: DseConfigMap{
				"a": DseConfigMap{
					"d": 3,
					"f": 4,
				},
				"b": 2,
				"c": 4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeToDepth(tt.mapA, tt.mapB, tt.depth); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeToDepth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GenerateBaseConfig(t *testing.T) {
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			Config:      "{}",
			ClusterName: "bobs-cluster",
		},
	}
	result, err := GenerateBaseConfig(dseDatacenter)
	if err != nil {
		t.Errorf("Got error %v", err)
	}
	if name := result["cluster-info"].(DseConfigMap)["name"]; name != dseDatacenter.Spec.ClusterName {
		t.Errorf("Found cluster name of %v, want %v", name, dseDatacenter.Spec.ClusterName)
	}
}
