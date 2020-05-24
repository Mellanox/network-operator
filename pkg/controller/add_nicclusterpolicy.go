package controller

import (
	"github.com/Mellanox/nic-operator/pkg/controller/nicclusterpolicy"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, nicclusterpolicy.Add)
}
