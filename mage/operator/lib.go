package operator

import (
	"fmt"
	"os"
	"io/ioutil"

	"gopkg.in/yaml.v2"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/riptano/dse-operator/mage/util"
)

const (
	buildDir         = "operator/build"
	operatorSdkImage = "operator-sdk-binary"
	generatedDseDataCentersCrd = "operator/deploy/crds/datastax.com_dsedatacenters_crd.yaml"
)

func runGoModVendor() {
	os.Setenv("GO111MODULE", "on")
	sh.Run("go", "mod", "tidy")
	sh.Run("go", "mod", "download")
	sh.Run("go", "mod", "vendor")
}

// Generate operator-sdk-binary docker image
func CreateOperatorSdkDockerImage() {
	sh.RunV("docker", "build", "-t", operatorSdkImage, "tools/operator-sdk")
}

// generate the files and clean up afterwards
func doUpdateGeneratedFiles() {
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
	yaml map[interface{}]interface{}
	err error
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
	walker := yamlWalker{ yaml:data, err:nil, editsMade:false }
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
	walker := yamlWalker{ yaml:data, err:nil, editsMade:false }
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

//Update files generated with operator-sdk
func UpdateGeneratedFiles() {
	fmt.Println("- Updating operator-sdk generated files")
	mg.Deps(CreateOperatorSdkDockerImage)
	cwd, _ := os.Getwd()
	os.Chdir("operator")
	runGoModVendor()
	os.Chdir(cwd)

	// This is needed for operator-sdk generate k8s to run
	os.MkdirAll(buildDir, os.ModePerm)
	sh.Run("touch", fmt.Sprintf("%s/Dockerfile", buildDir))

	doUpdateGeneratedFiles()
	postProcessCrd()
}
