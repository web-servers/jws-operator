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
	"k8s.io/apimachinery/pkg/util/intstr"

	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	kbappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "app-img-stream-source-test"
	appName := "image-stream-source-test"
	testURI := "/health"
	imageStreamName := "secure-route-test"
	imageStreamNamespace := namespace

	routeName := "def"
	serviceName := "def"
	host := ""

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
			ApplicationName:      appName,
			Replicas:             2,
			UseSessionClustering: false,
			TLSConfig: webserversv1alpha1.TLSConfig{
				RouteHostname: "Will be set later",
				TLSSecret:     "test-tls-secret",
			},
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
			},
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "def",
				Port:       int32(8080),
				TargetPort: intstr.FromInt(8080),
			}},
		},
	}

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			Subdomain: "sub",
			To: routev1.RouteTargetReference{
				Name: "def",
				Kind: "Service",
			},
		},
	}

	BeforeAll(func() {
		// create image stream
		createImageStream(imgStream)

		// create service
		Expect(k8sClient.Create(ctx, service)).Should(Succeed())

		foundService := &corev1.Service{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, foundService)
			return err == nil
		}, "1m", "1s").Should(BeTrue())

		// create route
		Expect(k8sClient.Create(ctx, route)).Should(Succeed())

		foundRoute := &routev1.Route{}
		Eventually(func() bool {
			if k8sClient.Get(ctx, types.NamespacedName{Name: routeName, Namespace: namespace}, foundRoute) != nil {
				return false
			}

			host = utils.GetHost(foundRoute)
			return host != ""
		}, "1m", "1s").Should(BeTrue())

		webserver.Spec.TLSConfig.RouteHostname = "tls:hosttest-" + namespace + "." + host[4:]

		// create WebServer
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)

		Expect(k8sClient.Delete(ctx, route)).Should(Succeed())
		routeLookupKey := types.NamespacedName{Name: routeName, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, routeLookupKey, &routev1.Route{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the route should be deleted")

		Expect(k8sClient.Delete(ctx, service)).Should(Succeed())
		serviceLookupKey := types.NamespacedName{Name: serviceName, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, serviceLookupKey, &corev1.Service{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the service should be deleted")

		deleteImageStream(imgStream)
	})

	Context("SecureRouteTest", func() {

		It("Basic Test", func() {
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, true)
			Expect(err).Should(Succeed())
		})

		It("Number of replicas", func() {
			foundDeployment := &kbappsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundDeployment)
				if err != nil {
					thetest.Logf("can't read Deployment")
					return false
				}
				if webserver.Spec.Replicas != foundDeployment.Status.AvailableReplicas {
					return false
				}

				return true
			}, "5m", "10s").Should(BeTrue())
		})

		It("SecretUpdateTest", func() {
			createdWebserver := getWebServer(name)
			original_secret := createdWebserver.Spec.TLSConfig.TLSSecret
			new_secret := "my-nonexisting-secret"

			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				createdWebserver.Spec.TLSConfig.TLSSecret = new_secret

				err := k8sClient.Update(ctx, createdWebserver)

				if err != nil {
					return false
				}

				foundDeployment := &kbappsv1.Deployment{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundDeployment)
				if err != nil {
					return false
				}

				for _, volume := range foundDeployment.Spec.Template.Spec.Volumes {
					if volume.Secret != nil && volume.Secret.SecretName == new_secret {
						return true
					}
				}
				return false

			}, time.Second*60, time.Millisecond*250).Should(BeTrue())

			createdWebserver = getWebServer(name)
			createdWebserver.Spec.TLSConfig.TLSSecret = original_secret
			Expect(k8sClient.Update(ctx, createdWebserver)).Should(Succeed())

			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, true)
			Expect(err).Should(Succeed())
		})

		It("SessionClusteringTest", func() {
			Eventually(func() error {
				createdWebserver := getWebServer(name)
				createdWebserver.Spec.UseSessionClustering = true

				err := k8sClient.Update(ctx, createdWebserver)

				if err != nil {
					thetest.Logf("Error: %s", err)
					return err
				}
				thetest.Logf("WebServer %s was updated\n", createdWebserver.Name)

				_, err = utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, true)
				return err

			}, time.Second*60, time.Millisecond*250).Should(Succeed())

		})
	})
})
