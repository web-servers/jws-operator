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
	"bytes"
	"context"
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"

	//	kbappsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "app-img-source-test"
	appName := "source-test"
	pkgName := "demo" + utils.UnixEpoch()
	testURI := "/" + pkgName + "/demo"
	image := "quay.io/web-servers/tomcat10:latest"
	sourceRepositoryURL := "https://github.com/web-servers/demo-webapp"
	sourceRepositoryRef := "main"
	pushsecret := "secretfortests"
	pushedimage := "quay.io/" + os.Getenv("USER") + "/source-test"
	imagebuilder := "quay.io/web-servers/tomcat10-buildah"
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
				WebApp: &webserversv1alpha1.WebAppSpec{
					Name:                     pkgName + ".war",
					SourceRepositoryURL:      sourceRepositoryURL,
					SourceRepositoryRef:      sourceRepositoryRef,
					WebAppWarImagePushSecret: pushsecret,
					WebAppWarImage:           pushedimage,
					Builder: &webserversv1alpha1.BuilderSpec{
						Image: imagebuilder,
					},
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

	Context("ApplicationImageSourceTest", func() {
		It("Basic Test", func() {
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})

		It("Update Test", func() {
			var createdWebserver *webserversv1alpha1.WebServer

			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				createdWebserver.Spec.WebImage.ApplicationImage = newImage

				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			builderPodLogCheck(appName+"-build", newImage)

			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})

	})
})

func builderPodLogCheck(podName string, expectedString string) {
	container := "war"

	Eventually(func() bool {
		podLogOptions := &corev1.PodLogOptions{
			Container: container,
		}

		podLogs, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions).Stream(ctx)

		if err != nil {
			return false
		}

		var buffer bytes.Buffer
		_, err = io.Copy(&buffer, podLogs)

		if err != nil {
			return false
		}

		Expect(podLogs.Close()).ShouldNot(HaveOccurred())
		return bytes.Contains(bytes.ToLower(buffer.Bytes()), []byte(expectedString))
	}, time.Minute*10, time.Second*1).Should(BeTrue(), "Build Pod: Expected log was not found.")
}
