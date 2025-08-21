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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	//	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	webserversorgv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
)

var c client.Client

var _ = Describe("WebServer Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "mm-test", // TODO(user):Modify as needed
		}
		webserver := &webserversorgv1alpha1.WebServer{
			TypeMeta: metav1.TypeMeta{
				Kind:       "WebServer",
				APIVersion: "web.servers.org/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "xyz",
				Namespace: "mm-test",
			},
			Spec: webserversorgv1alpha1.WebServerSpec{
				Replicas:        3,
				ApplicationName: "abc",
				WebImage: &webserversorgv1alpha1.WebImageSpec{
					ApplicationImage: "quay.io/web-servers/tomcat10:latest",
				},
			},
		}

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &WebServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			err := c.Create(ctx, webserver)

			Expect(err).NotTo(HaveOccurred())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
