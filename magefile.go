//+build mage

package main

import (
	"github.com/magefile/mage/mg"

	// mage:import jenkins
	"github.com/riptano/dse-operator/mage/jenkins"
	// mage:import fallout
	"github.com/riptano/dse-operator/mage/fallout"
	// mage:import operator
	"github.com/riptano/dse-operator/mage/operator"
	// mage:import kind
	_ "github.com/riptano/dse-operator/mage/kind"
)

// Clean all build artifacts, does not clean up old docker images.
func Clean() {
	mg.Deps(operator.Clean)
	mg.Deps(fallout.Clean)
	mg.Deps(jenkins.Clean)
}
