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

	name := "validation-test"
	appName := "test-tomcat"

	Context("InputValicationTest", func() {
		It("NameTest", func() {
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MyWebServer",
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

			Expect(k8sClient.Create(ctx, webserver)).ShouldNot(Succeed())
		})

		It("ApplicationNameTest", func() {
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: "My App",
					Replicas:        int32(2),
					WebImage: &webserversv1alpha1.WebImageSpec{
						ApplicationImage: testImg,
					},
				},
			}

			Expect(k8sClient.Create(ctx, webserver)).ShouldNot(Succeed())
		})

		It("ReplicaTest", func() {
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: appName,
					Replicas:        int32(-2),
					WebImage: &webserversv1alpha1.WebImageSpec{
						ApplicationImage: testImg,
					},
				},
			}

			Expect(k8sClient.Create(ctx, webserver)).ShouldNot(Succeed())
		})

		It("WebImageTest", func() {
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: appName,
					Replicas:        int32(2),
					WebImage: &webserversv1alpha1.WebImageSpec{
						ImagePullSecret: "secretfortests",
					},
				},
			}

			Expect(k8sClient.Create(ctx, webserver)).ShouldNot(Succeed())
		})

		It("WebImageTest", func() {
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: appName,
					Replicas:        int32(2),
					WebImage: &webserversv1alpha1.WebImageSpec{
						ImagePullSecret: "secretfortests",
					},
				},
			}

			Expect(k8sClient.Create(ctx, webserver)).ShouldNot(Succeed())
		})

		It("WebImageStreamTest-I", func() {
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: appName,
					Replicas:        int32(2),
					WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
						ImageStreamNamespace: "openshift",
					},
				},
			}

			Expect(k8sClient.Create(ctx, webserver)).ShouldNot(Succeed())
		})

		It("WebImageStreamTest-II", func() {
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: appName,
					Replicas:        int32(2),
					WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
						ImageStreamName: "img-stream-test",
					},
				},
			}

			Expect(k8sClient.Create(ctx, webserver)).ShouldNot(Succeed())
		})
	})
})
