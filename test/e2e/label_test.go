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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	kbappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	name := "label-test"
	appName := "test-tomcat-demo"

	webserver := &webserversv1alpha1.WebServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"WebServer":     name,
				"ready":         "oui",
				"willBeRemoved": "InRemoveLabelTest",
			},
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

	Context("LabelPropagationTest", func() {
		It("CheckLabelsTest", func() {
			Expect(checkLabel(appName, "app.kubernetes.io/name", name)).Should(BeTrue())
			Expect(checkLabel(appName, "ready", "oui")).Should(BeTrue())
		})

		It("AddLabelTest", func() {
			key := "my-new-label"
			value := "label-string-1"

			// Update labals and update the WebServer
			Eventually(func() bool {
				createdWebserver := getWebServer(name)

				labels := createdWebserver.Labels
				labels[key] = value

				createdWebserver.Labels = labels

				err := k8sClient.Update(ctx, createdWebserver)
				if err != nil {
					thetest.Logf("WebServer update failed:  %s\n", err)
					return false
				}
				thetest.Logf("WebServer %s updated\n", createdWebserver.Name)
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				return checkLabel(appName, key, value)
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})

		It("RemoveLabelTest", func() {
			key := "willBeRemoved"
			value := "InRemoveLabelTest"

			Expect(checkLabel(appName, key, value)).Should(BeTrue())

			// Update labals and update the WebServer
			Eventually(func() bool {
				createdWebserver := getWebServer(name)

				labels := createdWebserver.Labels
				delete(labels, key)

				createdWebserver.Labels = labels

				err := k8sClient.Update(ctx, createdWebserver)
				if err != nil {
					thetest.Logf("WebServer update failed:  %s\n", err)
					return false
				}
				thetest.Logf("WebServer %s updated\n", createdWebserver.Name)
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				return checkLabel(appName, key, value)
			}, time.Second*30, time.Millisecond*250).Should(BeFalse())
		})

		It("SelectorTest", func() {
			Eventually(func() bool {
				podList := &corev1.PodList{}

				labels := map[string]string{
					"ready": "oui",
				}

				listOpts := []client.ListOption{
					client.InNamespace(webserver.Namespace),
					client.MatchingLabels(labels),
				}
				Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())

				numberOfDeployedPods := int32(len(podList.Items))
				if numberOfDeployedPods != webserver.Spec.Replicas {
					fmt.Printf("The number of deployed pods does not match the WebServer specification podList.")
					return false
				} else {
					return true
				}
			}, time.Second*300, time.Millisecond*500).Should(BeTrue())
		})

		It("MeteringLabelsPresenceTest", func() {
			labels := map[string]string{
				"com.company":   "Red_Hat",
				"rht.prod_name": "Red_Hat_Runtimes",
				"rht.prod_ver":  "2022-Q1",
				"rht.comp":      "JBoss_Web_Server",
				"rht.comp_ver":  "5.8.4",
				"rht.subcomp":   "Tomcat_9",
				"rht.subcomp_t": "application",
			}

			Eventually(func() bool {
				createdWebserver := getWebServer(name)
				createdWebserver.Labels = labels

				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				podList := &corev1.PodList{}

				listOpts := []client.ListOption{
					client.InNamespace(webserver.Namespace),
					client.MatchingLabels(labels),
				}
				if k8sClient.List(ctx, podList, listOpts...) != nil {
					return false
				}

				return webserver.Spec.Replicas != int32(len(podList.Items))
			}, time.Second*300, time.Millisecond*500).Should(BeTrue(), "The number of deployed pods with metering labels does not match the WebServer specification podList.")
		})
	})
})

func checkLabel(appName string, key string, value string) bool {
	// Verify deployment template selector label.
	deployment := &kbappsv1.Deployment{}
	deploymentookupKey := types.NamespacedName{Name: appName, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, deploymentookupKey, deployment)
		return err == nil
	}, time.Second*10, time.Millisecond*250).Should(BeTrue())

	// check the label
	return deployment.Spec.Template.GetLabels()[key] == value
}
