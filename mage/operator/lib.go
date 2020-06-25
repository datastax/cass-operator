// Copyright DataStax, Inc.
// Please see the included license file for details.

package operator

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	dockerutil "github.com/datastax/cass-operator/mage/docker"
	gitutil "github.com/datastax/cass-operator/mage/git"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
	"github.com/magefile/mage/mg"
	"gopkg.in/yaml.v2"
)

const (
	dockerBase                 = "./operator/docker/base/Dockerfile"
	dockerUbi                  = "./operator/docker/ubi/Dockerfile"
	rootBuildDir               = "./build"
	sdkBuildDir                = "operator/build"
	diagramsDir                = "./docs/developer/diagrams"
	operatorSdkImage           = "operator-sdk-binary"
	testSdkImage               = "operator-sdk-binary-tester"
	genClientImage             = "operator-gen-client"
	mermaidJsImage             = "operator-mermaid-js"
	generatedDseDataCentersCrd = "operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml"
	helmChartCrd               = "charts/cass-operator-chart/templates/customresourcedefinition.yaml"
	packagePath                = "github.com/datastax/cass-operator/operator"
	envGitBranch               = "MO_BRANCH"
	envVersionString           = "MO_VERSION"
	envGitHash                 = "MO_HASH"
	EnvBaseOs                  = "MO_BASE_OS"

	errorUnstagedPreGenerate = `
  Unstaged changes detected.
  - Please clean your working tree of
    uncommitted changes before running this target.`

	errorUnstagedPostSdkGenerate = `
  Unstaged changes found after running "operator-sdk generate"
  - This indicates that the operator-sdk
    updated some boilerplate in your working tree.
  - You may be able commit these changes if you have
    intentionally modified a resource spec and wish
    to update the sdk boilerplate, but be careful
    with backwards compatibility.`

	errorUnstagedPostClientGenerate = `
  Unstaged changes found after running the generate-groups.sh
  script from the k8s code-generator.
  - This indicates that the code-generator
    updated some boilerplate in your working tree.
  - You may be able commit these changes if you have
    intentionally modified something that caused a
    client change and wish to update the client boilerplate,
    but be careful with backwards compatibility.`
)

// check unstaged changes and exit with the changes if they are present
func checkForUnstagedChanges(message string) {
	if unstagedChanges := gitutil.GetUnstagedChanges(); unstagedChanges != "" {
		err := fmt.Errorf(message + "\n\n" + unstagedChanges)
		panic(err)
	}
}

func writeBuildFile(fileName string, contents string) {
	mageutil.EnsureDir(rootBuildDir)
	outputPath := filepath.Join(rootBuildDir, fileName)
	err := ioutil.WriteFile(outputPath, []byte(contents), 0666)
	if err != nil {
		fmt.Printf("Failed to write file at %s\n", outputPath)
		panic(err)
	}
}

func runGoModVendor() {
	os.Setenv("GO111MODULE", "on")
	shutil.RunVPanic("go", "mod", "tidy")
	shutil.RunVPanic("go", "mod", "download")
	shutil.RunVPanic("go", "mod", "vendor")
}

// Generate operator-sdk-binary docker image
func createSdkDockerImage() {
	dockerutil.Build("./", "install-operator-sdk", "tools/operator-sdk/Dockerfile",
		[]string{operatorSdkImage}, nil).ExecVPanic()
}

// Generate operator-sdk-binary-tester docker image
func createTestSdkDockerImage() {
	dockerutil.Build("./", "test-operator-sdk", "tools/operator-sdk/Dockerfile",
		[]string{testSdkImage}, nil).ExecVPanic()
}

// generate the files and clean up afterwards
func generateK8sAndOpenApi() {
	cwd, _ := os.Getwd()
	runArgs := []string{"-t", "--rm"}
	repoPath := "/go/src/github.com/datastax/cass-operator"
	execArgs := []string{
		"/bin/bash", "-c",
		fmt.Sprintf("set -eufx; export GO111MODULE=on; cd %s/operator && operator-sdk generate k8s && operator-sdk generate crds && rm -rf build", repoPath),
	}
	volumes := []string{fmt.Sprintf("%s:%s", cwd, repoPath)}
	dockerutil.Run(operatorSdkImage, volumes, nil, nil, runArgs, execArgs).ExecVPanic()
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
	shutil.RunVPanic("touch", fmt.Sprintf("%s/Dockerfile", sdkBuildDir))

	generateK8sAndOpenApi()
	postProcessCrd()
	patchCrdToTemplate()
}

func cpCrdToChart() {
	crd, err := ioutil.ReadFile(generatedDseDataCentersCrd)
	mageutil.PanicOnError(err)

	err = ioutil.WriteFile(helmChartCrd, crd, os.ModePerm)
	mageutil.PanicOnError(err)
}

func patchCrdToTemplate() {
	shutil.RunVPanic("patch", generatedDseDataCentersCrd, "mage/operator/crd.patch", "-o", helmChartCrd)
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
	checkForUnstagedChanges(errorUnstagedPreGenerate)
	createSdkDockerImage()
	doSdkGenerate()
	checkForUnstagedChanges(errorUnstagedPostSdkGenerate)
}

// Tests the operator-sdk itself.
//
// Uses the example project and kubernetes CLI tools. This
// does not test the DSE operator code in any way.
func TestSdk() {
	fmt.Println("- Testing the operator-sdk itself")
	createSdkDockerImage()
	createTestSdkDockerImage()
}

type GitData struct {
	Branch                string
	LongHash              string
	HasUncommittedChanges bool
}

func getGitData() GitData {
	return GitData{
		Branch:                gitutil.GetBranch(envGitBranch),
		HasUncommittedChanges: gitutil.HasStagedChanges() || gitutil.HasUnstagedChanges(),
		LongHash:              gitutil.GetLongHash(envGitHash),
	}
}

type FullVersion struct {
	Core        cfgutil.Version
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
		sanitized := cfgutil.EnsureAlphaNumericDash(v.Branch)
		str = fmt.Sprintf("%s%s.", str, sanitized)
	}
	if v.Uncommitted {
		str = fmt.Sprintf("%suncommitted.", str)
	}
	str = fmt.Sprintf("%s%s", str, v.Hash)
	return str
}

func calcFullVersion(settings cfgutil.BuildSettings, git GitData) FullVersion {
	return FullVersion{
		Core:        settings.Version,
		Branch:      git.Branch,
		Uncommitted: git.HasUncommittedChanges,
		Hash:        git.LongHash,
	}
}

func calcVersionAndTags(version FullVersion, ubiBase bool) (string, []string) {
	repoPath := "datastax/cass-operator"
	var versionedTag string
	var tagsToPush []string

	if ubiBase {
		versionedTag = fmt.Sprintf("%s:%v-ubi", repoPath, version)
		tagsToPush = []string{
			versionedTag,
			fmt.Sprintf("%s:%s-ubi", repoPath, version.Hash),
			fmt.Sprintf("%s:latest-ubi", repoPath),
		}
	} else {
		versionedTag = fmt.Sprintf("%s:%v", repoPath, version)
		tagsToPush = []string{
			versionedTag,
			fmt.Sprintf("%s:%s", repoPath, version.Hash),
			fmt.Sprintf("%s:latest", repoPath),
		}
	}

	return versionedTag, tagsToPush
}

func runDockerBuild(versionedTag string, dockerTags []string, extraBuildArgs []string, dockerfile string) {
	buildArgs := []string{fmt.Sprintf("VERSION_STAMP=%s", versionedTag)}
	buildArgs = append(buildArgs, extraBuildArgs...)
	dockerutil.Build(".", "", dockerfile, dockerTags, buildArgs).ExecVPanic()
}

func runGoBuild(version string) {
	os.Chdir("./operator")
	os.Setenv("CGO_ENABLED", "0")
	binaryPath := fmt.Sprintf("../build/bin/cass-operator-%s-%s", runtime.GOOS, runtime.GOARCH)
	goArgs := []string{
		"build", "-o", binaryPath,
		"-ldflags", fmt.Sprintf("-X main.version=%s", version),
		fmt.Sprintf("%s/cmd/manager", packagePath),
	}
	shutil.RunVPanic("go", goArgs...)
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
	shutil.RunVPanic("go", goArgs...)
	os.Chdir("..")
}

// Runs unit tests for operator mage library.
//
// Since we have a good amount of logic around building
// and versioning the operator, we want to make sure that
// the logic is sound.
func TestMage() {
	fmt.Println("- Running operator mage unit tests")
	os.Setenv("CGO_ENABLED", "0")
	os.Chdir("./mage/operator")
	goArgs := []string{"test"}
	shutil.RunVPanic("go", goArgs...)
	os.Chdir("../../")
}

// Builds Docker image for the operator.
//
// This step will also build and test the operator go code.
// The docker image will be tagged based on the state
// of your git working tree.
func BuildDocker() {
	fmt.Println("- Building operator docker image")
	settings := cfgutil.ReadBuildSettings()
	git := getGitData()
	version := calcFullVersion(settings, git)

	//build regular docker image
	versionedTag, dockerTags := calcVersionAndTags(version, false)
	runDockerBuild(versionedTag, dockerTags, nil, dockerBase)

	if baseOs := os.Getenv(EnvBaseOs); baseOs != "" {
		//build ubi docker image
		args := []string{fmt.Sprintf("BASE_OS=%s", baseOs)}
		ubiVersionedTag, ubiDockerTags := calcVersionAndTags(version, true)
		runDockerBuild(ubiVersionedTag, ubiDockerTags, args, dockerUbi)
		dockerTags = append(dockerTags, ubiDockerTags...)
	}

	// Write the versioned image tags to a file in our build
	// directory so that other targets in the build process can identify
	// what was built. This is particularly important to know
	// for targets that retag and deploy to external docker repositories
	outputText := strings.Join(dockerTags, "|")
	writeBuildFile("tagsToPush.txt", outputText)
}

func buildMermaidJsDockerImage() {
	dockerutil.Build("./tools/mermaid-js", "", "./tools/mermaid-js/Dockerfile",
		[]string{mermaidJsImage}, []string{}).ExecVPanic()
}

func generateDocDiagram(in string, out string) {
	runArgs := []string{}
	execArgs := []string{
		"-i", in,
		"-o", out,
	}
	diagramsDirAbs, err := filepath.Abs(diagramsDir)
	mageutil.PanicOnError(err)
	volumes := []string{fmt.Sprintf("%s:%s", diagramsDirAbs, "/data")}
	dockerutil.Run(mermaidJsImage, volumes, nil, nil, runArgs, execArgs).ExecVPanic()
}

func doGenerateDocDiagrams() {
	diagramSrcExt := ".mmd"
	svgExt := ".svg"
	files, err := ioutil.ReadDir(diagramsDir)
	mageutil.PanicOnError(err)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), diagramSrcExt) {
			base := strings.TrimSuffix(file.Name(), diagramSrcExt)
			svg := fmt.Sprintf("%s%s", base, svgExt)
			generateDocDiagram(file.Name(), svg)
		}
	}
}

func buildCodeGeneratorDockerImage() {
	// Use the version of code-generator that we are pinned to
	// in operator/go.mod.
	var genVersion string
	f, err := os.Open("./go.mod")
	mageutil.PanicOnError(err)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Contains(txt, "code-generator =>") {
			versionIdx := strings.LastIndex(txt, " ")
			genVersion = txt[versionIdx+1:]
			break
		}
	}
	mageutil.PanicOnError(scanner.Err())
	fmt.Println(genVersion)
	dockerutil.Build("./", "", "./tools/k8s-code-generator/Dockerfile",
		[]string{genClientImage}, []string{fmt.Sprintf("CODEGEN_VERSION=%s", genVersion)}).ExecVPanic()

}

func doGenerateClient() {
	cwd, _ := os.Getwd()
	usr, err := user.Current()
	mageutil.PanicOnError(err)
	runArgs := []string{"-t", "--rm", "-u", fmt.Sprintf("%s:%s", usr.Uid, usr.Gid)}
	execArgs := []string{"client", "github.com/datastax/cass-operator/operator/pkg/generated",
		"github.com/datastax/cass-operator/operator/pkg/apis", "cassandra:v1beta1"}
	volumes := []string{fmt.Sprintf("%s/operator:/go/src/github.com/datastax/cass-operator/operator", cwd)}
	dockerutil.Run(genClientImage, volumes, nil, nil, runArgs, execArgs).ExecVPanic()
}

// Generate diagrams for docs.
func GenerateDiagrams() {
	buildMermaidJsDockerImage()
	doGenerateDocDiagrams()
}

// Gen operator client code.
//
// Uses k8s code-generator to generate client code that
// resides in the operator/pkg/generated/clientset/ directory.
func GenerateClient() {
	buildCodeGeneratorDockerImage()
	doGenerateClient()
}

// Asserts that generated client boilerplate files are up to date.
//
// Note: this test WILL UPDATE YOUR WORKING DIRECTORY if it fails.
// There is no dry run mode for code-generation, so this test simply
// tries to do it and fails if there are uncommitted changes to your
// working directory afterward.
func TestGenerateClient() {
	fmt.Println("- Asserting that generated client files are already up to date")
	checkForUnstagedChanges(errorUnstagedPreGenerate)
	buildCodeGeneratorDockerImage()
	doGenerateClient()
	checkForUnstagedChanges(errorUnstagedPostClientGenerate)
}

// Alias for buildDocker target
func Build() {
	mg.Deps(BuildDocker)
}

// Run all automated test targets
func Test() {
	mg.Deps(TestMage)
	mg.Deps(TestGo)
	mg.Deps(TestSdk)
	mg.Deps(TestSdkGenerate)
	mg.Deps(TestGenerateClient)
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

// Run go tests and build a docker image if they pass
func TestAndBuild() {
	mg.SerialDeps(TestGo, BuildDocker)
}
