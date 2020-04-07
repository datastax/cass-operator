package reconciliation

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
)

func TestMapContains(t *testing.T) {
	labels := make(map[string]string)
	labels["key1"] = "val1"
	labels["key2"] = "val2"
	labels["key3"] = "val3"

	selectors := make(map[string]string)
	selectors["key1"] = "val1"
	selectors["key3"] = "val3"

	isMatch := mapContains(labels, selectors)

	if !isMatch {
		t.Fatalf("mapContains should have found match.")
	}
}

func TestMapContainsDoesntContain(t *testing.T) {
	labels := make(map[string]string)
	labels["key1"] = "val1"
	labels["key2"] = "val2"
	labels["key3"] = "val3"

	selectors := make(map[string]string)
	selectors["key1"] = "val4"
	selectors["key3"] = "val3"

	isMatch := mapContains(labels, selectors)

	if isMatch {
		t.Fatalf("mapContains should not have found match.")
	}
}

func TestPodPrtsFromPodList(t *testing.T) {
	pod1 := corev1.Pod{}
	pod1.Name = "pod1"

	pod2 := corev1.Pod{}
	pod2.Name = "pod2"

	pod3 := corev1.Pod{}
	pod3.Name = "pod3"
	podList := corev1.PodList{
		Items: []corev1.Pod{pod1, pod2, pod3},
	}

	prts := PodPtrsFromPodList(&podList)

	expectedNames := []string{"pod1", "pod2", "pod3"}
	var actualNames []string
	for _, p := range prts {
		actualNames = append(actualNames, p.Name)
	}
	assert.ElementsMatch(t, expectedNames, actualNames)

}
