// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"

	"../mage"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	buildDir         = "build"
	operatorSdkImage = "operator-sdk-binary"
)

func runGoModVendor() {
	os.Setenv("GO111MODULE", "on")
	sh.Run("go", "mod", "vendor")
}

// This is needed for operator-sdk generate k8s to run
func touchBuildDockerfile() {
	os.MkdirAll(buildDir, os.ModePerm)
	sh.Run("touch", fmt.Sprintf("%s/Dockerfile", buildDir))
}

// Generate operator-sdk-binary docker image
func CreateOperatorSdkDockerImage() {
	sh.Run("docker", "build", "-t", operatorSdkImage, "../tools/operator-sdk")
}

// generate the files and clean up afterwards
func doUpdateGeneratedFiles() {
	os.Chdir("..")
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

//Update files generated with operator-sdk
func UpdateGeneratedFiles() {
	fmt.Println("- Updating operator-sdk generated files")
	mg.Deps(CreateOperatorSdkDockerImage)
	runGoModVendor()
	touchBuildDockerfile()
	// A perl one-liner seems to be more portable and readable than sed/awk solutions
	// Remove lines between config: and type:
	doUpdateGeneratedFiles()
	os.Chdir("..")
	err := exec.Command("perl", "-i", "-ne", "print unless /config:/ .. /type:/", "operator/deploy/crds/datastax_v1alpha1_dsedatacenter_crd.yaml").Run()
	if err != nil {
		panic(err)
	}
}
