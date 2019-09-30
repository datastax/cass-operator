package httphelper

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
)

func Test_BuildPodHostFromPod(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-foo",
			Namespace: "somenamespace",
			Labels: map[string]string{
				datastaxv1alpha1.DatacenterLabel: "dc-bar",
				datastaxv1alpha1.ClusterLabel:    "the-foobar-cluster",
			},
		},
	}

	result := BuildPodHostFromPod(pod)
	expected := "pod-foo.the-foobar-cluster-dc-bar-service.somenamespace"

	assert.Equal(t, expected, result)
}
