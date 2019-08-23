package reconciliation

import (
	"reflect"
	"testing"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
)

func TestDseDatacenter_buildLabelSelectorForSeedService(t *testing.T) {
	dc := &datastaxv1alpha1.DseDatacenter{
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			DseClusterName: "bob",
		},
	}
	want := map[string]string{
		datastaxv1alpha1.ClusterLabel:  "bob",
		datastaxv1alpha1.SeedNodeLabel: "true",
	}

	got := buildLabelSelectorForSeedService(dc)

	if !reflect.DeepEqual(want, got) {
		t.Errorf("buildLabelSelectorForSeedService = %v, want %v", got, want)
	}
}
