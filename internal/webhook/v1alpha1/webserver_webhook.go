/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var webserverlog = logf.Log.WithName("webserver-resource")

// SetupWebServerWebhookWithManager registers the webhook for WebServer in the manager.
func SetupWebServerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&webserversv1alpha1.WebServer{}).
		WithValidator(&WebServerCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-web-servers-org-v1alpha1-webserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=web.servers.org,resources=webservers,verbs=create;update;delete,versions=v1alpha1,name=vwebserver-v1alpha1.kb.io,admissionReviewVersions=v1

// WebServerCustomValidator struct is responsible for validating the WebServer resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type WebServerCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &WebServerCustomValidator{}
var webserver_appNames = make(map[string]string)
var appNames_webserver = make(map[string]string)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type WebServer.
func (v *WebServerCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	webserver, ok := obj.(*webserversv1alpha1.WebServer)
	if !ok {
		return nil, fmt.Errorf("expected a WebServer object but got %T", obj)
	}
	webserverlog.Info("Validation for WebServer upon creation", "name", webserver.GetName())
	webserverlog.Info("Validation for WebServer upon creation", "name2", webserver.Name)
	webserverlog.Info("Validation for WebServer upon creation", "app-name", webserver.Spec.ApplicationName)

	printWebServer(webserver)
	return checkApplicationName(webserver)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type WebServer.
func (v *WebServerCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	webserver, ok := newObj.(*webserversv1alpha1.WebServer)
	if !ok {
		return nil, fmt.Errorf("expected a WebServer object for the newObj but got %T", newObj)
	}
	webserverlog.Info("Validation for WebServer upon update", "name", webserver.GetName())

	printWebServer(webserver)
	return checkApplicationName(webserver)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type WebServer.
func (v *WebServerCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	webserver, ok := obj.(*webserversv1alpha1.WebServer)
	if !ok {
		return nil, fmt.Errorf("expected a WebServer object but got %T", obj)
	}
	webserverlog.Info("Validation for WebServer upon deletion", "name", webserver.GetName())

	delete(appNames_webserver, webserver.Spec.ApplicationName)
	delete(webserver_appNames, webserver.Name)

	return nil, nil
}

func printWebServer(webserver *webserversv1alpha1.WebServer) {
	fmt.Printf("name: %s\n", webserver.Name)
	fmt.Printf("applicationName: %s\n", webserver.Spec.ApplicationName)
}

func checkApplicationName(webserver *webserversv1alpha1.WebServer) (admission.Warnings, error) {
	appName := webserver.Spec.ApplicationName
	webserverName := webserver.Name

	webserverNameForAppName, appNameExists := appNames_webserver[appName]

	if appNameExists && webserverNameForAppName == webserverName {
		return nil, nil
	}

	if appNameExists && webserverNameForAppName != webserverName {
		return nil, errors.New("application name is already used")
	}

	if !appNameExists && webserverNameForAppName != webserverName {
		old_appName, webserverExists := webserver_appNames[webserverName]

		if webserverExists {
			delete(appNames_webserver, old_appName)
		}
		webserver_appNames[webserverName] = appName
		appNames_webserver[appName] = webserverName
	}

	return nil, nil
}
