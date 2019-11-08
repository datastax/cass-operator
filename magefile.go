//+build mage

package main

import (
	// mage:import jenkins
	_ "github.com/riptano/dse-operator/mage/jenkins"
	// mage:import fallout
	_ "github.com/riptano/dse-operator/mage/fallout"
	// mage:import operator
	_ "github.com/riptano/dse-operator/mage/operator"
)
