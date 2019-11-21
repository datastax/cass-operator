package operator

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/magefile/mage/sh"
	"github.com/riptano/dse-operator/mage/util"
	"gopkg.in/yaml.v2"
)

const (
	buildDir                   = "operator/build"
	operatorSdkImage           = "operator-sdk-binary"
	generatedDseDataCentersCrd = "operator/deploy/crds/datastax.com_dsedatacenters_crd.yaml"
)

func runGoModVendor() {
	os.Setenv("GO111MODULE", "on")
	sh.Run("go", "mod", "tidy")
	sh.Run("go", "mod", "download")
	sh.Run("go", "mod", "vendor")
}

// Generate operator-sdk-binary docker image
func createSdkDockerImage() {
	sh.RunV("docker", "build", "-t", operatorSdkImage, "tools/operator-sdk")
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
	fmt.Println(out)
	if err != nil {
		panic(err)
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
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(d, &data)
	if err != nil {
		panic(err)
	}
	w1 := ensurePreserveUnknownFields(data)
	if w1.err != nil {
		panic(w1.err)
	}

	w2 := removeConfigSection(data)
	if w2.err != nil {
		panic(w2.err)
	}

	if w1.editsMade || w2.editsMade {
		updated, err := yaml.Marshal(data)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(generatedDseDataCentersCrd, updated, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
}

func doSdkGenerate() {
	cwd, _ := os.Getwd()
	os.Chdir("operator")
	runGoModVendor()
	os.Chdir(cwd)

	// This is needed for operator-sdk generate k8s to run
	os.MkdirAll(buildDir, os.ModePerm)
	sh.Run("touch", fmt.Sprintf("%s/Dockerfile", buildDir))

	generateK8sAndOpenApi()
	postProcessCrd()
}

func assertCleanGitDiff() error {
    return sh.Run("git", "diff", "--quiet")
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
	if err := assertCleanGitDiff(); err != nil {
		fmt.Println("Failed to get a success response from git diff.")
		fmt.Println("Please clean your working tree of uncommitted changes before running this target.")
		panic(err)
	}
	createSdkDockerImage()
	doSdkGenerate()
	if err := assertCleanGitDiff(); err != nil {
		fmt.Println("Failed to get a success response from git diff.")
		fmt.Println("- This is indicates that `operator-sdk generate` tried")
		fmt.Println("  to update some boilerplate in your working tree.")
		fmt.Println("- You may be able commit these changes if you have")
		fmt.Println("  intentionally modified a resource spec and wish")
		fmt.Println("  to update the sdk boilerplate, but be careful")
		fmt.Println("  with backwards compatibility.")
		panic(err)
	}
}
