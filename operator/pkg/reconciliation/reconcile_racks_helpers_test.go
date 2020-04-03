package reconciliation

import (
	"testing"
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
