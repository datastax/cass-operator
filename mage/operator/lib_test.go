// Copyright DataStax, Inc.
// Please see the included license file for details.

package operator

import (
	"testing"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	"github.com/stretchr/testify/assert"
)

func TestMageOperator_trimFullVersionBranch_ok(t *testing.T) {
	v := cfgutil.Version{
		Major:      1,
		Minor:      2,
		Patch:      3,
		Prerelease: "alpha-1",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "something-short",
		Uncommitted: true,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}
	want := "something-short"
	got := trimFullVersionBranch(full).Branch
	assert.Equal(t, got, want)
}
func TestMageOperator_trimFullVersionBranch_overflow(t *testing.T) {
	v := cfgutil.Version{
		Major:      1,
		Minor:      2,
		Patch:      3,
		Prerelease: "alpha-1",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "something-long-really-should-be-trimmed-to-avoid-128-chars-or-we-cant-push",
		Uncommitted: true,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}
	trimmed := trimFullVersionBranch(full)
	want := 128
	got := len(trimmed.String())
	assert.Equal(t, got, want)
}

func TestMageOperator_fullVersion_string_simple(t *testing.T) {
	v := cfgutil.Version{
		Major:      9,
		Minor:      3,
		Patch:      7,
		Prerelease: "",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "ko-123-test",
		Uncommitted: false,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}

	want := "9.3.7-ko-123-test.341fe22e8af310fd7694a0c5db4f065131799047"
	got := full.String()
	assert.Equal(t, got, want)
}

func TestMageOperator_fullVersion_string_prerelease(t *testing.T) {
	v := cfgutil.Version{
		Major:      9,
		Minor:      3,
		Patch:      7,
		Prerelease: "alpha-9",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "ko-123-test",
		Uncommitted: false,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}

	want := "9.3.7-alpha-9.ko-123-test.341fe22e8af310fd7694a0c5db4f065131799047"
	got := full.String()
	assert.Equal(t, got, want)
}

func TestMageOperator_fullVersion_string_uncommitted(t *testing.T) {
	v := cfgutil.Version{
		Major:      9,
		Minor:      3,
		Patch:      7,
		Prerelease: "",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "ko-123-test",
		Uncommitted: true,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}

	want := "9.3.7-ko-123-test.uncommitted.341fe22e8af310fd7694a0c5db4f065131799047"
	got := full.String()
	assert.Equal(t, got, want)
}

func TestMageOperator_fullVersion_string_prerelease_uncommitted(t *testing.T) {
	v := cfgutil.Version{
		Major:      9,
		Minor:      3,
		Patch:      7,
		Prerelease: "alpha-9",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "ko-123-test",
		Uncommitted: true,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}

	want := "9.3.7-alpha-9.ko-123-test.uncommitted.341fe22e8af310fd7694a0c5db4f065131799047"
	got := full.String()
	assert.Equal(t, got, want)
}

func TestMageOperator_fullVersion_string_master(t *testing.T) {
	v := cfgutil.Version{
		Major:      9,
		Minor:      3,
		Patch:      7,
		Prerelease: "",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "master",
		Uncommitted: false,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}

	want := "9.3.7-341fe22e8af310fd7694a0c5db4f065131799047"
	got := full.String()
	assert.Equal(t, got, want)
}

func TestMageOperator_fullVersion_string_master_prerelease(t *testing.T) {
	v := cfgutil.Version{
		Major:      9,
		Minor:      3,
		Patch:      7,
		Prerelease: "beta-2",
	}
	full := FullVersion{
		Core:        v,
		Branch:      "master",
		Uncommitted: false,
		Hash:        "341fe22e8af310fd7694a0c5db4f065131799047",
	}

	want := "9.3.7-beta-2.341fe22e8af310fd7694a0c5db4f065131799047"
	got := full.String()
	assert.Equal(t, got, want)
}
