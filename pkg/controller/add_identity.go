package controller

import (
	"github.com/redhat-cop/namespace-configuration-operator/pkg/controller/identity"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, identity.Add)
}
