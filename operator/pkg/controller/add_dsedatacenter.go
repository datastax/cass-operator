package controller

import (
	"github.com/riptano/dse-operator/operator/pkg/controller/dsedatacenter"
	"github.com/riptano/dse-operator/operator/pkg/controller/dsenode"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, dsedatacenter.Add, dsenode.Add)
}
