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

	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServer controller", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "image-basic-test"
	appName := "test-tomcat-demo"
	namespace := "jws-operator-tests"
	image := "quay.io/web-servers/tomcat10:latest"

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
				ApplicationImage: image,
			},
			EnvironmentVariables: []corev1.EnvVar{
				{
					Name:  "MY_ENV",
					Value: "my-random-string",
				},
			},
		},
	}

	BeforeAll(func() {
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
	})

	Context("EnvariableVariablesTest", func() {
		It("Update Test", func() {
			Eventually(func() bool {
				return checkPodsEnvVariables(k8sClient, ctx, name, namespace, "MY_ENV", "my-random-string")
			}, time.Minute*5, time.Second*10).Should(BeTrue())

			createdWebserver := getWebServer(name)
			envVars := createdWebserver.Spec.EnvironmentVariables

			for index, env := range envVars {
				if env.Name == "MY_ENV" {
					envVars[index] = corev1.EnvVar{Name: "MY_ENV", Value: "my-updated-string"}
				}
			}

			createdWebserver.Spec.EnvironmentVariables = envVars

			Eventually(func() bool {
				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				return checkPodsEnvVariables(k8sClient, ctx, name, namespace, "MY_ENV", "my-updated-string")
			}, time.Minute*5, time.Second*10).Should(BeTrue())
		})

		It("Add and Remove Test", func() {
			createdWebserver := getWebServer(name)
			envVars := createdWebserver.Spec.EnvironmentVariables
			envVars = append(envVars, corev1.EnvVar{Name: "MY_ENV_II", Value: "specific-string"})
			createdWebserver.Spec.EnvironmentVariables = envVars
			envVarsCount := len(envVars)

			Eventually(func() bool {
				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdWebserver.Spec.EnvironmentVariables).Should(HaveLen(envVarsCount))

			Eventually(func() bool {
				return checkPodsEnvVariables(k8sClient, ctx, name, namespace, "MY_ENV_II", "specific-string")
			}, time.Minute*5, time.Second*10).Should(BeTrue())

			createdWebserver = getWebServer(name)

			envVars = createdWebserver.Spec.EnvironmentVariables
			var removeIndex int

			for i, env := range envVars {
				if env.Name == "MY_ENV_II" {
					removeIndex = i
				}
			}

			envVars = append(envVars[:removeIndex], envVars[removeIndex+1:]...)

			createdWebserver.Spec.EnvironmentVariables = envVars
			envVarsCount = len(envVars)

			Eventually(func() bool {
				return k8sClient.Update(ctx, createdWebserver) == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				return k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, createdWebserver) == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdWebserver.Spec.EnvironmentVariables).Should(HaveLen(envVarsCount))
			Eventually(func() bool {
				return checkPodsEnvVariables(k8sClient, ctx, name, namespace, "MY_ENV_II", "specific-string")
			}, time.Minute*5, time.Second*10).Should(BeFalse())
		})
	})
})

func checkPodsEnvVariables(k8sClient client.Client, ctx context.Context, webServerName, webServerNamespace, EnvVarName, EnvVarValue string) bool {
	createdWebserver := getWebServer(webServerName)

	for _, statusPod := range createdWebserver.Status.Pods {
		podLookupKey := types.NamespacedName{Name: statusPod.Name, Namespace: webServerNamespace}
		pod := &corev1.Pod{}

		if k8sClient.Get(ctx, podLookupKey, pod) != nil {
			return false
		}

		container := pod.Spec.Containers[0]
		found := false

		for _, envVar := range container.Env {
			if envVar.Name == EnvVarName && envVar.Value == EnvVarValue {
				found = true
				break
			}
		}

		if !found {
			return false
		}

	}
	return true
}
