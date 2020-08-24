package controller

import (
	"jws-image-operator/pkg/controller/tomcat"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, tomcat.Add)
}
