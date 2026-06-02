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

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "image-basic-test"
	appName := "test-tomcat-demo"
	testURI := "/health"

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
				ApplicationImage: testImg,
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
			// Checks if Web Server was deployed with the PREVIOUS version of the image
			waitForPodsActiveState(name)
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})

		It("Update Test", func() {
			var createdWebserver *webserversv1alpha1.WebServer

			// Update WebImage and update WebServer
			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				createdWebserver.Spec.WebImage.ApplicationImage = testImgUpdate

				err := k8sClient.Update(ctx, createdWebserver)
				if err != nil {
					thetest.Logf("WebServer update failed:  %s\n", err)
					return false
				}
				thetest.Logf("WebServer %s updated\n", createdWebserver.Name)
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			waitForPodsActiveState(name)
			isApplicationImageWasUpdated(createdWebserver, testImg)

			// Checks if Web Server was deployed with the NEW version of the image
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})

		It("Operator Log Test", func() {
			checkOperatorLogs()
		})
	})
})
