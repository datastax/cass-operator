package gitutil

import (
	"strings"

	"github.com/riptano/dse-operator/mage/sh"
	"github.com/riptano/dse-operator/mage/util"
)

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
