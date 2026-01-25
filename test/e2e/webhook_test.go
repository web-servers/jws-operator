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
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	imagev1 "github.com/openshift/api/image/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "app-img-stream-source-test"
	appName := "image-stream-source-test"
	imageStreamName := "my-img-stream"
	imageStreamNamespace := namespace
	sourceRepositoryURL := "https://gitlab.com/PaulLodge/ocp-app.git"
	sourceRepositoryRef := "main"
	contextDir := "/"
	artifactDir := "my-target"
	secretName := "webhook-secret"
	secretNameUpdate := "webhook-secret-update"
	password := "qwerty"
	passwordUpdate := "asdfg"

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

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"WebHookSecretKey": []byte(password),
		},
	}

	secretUpdate := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNameUpdate,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"WebHookSecretKey": []byte(passwordUpdate),
		},
	}

	webserver := &webserversv1alpha1.WebServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			Replicas:        1,
			ApplicationName: appName,
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
				WebSources: &webserversv1alpha1.WebSourcesSpec{
					SourceRepositoryURL: sourceRepositoryURL,
					SourceRepositoryRef: sourceRepositoryRef,
					ContextDir:          contextDir,
					WebSourcesParams: &webserversv1alpha1.WebSourcesParamsSpec{
						ArtifactDir: artifactDir,
					},
					WebhookSecrets: &webserversv1alpha1.WebhookSecrets{
						Generic: secret.Name,
					},
				},
			},
		},
	}

	BeforeAll(func() {
		createSecret(secret)
		createSecret(secretUpdate)
		createImageStream(imgStream)
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
		deleteImageStream(imgStream)
		deleteSecret(secretUpdate)
		deleteSecret(secret)
	})

	Context("WebHookTest", func() {
		It("GenericTest", func() {
			getURL(name, "/ocp-app/MainApp", []byte{})

			podList := &corev1.PodList{}
			listOpts := []client.ListOption{
				client.InNamespace(webserver.Namespace),
			}

			Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())

			original_count := 0

			for _, pod := range podList.Items {
				if strings.Contains(pod.Name, appName) && strings.Contains(pod.Name, "build") {
					original_count++
				}
			}

			webhookRequest(name, password)

			Eventually(func() bool {
				Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())

				new_count := 0

				for _, pod := range podList.Items {
					if strings.Contains(pod.Name, appName) && strings.Contains(pod.Name, "build") {
						new_count++
					}
				}

				return new_count > original_count
			}, time.Second*30, time.Second*1).Should(BeTrue(), "no new build pod was recognized.")
		})

		It("GenericUpdateTest", func() {
			Eventually(func() bool {
				createdWebserver := getWebServer(name)
				createdWebserver.Spec.WebImageStream.WebSources.WebhookSecrets.Generic = secretNameUpdate
				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*30, time.Second*1).Should(BeTrue(), "not able to update the webserver.")

			getURL(name, "/ocp-app/MainApp", []byte{})

			podList := &corev1.PodList{}
			listOpts := []client.ListOption{
				client.InNamespace(webserver.Namespace),
			}

			Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())

			original_count := 0

			for _, pod := range podList.Items {
				if strings.Contains(pod.Name, appName) && strings.Contains(pod.Name, "build") {
					original_count++
				}
			}

			webhookRequest(name, passwordUpdate)

			Eventually(func() bool {
				Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())

				new_count := 0

				for _, pod := range podList.Items {
					if strings.Contains(pod.Name, appName) && strings.Contains(pod.Name, "build") {
						new_count++
					}
				}

				return new_count > original_count
			}, time.Minute*5, time.Second*1).Should(BeTrue(), "no new build pod was recognized.")
		})
	})
})

func webhookRequest(name string, password string) {
	Eventually(func() bool {
		createdWebServer := getWebServer(name)

		URL := cfg.Host + "/apis/build.openshift.io/v1/namespaces/" + namespace
		URL = URL + "/buildconfigs/" + createdWebServer.Spec.ApplicationName
		URL = URL + "/webhooks/" + password + "/generic"

		fmt.Printf("POST request: %s \n", URL)
		req, err := http.NewRequest("POST", URL, nil)
		if err != nil {
			return false
		}

		transport, err := rest.TransportFor(cfg)
		if err != nil {
			log.Fatalf("Failed to create REST transport: %v", err)
		}

		httpClient := &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		}
		res, err := httpClient.Do(req)

		if err != nil {
			fmt.Printf("Error: %s; \n", err.Error())
			return false
		}

		if res.StatusCode != http.StatusOK {
			fmt.Printf("StatusCode: %d \n", res.StatusCode)
			return false
		}

		return true
	}, time.Minute*5, time.Second*1).Should(BeTrue(), "URL testing failed")
}
