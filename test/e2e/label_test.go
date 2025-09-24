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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	//	webserverstests "github.com/web-servers/jws-operator/test/utils"
	kbappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("LabelTest", func() {
		It("validating labels propagation", func() {
			By("creating webserver")

			ctx := context.Background()
			name := "label-test"
			appName := "test-tomcat-demo"

			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						"WebServer": name,
						"ready":     "oui",
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

			// make sure we cleanup at the end of this test.
			defer func() {
				k8sClient.Delete(context.Background(), webserver)
				time.Sleep(time.Second * 5)
			}()

			// create the webserver
			Expect(k8sClient.Create(ctx, webserver)).Should(Succeed())

			// Check it is started.
			webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
			createdWebserver := &webserversv1alpha1.WebServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			fmt.Printf("new WebServer Name: %s Namespace: %s\n", createdWebserver.ObjectMeta.Name, createdWebserver.ObjectMeta.Namespace)

			// Verify deployment template selector label.
			deployment := &kbappsv1.Deployment{}
			deploymentookupKey := types.NamespacedName{Name: appName, Namespace: namespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// check the labels
			stringmap := deployment.Spec.Template.GetLabels()
			fmt.Println(stringmap)
			Expect(deployment.Spec.Template.GetLabels()["app.kubernetes.io/name"]).Should(Equal(name))
			Expect(deployment.Spec.Template.GetLabels()["ready"]).Should(Equal("oui"))

			//TODO why this check whether createdWebserver exists?
			//get created webserver with updated recourceversion to continue
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, createdWebserver)

			if errors.IsNotFound(err) {
				thetest.Fatal(err)
			}

			//patch the webserver to change the labels
			text := "{\"metadata\":{\"labels\":{\"ready\":\"non\",\"ready1\": \"non1\",\"ready2\": \"non2\",\"ready3\": \"non3\",\"ready4\": \"non4\"}}}"
			bytes := []byte(text)

			_ = k8sClient.Patch(ctx, createdWebserver, client.RawPatch(types.MergePatchType, bytes))

			// Check it is started.
			webserverLookupKey = types.NamespacedName{Name: name, Namespace: namespace}
			createdWebserver = &webserversv1alpha1.WebServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			fmt.Printf("new WebServer Name: %s Namespace: %s\n", createdWebserver.ObjectMeta.Name, createdWebserver.ObjectMeta.Namespace)

			// Verify deployment template selector label.
			deployment = &kbappsv1.Deployment{}
			deploymentookupKey = types.NamespacedName{Name: appName, Namespace: namespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				podList := &corev1.PodList{}

				labels := map[string]string{
					"ready": "non",
				}

				listOpts := []client.ListOption{
					client.InNamespace(webserver.Namespace),
					client.MatchingLabels(labels),
				}
				k8sClient.List(ctx, podList, listOpts...)

				numberOfDeployedPods := int32(len(podList.Items))
				if numberOfDeployedPods != webserver.Spec.Replicas {
					//					log.Info("The number of deployed pods does not match the WebServer specification podList.")
					return false
				} else {
					return true
				}
			}, time.Second*300, time.Millisecond*500).Should(BeTrue())

			// remove the created webserver
			Expect(k8sClient.Delete(ctx, webserver)).Should(Succeed())

			// Check it is deleted.
			webserverLookupKey = types.NamespacedName{Name: name, Namespace: namespace}
			createdWebserver = &webserversv1alpha1.WebServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
				return errors.IsNotFound(err)
			}, time.Second*20, time.Millisecond*250).Should(BeTrue())

		})
	})
})
