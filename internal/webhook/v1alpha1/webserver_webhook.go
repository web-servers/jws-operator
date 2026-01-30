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
	"sync"

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

type AppRegistry struct {
	mu      sync.RWMutex
	allApps map[string][]WebApp
}

type WebApp struct {
	WebServerName string
	AppName       string
}

var _ webhook.CustomValidator = &WebServerCustomValidator{}
var registry = AppRegistry{
	allApps: make(map[string][]WebApp),
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type WebServer.
func (v *WebServerCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	webserver, ok := obj.(*webserversv1alpha1.WebServer)
	if !ok {
		return nil, fmt.Errorf("expected a WebServer object but got %T", obj)
	}
	webserverlog.Info("Validation for WebServer upon creation", "name", webserver.GetName())

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

	list := registry.allApps[webserver.Namespace]
	for i, web := range list {
		if web.WebServerName == webserver.Name {
			// Remove the item by slicing around it
			registry.allApps[webserver.Namespace] = append(list[:i], list[i+1:]...)
			break
		}
	}

	return nil, nil
}

func printWebServer(webserver *webserversv1alpha1.WebServer) {
	fmt.Printf("name: %s\n", webserver.Name)
	fmt.Printf("applicationName: %s\n", webserver.Spec.ApplicationName)
	fmt.Printf("namespace: %s\n", webserver.Namespace)
}

func checkApplicationName(webserver *webserversv1alpha1.WebServer) (admission.Warnings, error) {
	// Acquire a Write Lock immediately
	registry.mu.Lock()
	// Ensure the lock is released when the function exits
	defer registry.mu.Unlock()

	appName := webserver.Spec.ApplicationName
	webserverName := webserver.Name
	namespace := webserver.Namespace

	list := registry.allApps[namespace]

	for i := range list {
		// Webserver is managing existing app - OK
		if list[i].WebServerName == webserverName && list[i].AppName == appName {
			return nil, nil
		}

		// Webserver updated app name - need to search whether app name is available in that namespace
		if list[i].WebServerName == webserverName && list[i].AppName != appName {
			for j := range list {
				if list[j].AppName == appName {
					// Application name already used - return error
					return nil, errors.New("application name is already used")
				}
			}
			// Application name can be used
			list[i].AppName = appName
			return nil, nil
		}

		// Application name is used by other webserver
		if list[i].WebServerName != webserverName && list[i].AppName == appName {
			return nil, errors.New("application name is already used")
		}
	}

	// Application name can be used
	registry.allApps[namespace] = append(registry.allApps[namespace], WebApp{WebServerName: webserverName, AppName: appName})

	return nil, nil
}
