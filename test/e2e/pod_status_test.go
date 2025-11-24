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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	name := "pod-status-test"
	appName := "test-tomcat-demo"

	webserver := &webserversv1alpha1.WebServer{
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

	Context("PodStatusTest", func() {
		It("InicialStartupTest", func() {
			Eventually(func() bool {
				createdWebserver := getWebServer(name)
				pods := createdWebserver.Status.Pods

				for _, status := range pods {
					if status.Name == "" || status.PodIP == "" || status.State != "ACTIVE" {
						return false
					}
				}

				_ = getURL(name, "", []byte{})

				return true
			}, time.Second*60, time.Millisecond*250).Should(BeTrue())
		})

		It("UpdateTest", func() {
			var createdWebserver *webserversv1alpha1.WebServer
			var pods []webserversv1alpha1.PodStatus

			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				pods = createdWebserver.Status.Pods

				for _, status := range pods {
					if status.Name == "" || status.PodIP == "" || status.State == "" {
						return false
					}
				}

				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			createdWebserver.Spec.ApplicationName = appName + "-update"

			Eventually(func() bool {
				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				newPods := createdWebserver.Status.Pods

				for _, status := range newPods {
					if status.Name == "" || status.PodIP == "" || status.State == "" || findPod(pods, status.Name) {
						return false
					}
				}

				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})
	})
})

func findPod(pods []webserversv1alpha1.PodStatus, name string) bool {
	for _, pod := range pods {
		if pod.Name == name {
			return true
		}
	}
	return false
}
