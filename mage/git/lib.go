// Copyright DataStax, Inc.
// Please see the included license file for details.

package gitutil

import (
	"strings"

	"github.com/datastax/cass-operator/mage/sh"
	"github.com/datastax/cass-operator/mage/util"
)

func GetUnstagedChanges() string {
	out := shutil.OutputPanic("git", "--no-pager", "diff")
	return strings.TrimSpace(out)
}

func HasUnstagedChanges() bool {
	out := shutil.OutputPanic("git", "diff")
	return strings.TrimSpace(out) != ""
}

func HasStagedChanges() bool {
	out := shutil.OutputPanic("git", "diff", "--staged")
	return strings.TrimSpace(out) != ""
}

// First check env var for branch value
// and fall back to executing git cli
func GetBranch(env string) string {
	var gitFunc = func() string {
		branch := shutil.OutputPanic("git", "rev-parse", "--abbrev-ref", "HEAD")
		return branch
	}
	val := mageutil.FromEnvOrF(env, gitFunc)
	return strings.TrimSpace(val)
}

// First check env var for hash value
// and fall back to executing git cli
func GetLongHash(env string) string {
	var gitFunc = func() string {
		hash := shutil.OutputPanic("git", "rev-parse", "HEAD")
		return hash
	}
	val := mageutil.FromEnvOrF(env, gitFunc)
	return strings.TrimSpace(val)
}
