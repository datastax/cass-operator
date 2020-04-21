// Copyright DataStax, Inc.
// Please see the included license file for details.

package integutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

const (
	envIntegDir      = "M_INTEG_DIR"
	envGinkgoNoColor = "M_GINKGO_NOCOLOR"
)

func getTestSuiteDirs() []string {
	contents, err := ioutil.ReadDir("./tests")
	mageutil.PanicOnError(err)
	var testDirs []string
	for _, info := range contents {
		if info.IsDir() && info.Name() != "testdata" {
			testDirs = append(testDirs, info.Name())
		}
	}
	return testDirs
}

// Prints a comma-delimited list of test suite dirs.
// This is mainly useful for a CI pipeline.
func List() {
	dirs := getTestSuiteDirs()
	fmt.Println(strings.Join(dirs, ","))
}

func runGinkgoTestSuite(path string) {
	os.Setenv("CGO_ENABLED", "0")
	args := []string{
		"test",
		"-timeout", "99999s",
		"-v",
		"--ginkgo.v",
		"--ginkgo.progress",
	}
	noColor := os.Getenv(envGinkgoNoColor)
	if strings.ToLower(noColor) == "true" {
		args = append(args, "--ginkgo.noColor")
	}
	args = append(args, path)

	cwd, _ := os.Getwd()

	err := os.Chdir(path)
	mageutil.PanicOnError(err)

	shutil.RunVPanic("go", args...)

	err = os.Chdir(cwd)
	mageutil.PanicOnError(err)
}

// Run ginkgo integration tests.
//
// Default behavior is to discover and run
// all test suites located under the ./tests/ directory.
//
// To run a subset of test suites, specify the name of the suite
// directories in env var M_INTEG_DIR, separated by a comma
//
// Example:
// M_INTEG_DIR=scale_up,stop_resume
//
// This target assumes that helm is installed and available on path.
func Run() {
	var testDirs []string
	integDir := os.Getenv(envIntegDir)
	if integDir != "" {
		testDirs = strings.Split(integDir, ",")
	} else {
		testDirs = getTestSuiteDirs()
	}

	for _, dir := range testDirs {
		path := fmt.Sprintf("./tests/%s", dir)
		runGinkgoTestSuite(path)
	}
}
