package integutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	shutil "github.com/riptano/dse-operator/mage/sh"
	mageutil "github.com/riptano/dse-operator/mage/util"
)

const (
	envIntegDir      = "M_INTEG_DIR"
	envGinkgoNoColor = "M_GINKGO_NOCOLOR"
)

func listTestSuiteDirs() []string {
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

/// Prints a comma-delimited list of test suite dirs.
/// This is mainly useful for a CI pipeline.
func List() {
	dirs := listTestSuiteDirs()
	fmt.Println(strings.Join(dirs, ","))
}

func runGinkgoTests(path string) {
	os.Setenv("CGO_ENABLED", "0")
	args := []string{"test", "-timeout", "99999s", "-v"}
	noColor := os.Getenv(envGinkgoNoColor)
	if strings.ToLower(noColor) == "true" {
		args = append(args, "--ginkgo.noColor")
	}

	args = append(args, path)
	shutil.RunVPanic("go", args...)

}

/// Run all ginkgo integration tests.
func RunAll() {
	runGinkgoTests("./tests/...")
}

/// Run a single ginkgo integration test.
///
/// This target requires that the env var
/// M_INTEG_DIR is set to the desired test
/// suite directory name.
func RunSingle() {
	integDir := mageutil.RequireEnv(envIntegDir)
	dir := fmt.Sprintf("./tests/%s", integDir)
	err := os.Chdir(dir)
	mageutil.PanicOnError(err)
	runGinkgoTests("./")
}
