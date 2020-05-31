package controller

import (
	"github.com/Mellanox/mellanox-network-operator/pkg/controller/nicclusterpolicy"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, nicclusterpolicy.Add)
}
