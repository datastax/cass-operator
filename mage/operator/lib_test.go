package operator

import (
	"gotest.tools/assert"
	"testing"
)

func TestMageOperator_versionSuffix_simple(t *testing.T) {
	git := GitData{
		Branch:             "ko-123-test",
		HasStagedChanges:   false,
		HasUnstagedChanges: false,
		ShortHash:          "abcdef",
	}
	want := "ko.123.test.abcdef"
	got := versionSuffix(git)
	assert.Equal(t, got, want)
}

func TestMageOperator_versionSuffix_staged(t *testing.T) {
	git := GitData{
		Branch:             "ko-123-test",
		HasStagedChanges:   true,
		HasUnstagedChanges: false,
		ShortHash:          "abcdef",
	}
	want := "ko.123.test.uncommitted.abcdef"
	got := versionSuffix(git)
	assert.Equal(t, got, want)
}

func TestMageOperator_versionSuffix_unstaged(t *testing.T) {
	git := GitData{
		Branch:             "ko-123-test",
		HasStagedChanges:   false,
		HasUnstagedChanges: true,
		ShortHash:          "abcdef",
	}
	want := "ko.123.test.uncommitted.abcdef"
	got := versionSuffix(git)
	assert.Equal(t, got, want)
}

func TestMageOperator_calcFullVersion_simple(t *testing.T) {
	git := GitData{
		Branch:             "ko-123-test",
		HasStagedChanges:   false,
		HasUnstagedChanges: false,
		ShortHash:          "abcdef",
	}

	settings := BuildSettings{
		Version: Version{
			Major:      9,
			Minor:      3,
			Patch:      7,
			Prerelease: "",
		},
	}

	want := "9.3.7-ko.123.test.abcdef"
	got := calcFullVersion(settings, git)
	assert.Equal(t, got, want)
}

func TestMageOperator_calcFullVersion_prerelease(t *testing.T) {
	git := GitData{
		Branch:             "ko-123-test",
		HasStagedChanges:   false,
		HasUnstagedChanges: false,
		ShortHash:          "abcdef",
	}

	settings := BuildSettings{
		Version: Version{
			Major:      9,
			Minor:      3,
			Patch:      7,
			Prerelease: "alpha-9",
		},
	}

	want := "9.3.7-alpha.9.ko.123.test.abcdef"
	got := calcFullVersion(settings, git)
	assert.Equal(t, got, want)
}

func TestMageOperator_calcFullVersion_master(t *testing.T) {
	git := GitData{
		Branch:             "master",
		HasStagedChanges:   false,
		HasUnstagedChanges: false,
		ShortHash:          "abcdef",
	}

	settings := BuildSettings{
		Version: Version{
			Major:      9,
			Minor:      3,
			Patch:      7,
			Prerelease: "",
		},
	}

	want := "9.3.7-abcdef"
	got := calcFullVersion(settings, git)
	assert.Equal(t, got, want)
}

func TestMageOperator_calcFullVersion_master_prerelease(t *testing.T) {
	git := GitData{
		Branch:             "master",
		HasStagedChanges:   false,
		HasUnstagedChanges: false,
		ShortHash:          "abcdef",
	}

	settings := BuildSettings{
		Version: Version{
			Major:      9,
			Minor:      3,
			Patch:      7,
			Prerelease: "alpha",
		},
	}

	want := "9.3.7-alpha.abcdef"
	got := calcFullVersion(settings, git)
	assert.Equal(t, got, want)
}
