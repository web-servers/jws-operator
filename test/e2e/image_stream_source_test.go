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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	imagev1 "github.com/openshift/api/image/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "app-img-stream-source-test"
	appName := "image-stream-source-test"
	testURI := "/demo-1.0/demo"
	imageStreamName := "my-img-stream"
	imageStreamNamespace := namespace
	sourceRepositoryURL := "https://github.com/web-servers/demo-webapp"
	sourceRepositoryRef := "main"
	sourceRepositoryRefUpdated := "main-updated"

	imgStream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imageStreamName,
			Namespace: namespace,
		},
		Spec: imagev1.ImageStreamSpec{
			Tags: []imagev1.TagReference{
				{
					Name: "latest",
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: testImg,
					},
				},
			},
		},
	}

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
			Replicas:        2,
			ApplicationName: appName,
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
				WebSources: &webserversv1alpha1.WebSourcesSpec{
					SourceRepositoryURL: sourceRepositoryURL,
					SourceRepositoryRef: sourceRepositoryRef,
				},
			},
		},
	}

	BeforeAll(func() {
		createImageStream(imgStream)
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
		deleteImageStream(imgStream)
	})

	Context("ImageStreamSourceTest", func() {
		It("Basic Test", func() {
			testURL(name, testURI, false)
		})

		It("Update Test", func() {
			Eventually(func() bool {
				createdWebServer := getWebServer(name)
				createdWebServer.Spec.WebImageStream.WebSources.SourceRepositoryRef = sourceRepositoryRefUpdated

				return k8sClient.Update(ctx, createdWebServer) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue(), "Update failed")

			testURL(name, testURI, true)
		})
	})
})

func testURL(name string, testURI string, updated bool) {

	Eventually(func() bool {
		createdWebServer := getWebServer(name)

		if len(createdWebServer.Status.Hosts) == 0 {
			return false
		}

		URL := "http://" + createdWebServer.Status.Hosts[0] + testURI

		req, err := http.NewRequest("GET", URL, nil)
		if err != nil {
			return false
		}

		httpClient := &http.Client{}
		res, err := httpClient.Do(req)

		if err != nil || res.StatusCode != http.StatusOK {
			return false
		}

		body, err := io.ReadAll(res.Body)
		Expect(res.Body.Close()).Should(Succeed())
		if err != nil {
			return false
		}

		var result map[string]interface{}

		err = json.Unmarshal(body, &result)

		if err != nil {
			fmt.Println("Error unmarshaling JSON:", err)
			return false
		}

		_, ok := result["counter"]
		value_updated, ok_updated := result["branch"]

		if updated {
			return ok && ok_updated && value_updated == "updated"
		} else {
			return ok
		}
	}, time.Minute*5, time.Millisecond*250).Should(BeTrue(), "URL testing failed")
}
