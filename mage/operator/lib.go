package operator

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/riptano/dse-operator/mage/util"
	"gopkg.in/yaml.v2"
)

const (
	rootBuildDir               = "./build"
	sdkBuildDir                = "operator/build"
	operatorSdkImage           = "operator-sdk-binary"
	generatedDseDataCentersCrd = "operator/deploy/crds/datastax.com_dsedatacenters_crd.yaml"
	packagePath                = "github.com/riptano/dse-operator/operator"
	buildSettings              = "buildsettings.yaml"
	envVersionString           = "MO_VERSION"

	errorUnstagedPreGenerate = `
  Unstaged changes detected.
  - Please clean your working tree of
    uncommitted changes before running this target.`

	errorUnstagedPostGenerate = `
  Unstaged changes found after running "operator-sdk generate"
  - This indicates that the operator-sdk
    updated some boilerplate in your working tree.
  - You may be able commit these changes if you have
    intentionally modified a resource spec and wish
    to update the sdk boilerplate, but be careful
    with backwards compatibility.`
)

func writeBuildFile(fileName string, contents string) {
	mageutil.EnsureDir(rootBuildDir)
	outputPath := filepath.Join(rootBuildDir, fileName)
	err := ioutil.WriteFile(outputPath, []byte(contents+"\n"), 0666)
	if err != nil {
		fmt.Printf("Failed to write file at %s\n", outputPath)
		panic(err)
	}
}

func runGoModVendor() {
	os.Setenv("GO111MODULE", "on")
	mageutil.RunV("go", "mod", "tidy")
	mageutil.RunV("go", "mod", "download")
	mageutil.RunV("go", "mod", "vendor")
}

// Generate operator-sdk-binary docker image
func createSdkDockerImage() {
	mageutil.RunV("docker", "build", "-t", operatorSdkImage, "tools/operator-sdk")
}

// generate the files and clean up afterwards
func generateK8sAndOpenApi() {
	cwd, _ := os.Getwd()
	runArgs := []string{"-t"}
	execArgs := []string{
		"/bin/bash", "-c",
		"export GO111MODULE=on; cd ../../riptano/dse-operator/operator && operator-sdk generate k8s && operator-sdk generate openapi && rm -rf build"}
	volumes := []string{fmt.Sprintf("%s:/go/src/github.com/riptano/dse-operator", cwd)}
	out, err := mageutil.RunDocker(operatorSdkImage, volumes, nil, nil, runArgs, execArgs)
	mageutil.PanicOnError(err)
	if out != "" {
		fmt.Println(out)
	}
}

type yamlWalker struct {
	yaml      map[interface{}]interface{}
	err       error
	editsMade bool
}

func (y *yamlWalker) walk(key string) {
	if y.err != nil {
		return
	}
	val, ok := y.yaml[key]
	if !ok {
		y.err = fmt.Errorf("walk failed on %s", key)
	} else {
		y.yaml = val.(map[interface{}]interface{})
	}
}

func (y *yamlWalker) remove(key string) {
	if y.err != nil {
		return
	}
	delete(y.yaml, key)
	y.editsMade = true
}

func (y *yamlWalker) update(key string, val interface{}) {
	if y.err != nil {
		return
	}
	y.yaml[key] = val
	y.editsMade = true
}

func (y *yamlWalker) get(key string) (interface{}, bool) {
	val, ok := y.yaml[key]
	return val, ok
}

func ensurePreserveUnknownFields(data map[interface{}]interface{}) yamlWalker {
	// Ensure the openAPI and k8s allow unrecognized fields.
	// See postProcessCrd for why.
	walker := yamlWalker{yaml: data, err: nil, editsMade: false}
	preserve := "x-kubernetes-preserve-unknown-fields"
	walker.walk("spec")
	walker.walk("validation")
	walker.walk("openAPIV3Schema")
	if presVal, exists := walker.get(preserve); !exists || presVal != true {
		walker.update(preserve, true)
	}
	return walker
}

func removeConfigSection(data map[interface{}]interface{}) yamlWalker {
	// Strip the config section from the CRD.
	// See postProcessCrd for why.	x := data["spec"].(t)
	walker := yamlWalker{yaml: data, err: nil, editsMade: false}
	walker.walk("spec")
	walker.walk("validation")
	walker.walk("openAPIV3Schema")
	walker.walk("properties")
	walker.walk("spec")
	walker.walk("properties")
	if _, exists := walker.get("config"); exists {
		walker.remove("config")
	}
	return walker
}

func postProcessCrd() {
	// Remove the "config" section from the CRD, and enable
	// x-kubernetes-preserve-unknown-fields.
	//
	// This is necessary because the config field has a dynamic
	// schema which depends on the DSE version selected, and
	// dynamic schema aren't possible to fully specify and
	// validate via openAPI V3.
	//
	// Instead, we remove the config field from the schema
	// entirely and instruct openAPI/k8s to preserve fields even
	// if they aren't specified in the CRD. The field itself is defined
	// as a json.RawMessage, see dsedatacenter_types.go in the
	// api's subdirectory for details.
	//
	// We might be able to remove this when this lands:
	// [kubernetes-sigs/controller-tools#345](https://github.com/kubernetes-sigs/controller-tools/pull/345)

	var data map[interface{}]interface{}
	d, err := ioutil.ReadFile(generatedDseDataCentersCrd)
	mageutil.PanicOnError(err)

	err = yaml.Unmarshal(d, &data)
	mageutil.PanicOnError(err)

	w1 := ensurePreserveUnknownFields(data)
	mageutil.PanicOnError(w1.err)

	w2 := removeConfigSection(data)
	mageutil.PanicOnError(w2.err)

	if w1.editsMade || w2.editsMade {
		updated, err := yaml.Marshal(data)
		mageutil.PanicOnError(err)

		err = ioutil.WriteFile(generatedDseDataCentersCrd, updated, os.ModePerm)
		mageutil.PanicOnError(err)
	}
}

func doSdkGenerate() {
	cwd, _ := os.Getwd()
	os.Chdir("operator")
	runGoModVendor()
	os.Chdir(cwd)

	// This is needed for operator-sdk generate k8s to run
	os.MkdirAll(sdkBuildDir, os.ModePerm)
	mageutil.RunV("touch", fmt.Sprintf("%s/Dockerfile", sdkBuildDir))

	generateK8sAndOpenApi()
	postProcessCrd()
}

func hasUnstagedChanges() bool {
	out, err := sh.Output("git", "diff")
	mageutil.PanicOnError(err)
	return strings.TrimSpace(out) != ""
}

func hasStagedChanges() bool {
	out, err := sh.Output("git", "diff", "--staged")
	mageutil.PanicOnError(err)
	return strings.TrimSpace(out) != ""
}

// Generate files with the operator-sdk.
//
// This launches a docker container and executes `operator-sdk generate`
// with the k8s and kube-openapi code-generators
//
// The k8s code-generator currently only generates DeepCopy() functions
// for all custom resource types under pkg/apis/...
//
// The kube-openapi code-generator generates a crd yaml file for
// every custom resource under pkg/apis/... that are tagged for OpenAPIv3.
// The generated crd files are located under deploy/crds/...
func SdkGenerate() {
	fmt.Println("- Updating operator-sdk generated files")
	createSdkDockerImage()
	doSdkGenerate()
}

// Test that asserts that boilerplate files generated by the operator SDK are up to date.
//
// Ensures that we don't change the DseDatacenterSpec without also regenerating
// the boilerplate files that the Operator SDK manages which depend on that spec.
//
// Note: this test WILL UPDATE YOUR WORKING DIRECTORY if it fails.
// There is no dry run mode for sdk generation, so this test simply
// tries to do it and fails if there are uncommitted changes to your
// working directory afterward.
func TestSdkGenerate() {
	fmt.Println("- Asserting that generated files are already up to date")
	if hasUnstagedChanges() {
		err := fmt.Errorf(errorUnstagedPreGenerate)
		panic(err)
	}
	createSdkDockerImage()
	doSdkGenerate()
	if hasUnstagedChanges() {
		err := fmt.Errorf(errorUnstagedPostGenerate)
		panic(err)
	}
}

type Version struct {
	Major      int    `yaml:"major"`
	Minor      int    `yaml:"minor"`
	Patch      int    `yaml:"patch"`
	Prerelease string `yaml:"prerelease"`
}

func (v Version) String() string {
	version := fmt.Sprintf("%v.%v.%v", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		pre := ensureAlphaNumericDash(v.Prerelease)
		version = fmt.Sprintf("%v-%v", version, pre)
	}
	return version
}

type BuildSettings struct {
	Version Version `yaml:"version"`
}

type GitData struct {
	Branch                string
	LongHash              string
	HasUncommittedChanges bool
}

func getGitBranch() string {
	var branch string
	if b := os.Getenv("MO_BRANCH"); b != "" {
		branch = b
	} else {
		head, err := sh.Output("git", "rev-parse", "--abbrev-ref", "HEAD")
		mageutil.PanicOnError(err)
		branch = head
	}
	return strings.TrimSpace(branch)
}

func getGitLongHash() string {
	hash, err := sh.Output("git", "rev-parse", "HEAD")
	mageutil.PanicOnError(err)
	return strings.TrimSpace(hash)
}

func getGitData() GitData {
	return GitData{
		Branch:                getGitBranch(),
		HasUncommittedChanges: hasStagedChanges() || hasUnstagedChanges(),
		LongHash:              getGitLongHash(),
	}
}

func readBuildSettings() BuildSettings {
	var settings BuildSettings
	d, err := ioutil.ReadFile(buildSettings)
	mageutil.PanicOnError(err)

	err = yaml.Unmarshal(d, &settings)
	mageutil.PanicOnError(err)

	return settings
}

func ensureAlphaNumericDash(str string) string {
	r := regexp.MustCompile("[^A-z0-9\\-]")
	return r.ReplaceAllString(str, "-")
}

type FullVersion struct {
	Core        Version
	Branch      string
	Uncommitted bool
	Hash        string
}

func trimFullVersionBranch(v FullVersion) FullVersion {
	str := v.String()
	overflow := len(str) - 128
	if overflow > 0 {
		v.Branch = v.Branch[:len(v.Branch)-overflow]
	}
	return v
}

func (v FullVersion) String() string {
	str := fmt.Sprintf("%v", v.Core)
	if v.Core.Prerelease != "" {
		str = fmt.Sprintf("%s.", str)
	} else {
		str = fmt.Sprintf("%s-", str)
	}
	if v.Branch != "master" {
		sanitized := ensureAlphaNumericDash(v.Branch)
		str = fmt.Sprintf("%s%s.", str, sanitized)
	}
	if v.Uncommitted {
		str = fmt.Sprintf("%suncommitted.", str)
	}
	str = fmt.Sprintf("%s%s", str, v.Hash)
	return str
}

func calcFullVersion(settings BuildSettings, git GitData) FullVersion {
	return FullVersion{
		Core:        settings.Version,
		Branch:      getGitBranch(),
		Uncommitted: git.HasUncommittedChanges,
		Hash:        git.LongHash,
	}
}

func runDockerBuild(version FullVersion) []string {
	versionedTag := fmt.Sprintf("datastax/dse-operator:%v", version)
	tagsToPush := []string{
		versionedTag,
		fmt.Sprintf("datastax/dse-operator:%s", version.Hash),
	}
	tags := append(tagsToPush, "datastax/dse-operator:latest")
	buildArgs := []string{fmt.Sprintf("VERSION_STAMP=%s", versionedTag)}
	mageutil.BuildDocker(".", "./operator/Dockerfile", tags, buildArgs)
	return tagsToPush
}

func runGoBuild(version string) {
	os.Chdir("./operator")
	os.Setenv("CGO_ENABLED", "0")
	binaryPath := fmt.Sprintf("../build/bin/dse-operator-%s-%s", runtime.GOOS, runtime.GOARCH)
	goArgs := []string{
		"build", "-o", binaryPath,
		"-ldflags", fmt.Sprintf("-X main.version=%s", version),
		fmt.Sprintf("%s/cmd/manager", packagePath),
	}
	mageutil.RunV("go", goArgs...)
	os.Chdir("..")

}

// Builds operator go code.
//
// By default, a dev version will be stamped into
// the binary.
//
// Set env variable MO_VERSION to specify a specific
// version to stamp.
func BuildGo() {
	mg.Deps(Clean)
	fmt.Println("- Building operator go module")
	version := mageutil.EnvOrDefault(envVersionString, "DEV")
	runGoBuild(version)
}

// Runs unit tests for operator go code.
func TestGo() {
	fmt.Println("- Running go unit tests")
	os.Chdir("./operator")
	os.Setenv("CGO_ENABLED", "0")
	goArgs := []string{"test", "./..."}
	mageutil.RunV("go", goArgs...)
	os.Chdir("..")
}

// Runs unit tests for operator mage library.
//
// Since we have a good amount of logic around building
// and versioning the operator, we want to make sure that
// the logic is sound.
func TestMage() {
	fmt.Println("- Running operator mage unit tests")
	os.Chdir("./mage/operator")
	goArgs := []string{"test"}
	mageutil.RunV("go", goArgs...)
	os.Chdir("../../")
}

// Builds Docker image for the operator.
//
// This step will also build and test the operator go code.
// The docker image will be tagged based on the state
// of your git working tree.
func BuildDocker() {
	fmt.Println("- Building operator docker image")
	settings := readBuildSettings()
	git := getGitData()
	version := calcFullVersion(settings, git)
	tags := runDockerBuild(version)
	// Write the versioned image tags to a file in our build
	// directory so that other targets in the build process can identify
	// what was built. This is particularly important to know
	// for targets that retag and deploy to external docker repositories
	outputText := strings.Join(tags, "|")
	writeBuildFile("tagsToPush.txt", outputText)
}

// Alias for buildDocker target
func Build() {
	mg.Deps(BuildDocker)
}

// Run all automated test targets
func Test() {
	mg.Deps(TestMage)
	mg.Deps(TestGo)
	mg.Deps(TestSdkGenerate)
}

// Remove the operator build directories, and the top-level build directory.
//
// It's maybe a bit weird that this clean target reaches up out of it's
// directory to clean a top level thing, but right now that top-level thing
// holds the operator golang binary, so we clean it here.
func Clean() {
	os.RemoveAll(sdkBuildDir)
	os.RemoveAll(rootBuildDir)
}
