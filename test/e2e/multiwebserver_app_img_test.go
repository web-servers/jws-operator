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
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	imagev1 "github.com/openshift/api/image/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	name := "multiwebserver-app-img-test"
	appName := "test-tomcat-demo"
	numberOfConcurrentWebServers := int32(3)
	imageStreamName := "img-stream-test"

	var concurrentExecution = func(i int) {
		defer GinkgoRecover()

		_ = getURL(name+"-"+strconv.Itoa(i), "", []byte{})
		time.Sleep(time.Second * time.Duration(rand.IntN(10)+1))

		createdWebserver := getWebServer(name + "-" + strconv.Itoa(i))
		randomReplicas := int32(rand.IntN(5) + 1)
		createdWebserver.Spec.Replicas = randomReplicas

		fmt.Printf("Random replica count: %d\n", randomReplicas)

		Expect(k8sClient.Update(ctx, createdWebserver)).Should(Succeed())
		_ = getURL(name+"-"+strconv.Itoa(i), "", []byte{})

		Eventually(func() bool {
			podList := &corev1.PodList{}

			labels := map[string]string{
				"application": appName + "-" + strconv.Itoa(i),
			}

			listOpts := []client.ListOption{
				client.InNamespace(namespace),
				client.MatchingLabels(labels),
			}
			Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())

			numberOfDeployedPods := int32(len(podList.Items))
			if numberOfDeployedPods != createdWebserver.Spec.Replicas {
				fmt.Printf("The number of deployed pods does not match the WebServer specification podList.\n")
				return false
			} else {
				return true
			}
		}, time.Second*300, time.Millisecond*500).Should(BeTrue())
	}

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

	BeforeAll(func() {
		createImageStream(imgStream)
	})

	AfterEach(func() {
		for i := 0; i < int(numberOfConcurrentWebServers); i++ {
			deleteWebServer(getWebServer(name + "-" + strconv.Itoa(i)))
		}
	})

	AfterAll(func() {
		deleteImageStream(imgStream)
	})

	Context("MultiWebserverTest", func() {
		It("AppImageTest", func() {
			for i := 0; i < int(numberOfConcurrentWebServers); i++ {
				webserver := &webserversv1alpha1.WebServer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "-" + strconv.Itoa(i),
						Namespace: namespace,
					},
					Spec: webserversv1alpha1.WebServerSpec{
						ApplicationName: appName + "-" + strconv.Itoa(i),
						Replicas:        int32(2),
						WebImage: &webserversv1alpha1.WebImageSpec{
							ApplicationImage: testImg,
						},
					},
				}
				createWebServer(webserver)
			}

			var wg sync.WaitGroup

			for i := 0; i < int(numberOfConcurrentWebServers); i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					// Execute the target function
					concurrentExecution(i)
				}()
			}

			wg.Wait()
		})

		It("AppImageWithVolumeTemplateTest", func() {
			storageRequest, err := resource.ParseQuantity("1Gi")
			Expect(err).Should(Succeed())

			for i := 0; i < int(numberOfConcurrentWebServers); i++ {
				webserver := &webserversv1alpha1.WebServer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "-" + strconv.Itoa(i),
						Namespace: namespace,
					},
					Spec: webserversv1alpha1.WebServerSpec{
						ApplicationName: appName + "-" + strconv.Itoa(i),
						Replicas:        int32(2),
						WebImage: &webserversv1alpha1.WebImageSpec{
							ApplicationImage: testImg,
						},
						Volume: &webserversv1alpha1.VolumeSpec{
							VolumeClaimTemplates: []corev1.PersistentVolumeClaimSpec{
								{
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: storageRequest,
										},
									},
								},
							},
						},
					},
				}
				createWebServer(webserver)
			}

			var wg sync.WaitGroup

			for i := 0; i < int(numberOfConcurrentWebServers); i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					// Execute the target function
					concurrentExecution(i)
				}()
			}

			wg.Wait()
		})

		It("ImageStreamTest", func() {

			for i := 0; i < int(numberOfConcurrentWebServers); i++ {
				webserver := &webserversv1alpha1.WebServer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "-" + strconv.Itoa(i),
						Namespace: namespace,
					},
					Spec: webserversv1alpha1.WebServerSpec{
						ApplicationName: appName + "-" + strconv.Itoa(i),
						Replicas:        int32(2),
						WebImage: &webserversv1alpha1.WebImageSpec{
							ApplicationImage: testImg,
						},
					},
				}
				createWebServer(webserver)
			}

			var wg sync.WaitGroup

			for i := 0; i < int(numberOfConcurrentWebServers); i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					// Execute the target function
					concurrentExecution(i)
				}()
			}

			wg.Wait()
		})
	})
})
