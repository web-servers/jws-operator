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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	kbappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	name := "resource-test"
	appName := "test-tomcat-demo"
	testURI := "/health"

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
			PodResources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		},
	}

	BeforeAll(func() {
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
	})

	Context("PodResourcesTest", func() {
		It("Basic Test", func() {
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})

		It("UpdateTest", func() {
			// Update labals and update the WebServer
			Eventually(func() bool {
				createdWebserver := getWebServer(name)

				createdWebserver.Spec.PodResources.Limits = corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("750m"),
					corev1.ResourceMemory: resource.MustParse("2512Mi"),
				}

				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				deployment := &kbappsv1.Deployment{}
				deploymentookupKey := types.NamespacedName{Name: appName, Namespace: namespace}

				if k8sClient.Get(ctx, deploymentookupKey, deployment) != nil {
					return false
				}

				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					fmt.Printf("number of containers")
					return false
				}

				if deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu() == nil {
					fmt.Printf("cpu limit is nil")
					return false
				}

				return deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Cmp(resource.MustParse("750m")) == 0

			}, time.Second*120, time.Millisecond*250).Should(BeTrue())
		})

	})
})
