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
	// mage:import kind
	_ "github.com/datastax/cass-operator/mage/kind"
	// mage:import k3d
	_ "github.com/datastax/cass-operator/mage/k3d"
	// mage:import integ
	_ "github.com/datastax/cass-operator/mage/integ-tests"
	// mage:import lint
	_ "github.com/datastax/cass-operator/mage/linting"
	// mage:import gcp
	_ "github.com/datastax/cass-operator/mage/gcloud"
)

// Clean all build artifacts, does not clean up old docker images.
func Clean() {
	mg.Deps(operator.Clean)
	mg.Deps(jenkins.Clean)
}
