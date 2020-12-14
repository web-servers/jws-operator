package controller

import (
	"github.com/web-servers/jws-operator/pkg/controller/webserver"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, webserver.Add)
}
