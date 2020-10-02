// Copyright DataStax, Inc.
// Please see the included license file for details.

package integutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	k3d "github.com/datastax/cass-operator/mage/k3d"
	kind "github.com/datastax/cass-operator/mage/kind"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

const (
	envIntegDir       = "M_INTEG_DIR"
	envGinkgoNoColor  = "M_GINKGO_NOCOLOR"
	envLoadTestImages = "M_LOAD_TEST_IMAGES"
	envK8sFlavor      = "M_K8S_FLAVOR"
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

type testType int

const (
	OSS testType = iota
	UBI_OSS

	DSE
	UBI_DSE

	// NOTE: This line MUST be last in the const expression
	TestTypeEnumLength int = iota
)

func getTestType(testDir string) testType {
	lower := strings.ToLower(testDir)
	isOss := false
	isUbi := false

	if strings.Contains(lower, "_oss") || strings.Contains(lower, "oss_") {
		isOss = true
	}

	if strings.Contains(lower, "_ubi") || strings.Contains(lower, "ubi_") {
		isUbi = true
	}

	if isOss && isUbi {
		return UBI_OSS

	} else if isOss {
		return OSS
	} else if !isOss && isUbi {
		return UBI_DSE
	} else {
		return DSE
	}
}

func loadImagesFromBuildSettings(ca cfgutil.ClusterActions, bs cfgutil.BuildSettings, testType testType) {
	dev := bs.Dev
	// always load shared images
	images := dev.SharedImages

	// also load specific images based on test type
	switch testType {
	case DSE:
		images = append(images, dev.DseImages...)
	case UBI_DSE:
		images = append(images, dev.UbiDseImages...)
	case OSS:
		images = append(images, dev.OssImages...)
	case UBI_OSS:
		images = append(images, dev.UbiOssImages...)

	}

	for _, image := range images {
		// we likely don't always care if we fail to pull
		// because we could be testing local images
		_ = shutil.RunV("docker", "pull", image)
		ca.LoadImage(image)
	}
}

func loadClusterActions() cfgutil.ClusterActions {
	clusterType := mageutil.EnvOrDefault(envK8sFlavor, "kind")

	// We only care about pulling and loading images
	// in this library, so we have a more narrow list of
	// supported flavors than the more general k8s mage library
	var supportedFlavors = map[string]cfgutil.ClusterActions{
		"kind": kind.ClusterActions,
		"k3d":  k3d.ClusterActions,
	}

	if cfg, ok := supportedFlavors[clusterType]; ok {
		return cfg
	} else {
		panic(fmt.Sprintf("Unsupported %s specified: %s", envK8sFlavor, clusterType))
	}
}

func loadImagesForTest(testDir string) {
	clusterActions := loadClusterActions()
	buildSettings := cfgutil.ReadBuildSettings()
	testType := getTestType(testDir)
	loadImagesFromBuildSettings(clusterActions, buildSettings, testType)
}

// Run ginkgo integration tests.
//
// Default behavior is to discover and run
// all test suites located under the ./tests/ directory.
//
// To run a subset of test suites, specify the name of the suite
// directories in env var M_INTEG_DIR, separated by a comma
//
// To pull and load images based on test type (DSE, OSS, UBI)
// set the env var M_LOAD_TEST_IMAGES to true
//
// Examples:
// M_INTEG_DIR=scale_up,stop_resume
// M_LOAD_TEST_IMAGES=true M_INTEG_DIR=scale_up
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
		loadTestImages := os.Getenv(envLoadTestImages)
		if strings.ToLower(loadTestImages) == "true" {
			fmt.Println("Pulling and loading images from buildsettings.yaml")
			loadImagesForTest(dir)
		}

		runGinkgoTestSuite(path)
	}
}
