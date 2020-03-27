package controller

import (
	"github.com/datastax/cass-operator/operator/pkg/controller/cassandradatacenter"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, cassandradatacenter.Add)
}
