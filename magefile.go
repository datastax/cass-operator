// Copyright DataStax, Inc.
// Please see the included license file for details.

//+build mage

package main

import (
	"github.com/magefile/mage/mg"

	// mage:import jenkins
	"github.com/datastax/cass-operator/mage/jenkins"
	// mage:import operator
	"github.com/datastax/cass-operator/mage/operator"
	// mage:import integ
	_ "github.com/datastax/cass-operator/mage/integ-tests"
	// mage:import lint
	_ "github.com/datastax/cass-operator/mage/linting"
	// mage:import k8s
	_ "github.com/datastax/cass-operator/mage/k8s"
)

// Clean all build artifacts, does not clean up old docker images.
func Clean() {
	mg.Deps(operator.Clean)
	mg.Deps(jenkins.Clean)
}
