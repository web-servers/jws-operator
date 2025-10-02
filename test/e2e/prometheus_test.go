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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "tomcat-prometheus-test"
	appName := "prometheus-test"
	testURI := "/health"

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

	BeforeAll(func() {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "servicemonitors.monitoring.coreos.com", Namespace: "openshift-monitoring"}, crd)).Should(Succeed(), "Servicemonitor CRD not found")

		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster-monitoring-config", Namespace: "openshift-monitoring"}, cm)).Should(Succeed(), "Configmap cluster-monitoring-config not found")

		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
	})

	Context("PrometheusTest", func() {

		It("Basic Test", func() {
			err := utils.PrometheusTest(k8sClient, ctx, thetest, namespace, webserver, testURI, getDomain(name))
			Expect(err).Should(Succeed())
		})
	})
})

func getHost(name string) string {
	host := ""
	Eventually(func() bool {
		createdWebserver := getWebServer(name)

		if len(createdWebserver.Status.Hosts) > 0 {
			host = createdWebserver.Status.Hosts[0]
			return true
		}
		return false
	}, time.Second*120, time.Second).Should(BeTrue())

	return host
}

func getDomain(name string) string {
	host := getHost(name)
	_, after, _ := strings.Cut(host, namespace+".")

	fmt.Printf("DOMAIN: %s", after)
	return after
}
