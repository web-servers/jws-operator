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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServer controller", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "tomcat-prometheus-test"
	appName := "prometheus-test"
	namespace := "jws-operator-tests"
	testURI := "/health"

	routeName := "def"
	serviceName := "def"
	host := ""

	webserver := &webserversv1alpha1.WebServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			ApplicationName: appName,
			Replicas:        int32(1),
			WebImage: &webserversv1alpha1.WebImageSpec{
				ApplicationImage: "quay.io/web-servers/tomcat-prometheus",
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
		// create the service
		Expect(k8sClient.Create(ctx, service)).Should(Succeed())

		foundService := &corev1.Service{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, foundService)
			if err != nil {
				return false
			}
			return true
		}, "1m", "1s").Should(BeTrue())

		// create the route
		Expect(k8sClient.Create(ctx, route)).Should(Succeed())

		foundRoute := &routev1.Route{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: routeName, Namespace: namespace}, foundRoute)
			if err != nil {
				return false
			}
			return true
		}, "1m", "1s").Should(BeTrue())

		Eventually(func() string {
			host = getHost(foundRoute)
			return host
		}, "1m", "1s").ShouldNot(BeEmpty())

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
		k8sClient.Delete(ctx, webserver)
		webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, webserverLookupKey, &webserversv1alpha1.WebServer{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the webserver should be deleted")

		k8sClient.Delete(ctx, route)
		routeLookupKey := types.NamespacedName{Name: routeName, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, routeLookupKey, &routev1.Route{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the route should be deleted")

		k8sClient.Delete(ctx, service)
		serviceLookupKey := types.NamespacedName{Name: serviceName, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, serviceLookupKey, &corev1.Service{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the service should be deleted")
	})

	Context("PrometheusTest", func() {

		It("Basic Test", func() {
			err := utils.PrometheusTest(k8sClient, ctx, thetest, namespace, webserver, testURI, host[4:])
			Expect(err).Should(Succeed())
		})
	})
})

func getHost(route *routev1.Route) string {
	if len(route.Status.Ingress) > 0 {
		host := route.Status.Ingress[0].Host
		return host
	}
	return ""
}
