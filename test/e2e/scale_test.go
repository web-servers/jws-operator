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
	"sigs.k8s.io/controller-runtime/pkg/client"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "scale-test"
	appName := "test-tomcat-demo"
	testURI := "/health"

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
				ApplicationImage: testImg,
			},
		},
	}

	BeforeAll(func() {
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
	})

	//TODO The following tests are testing scaling when Deployment is created.
	// We need to add aslo tests when StatefullSet is created.
	Context("ScaleTest", func() {
		It("ScaleUpDownTest", func() {

			// scale up to 4
			scaleTo(name, 4)
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())

			// scale down to 1
			scaleTo(name, 1)
			_, err = utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())

			// scale down to 0
			scaleTo(name, 0)

			// scale up to 2
			scaleTo(name, 2)
			_, err = utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})
	})
})

func scaleTo(name string, replicas int32) {
	var createdWebSeerver *webserversv1alpha1.WebServer

	Eventually(func() bool {
		createdWebSeerver = getWebServer(name)
		createdWebSeerver.Spec.Replicas = replicas

		return k8sClient.Update(ctx, createdWebSeerver) == nil
	}, time.Second*30, time.Millisecond*100).Should(BeTrue())

	Eventually(func() bool {
		podList := &corev1.PodList{}
		labels := map[string]string{
			"WebServer": name,
		}

		listOpts := []client.ListOption{
			client.InNamespace(createdWebSeerver.Namespace),
			client.MatchingLabels(labels),
		}
		if k8sClient.List(ctx, podList, listOpts...) != nil {
			return false
		}

		numberOfDeployedPods := int32(len(podList.Items))

		return numberOfDeployedPods == replicas
	}, time.Second*120, time.Millisecond*500).Should(BeTrue())
}
