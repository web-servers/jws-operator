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

	routev1 "github.com/openshift/api/route/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	kbappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServer controller", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "app-img-stream-source-test"
	appName := "image-stream-source-test"
	namespace := "jws-operator-tests"
	testURI := "/health"
	imageStreamName := "jboss-webserver57-openjdk11-tomcat9-openshift-ubi8"
	imageStreamNamespace := namespace

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
			Name:      "def",
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
			Name:      "def",
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
		// create the service
		Expect(k8sClient.Create(ctx, service)).Should(Succeed())

		foundService := &corev1.Service{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundService)
			if err != nil {
				return false
			}
			return true
		}, "1m", "1s").Should(BeTrue())

		// create the route
		Expect(k8sClient.Create(ctx, route)).Should(Succeed())

		foundRoute := &routev1.Route{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundRoute)
			if err != nil {
				return false
			}
			return true
		}, "1m", "1s").Should(BeTrue())

		host := route.Status.Ingress[0].Host

		Expect(host).ShouldNot(BeEmpty())

		webserver.Spec.TLSConfig.RouteHostname = "tls:hosttest-" + namespace + "." + host[4:]

		// create the webserver
		Expect(k8sClient.Create(ctx, webserver)).Should(Succeed())

		// is the webserver running
		webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdWebserver := &webserversv1alpha1.WebServer{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
			if err != nil {
				return false
			}
			return true
		}, time.Second*10, time.Millisecond*250).Should(BeTrue())
	})

	AfterAll(func() {
		k8sClient.Delete(context.Background(), webserver)
		webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, webserverLookupKey, &webserversv1alpha1.WebServer{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the webserver should be deleted")
	})

	Context("ImageStreamSourceTest", func() {

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
				Expect(int32(webserver.Spec.Replicas) == int32(foundDeployment.Status.AvailableReplicas)).Should(BeTrue())
				return true
			}, "5m", "10s").Should(BeTrue())
		})

		/*
		   		It("SessionClustering Test", func() {
		   			createdWebserver := &webserversv1alpha1.WebServer{}
		   	                Eventually(func() bool {
		           	                err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, createdWebserver)
		                   	        if err != nil {
		                           	        return false
		   	                        }
		           	                return true
		                   	}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		   			createdWebserver.Spec.UseSessionClustering = true

		   			Eventually(func() bool {
		                                   err := k8sClient.Update(ctx, createdWebserver)
		                                   if err != nil {
		                                           return false
		                                   }
		                                   return true
		                           }, time.Second*10, time.Millisecond*250).Should(BeTrue())

		   			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, true)
		                           Expect(err).Should(Succeed())
		   		})
		*/
	})
})
