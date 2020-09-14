package controller

import (
	"github.com/web-servers/jws-image-operator/pkg/controller/jbosswebserver"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, jbosswebserver.Add)
}
