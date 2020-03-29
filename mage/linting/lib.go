// Copyright DataStax, Inc.
// Please see the included license file for details.

package linting

import (
	"fmt"
	"os"
	"strings"

	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

const (
	envLintDirs = "M_LINT_DIRS"
)

func runGolangCiLint(dir string) error {
	cwd, err := os.Getwd()
	mageutil.PanicOnError(err)
	err = os.Chdir(dir)
	mageutil.PanicOnError(err)

	lintErr := shutil.RunV("golangci-lint", "run")

	err = os.Chdir(cwd)
	mageutil.PanicOnError(err)

	return lintErr
}

// Run golangci-lint tool.
//
// Several directories will be linted by default.
// To specify a custom list of dirs instead,
// populate M_LINT_DIRS with a comma-delimited
// list of paths.
func Run() {
	os.Setenv("CGO_ENABLED", "0")
	envDirs := os.Getenv(envLintDirs)
	var lintDirs []string
	if envDirs != "" {
		// Support the user specifying a comma-delimited
		// list of directories to lint instead of defaults
		lintDirs = strings.Split(envDirs, ",")
	} else {
		lintDirs = []string{"mage", "operator", "tests"}
	}

	for _, dir := range lintDirs {
		fmt.Println("==================================")
		fmt.Printf(" Linting in directory: %s\n", dir)
		fmt.Println("==================================")
		err := runGolangCiLint(dir)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

}
