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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	imageStreamName := "my-img-stream"
	imageStreamNamespace := namespace
	sourceRepositoryURL := "https://gitlab.com/PaulLodge/ocp-app.git"
	sourceRepositoryRef := "main"
	contextDir := "/"
	artifactDir := "my-target"
	readinessScript := "cd /opt; test -f jws-${JBOSS_WEBSERVER_VERSION:0:1}.${JBOSS_WEBSERVER_VERSION:2:1}/tomcat/webapps/ocp-app/readiness.jsp"
	livenessScript := "cd /opt; test -f jws-${JBOSS_WEBSERVER_VERSION:0:1}.${JBOSS_WEBSERVER_VERSION:2:1}/tomcat/webapps/ocp-app/liveness.jsp"

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

	Context("LivenessReadinessTest", func() {
		It("ArtifactAndContextDirTest", func() {
			getURL(name, "/ocp-app/MainApp", []byte{})
		})

		It("MavenMirrorTest", func() {
			mavenMirrorURL := "https://maven-central-eu.storage-download.googleapis.com/maven2/"

			Eventually(func() bool {
				createdWebServer := getWebServer(name)
				createdWebServer.Spec.WebImageStream.WebSources.WebSourcesParams.MavenMirrorURL = mavenMirrorURL

				return k8sClient.Update(ctx, createdWebServer) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue(), "Update failed")

			getURL(name, "/ocp-app/MainApp", []byte{})

			podList := &corev1.PodList{}
			listOpts := []client.ListOption{
				client.InNamespace(webserver.Namespace),
			}

			Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())

			found := false

			for _, pod := range podList.Items {
				if strings.Contains(pod.Name, appName) && strings.Contains(pod.Name, "build") {
					result := restClient.Get().
						Namespace(namespace).
						Resource("pods").
						Name(pod.Name).
						SubResource("log").
						Param("container", "sti-build")

					rc, err := result.Stream(ctx)
					if err != nil {
						continue
					}

					data, err := io.ReadAll(rc)
					Expect(rc.Close()).Should(Succeed())

					if err != nil {
						continue
					}

					if bytes.Contains(data, []byte(mavenMirrorURL)) {
						found = true
					}
				}
			}

			Expect(found).Should(BeTrue())
		})

		It("ArtifactAndContextDirUpdateTest", func() {
			Eventually(func() bool {
				createdWebServer := getWebServer(name)
				createdWebServer.Spec.WebImageStream.WebSources.ContextDir = "/subapp"
				createdWebServer.Spec.WebImageStream.WebSources.WebSourcesParams.ArtifactDir = "target"

				return k8sClient.Update(ctx, createdWebServer) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue(), "Update failed")

			_ = getURL(name, "/ocp-subapp", []byte{})
		})

		It("ReadinessScriptTest", func() {
			var createdWebServer *webserversv1alpha1.WebServer

			Eventually(func() bool {
				createdWebServer = getWebServer(name)
				createdWebServer.Spec.WebImageStream.WebSources.ContextDir = contextDir
				createdWebServer.Spec.WebImageStream.WebSources.WebSourcesParams.ArtifactDir = artifactDir
				createdWebServer.Spec.WebImageStream.WebServerHealthCheck = &webserversv1alpha1.WebServerHealthCheckSpec{
					ServerReadinessScript: readinessScript,
				}

				return k8sClient.Update(ctx, createdWebServer) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue(), "Update failed")

			_ = getURL(name, "/ocp-app/MainApp", []byte{})
			cutoffTime := createdWebServer.CreationTimestamp.Time
			_ = getURL(name, "/ocp-app/MainApp?readiness=delete", []byte{})

			eventList := &corev1.EventList{}
			listOpts := []client.ListOption{
				client.InNamespace(webserver.Namespace),
			}

			Eventually(func() bool {
				found := false

				Expect(k8sClient.List(ctx, eventList, listOpts...)).Should(Succeed())

				for _, event := range eventList.Items {
					if event.LastTimestamp.After(cutoffTime) && strings.Contains(event.Message, "Readiness probe failed") == true {
						found = true
						break
					}
				}

				return found

			}, time.Second*60, time.Millisecond*500).Should(BeTrue(), "Event about readiness failure was not found.")
		})

		It("LivenessScriptTest", func() {
			var createdWebServer *webserversv1alpha1.WebServer

			Eventually(func() bool {
				createdWebServer = getWebServer(name)
				createdWebServer.Spec.WebImageStream.WebSources.ContextDir = contextDir
				createdWebServer.Spec.WebImageStream.WebSources.WebSourcesParams.ArtifactDir = artifactDir
				createdWebServer.Spec.WebImageStream.WebServerHealthCheck = &webserversv1alpha1.WebServerHealthCheckSpec{
					ServerLivenessScript: livenessScript,
				}

				return k8sClient.Update(ctx, createdWebServer) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue(), "Update failed")

			_ = getURL(name, "/ocp-app/MainApp", []byte{})
			cutoffTime := createdWebServer.CreationTimestamp.Time
			_ = getURL(name, "/ocp-app/MainApp?liveness=delete", []byte{})

			eventList := &corev1.EventList{}
			listOpts := []client.ListOption{
				client.InNamespace(namespace),
			}

			Eventually(func() bool {
				found := false

				Expect(k8sClient.List(ctx, eventList, listOpts...)).Should(Succeed())

				for _, event := range eventList.Items {
					if event.LastTimestamp.After(cutoffTime) && strings.Contains(event.Message, "Liveness probe failed") == true {
						found = true
						break
					}
				}

				return found

			}, time.Minute*2, time.Second*5).Should(BeTrue(), "Event about liveness failure was not found.")
		})
	})
})
