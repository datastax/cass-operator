// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

func TestCassandraDatacenter_buildLabelSelectorForSeedService(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName: "bob",
		},
	}
	want := map[string]string{
		api.ClusterLabel:  "bob",
		api.SeedNodeLabel: "true",
	}

	got := buildLabelSelectorForSeedService(dc)

	if !reflect.DeepEqual(want, got) {
		t.Errorf("buildLabelSelectorForSeedService = %v, want %v", got, want)
	}
}

func TestCassandraDatacenter_allPodsServiceLabels(t *testing.T) {
	dc := &api.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dc1",
		},
		Spec: api.CassandraDatacenterSpec{
			ClusterName: "bob",
		},
	}
	wantLabels := map[string]string{
		oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
		api.ClusterLabel:        "bob",
		api.DatacenterLabel:     "dc1",
		api.PromMetricsLabel:    "true",
	}

	service := newAllPodsServiceForCassandraDatacenter(dc)

	gotLabels := service.ObjectMeta.Labels
	if !reflect.DeepEqual(wantLabels, gotLabels) {
		t.Errorf("allPodsService labels = %v, want %v", gotLabels, wantLabels)
	}
}
