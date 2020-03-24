//+build mage

package main

import (
	"github.com/magefile/mage/mg"

	// mage:import jenkins
	"github.com/riptano/dse-operator/mage/jenkins"
	// mage:import operator
	"github.com/riptano/dse-operator/mage/operator"
	// mage:import kind
	_ "github.com/riptano/dse-operator/mage/kind"
	// mage:import integ
	_ "github.com/riptano/dse-operator/mage/integ-tests"
	// mage:import lint
	_ "github.com/riptano/dse-operator/mage/linting"
)

// Clean all build artifacts, does not clean up old docker images.
func Clean() {
	mg.Deps(operator.Clean)
	mg.Deps(jenkins.Clean)
}
