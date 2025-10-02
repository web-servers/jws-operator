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

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	kbappsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "image-basic-test"
	appName := "test-tomcat-demo"
	testURI := "/health"
	image := "quay.io/web-servers/tomcat10:latest"
	newImage := "quay.io/web-servers/tomcat10update:latest"

	webserver := &webserversv1alpha1.WebServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			ApplicationName: appName,
			Replicas:        int32(2),
			WebImage: &webserversv1alpha1.WebImageSpec{
				ApplicationImage: image,
			},
		},
	}

	BeforeAll(func() {
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
	})

	Context("ApplicationImageTest", func() {
		It("Basic Test", func() {
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})

		It("Scale Test", func() {
			// scale up test.
			utils.WebServerScale(k8sClient, ctx, thetest, webserver, testURI, 4)

			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())

			// scale down test.
			utils.WebServerScale(k8sClient, ctx, thetest, webserver, testURI, 1)

			_, err = utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})

		It("Update Test", func() {
			createdWebserver := getWebServer(name)

			// Update WebImage and update WebServer
			Eventually(func() bool {
				createdWebserver.Spec.WebImage.ApplicationImage = newImage

				err := k8sClient.Update(ctx, createdWebserver)
				if err != nil {
					thetest.Logf("WebServer update failed:  %s\n", err)
					return false
				}
				thetest.Logf("WebServer %s updated\n", createdWebserver.Name)
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			foundDeployment := &kbappsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundDeployment)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			foundImage := foundDeployment.Spec.Template.Spec.Containers[0].Image
			Expect(foundImage == newImage).Should(BeTrue(), "Image Update Test: image check failed")

			// Wait until the replicas are available
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundDeployment)
				if err != nil {
					thetest.Fatalf("can't read Deployment")
					return false
				}

				if int32(createdWebserver.Spec.Replicas) == int32(foundDeployment.Status.AvailableReplicas) {
					return true
				} else {
					return false
				}
			}, time.Second*420, time.Second*30).Should(BeTrue(), "Image Update Test: Required amount of replicas were not achieved")

			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})
	})
})
