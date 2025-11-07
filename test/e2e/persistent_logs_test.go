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
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "persistent-logs-test"
	appName := "jws-img"

	webserver := &webserversv1alpha1.WebServer{
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
			PersistentLogsConfig: webserversv1alpha1.PersistentLogs{
				CatalinaLogs: false,
				AccessLogs:   false,

				VolumeName: "pv0002",
			},
		},
	}

	BeforeAll(func() {
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
	})

	Context("PersistentLogsTest", func() {

		It("CatalinaLogsAvailability", func() {
			createdWebserver := getWebServer(name)

			createdWebserver.Spec.PersistentLogsConfig.AccessLogs = false
			createdWebserver.Spec.PersistentLogsConfig.CatalinaLogs = true

			Expect(k8sClient.Update(ctx, createdWebserver)).Should(Succeed())

			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				stdout, stderr, err := getLogFiles(createdWebserver)

				if err != nil || stderr != "" {
					return false
				}

				for _, pod := range createdWebserver.Status.Pods {
					if !strings.Contains(stdout, "catalina-"+pod.Name) || strings.Contains(stdout, "access-"+pod.Name+".log") {
						return false
					}
				}

				return true
			}, time.Second*120, time.Millisecond*500).Should(BeTrue())

		})

		It("AccessLogsAvailability", func() {
			createdWebserver := getWebServer(name)

			createdWebserver.Spec.PersistentLogsConfig.AccessLogs = true
			createdWebserver.Spec.PersistentLogsConfig.CatalinaLogs = false

			Expect(k8sClient.Update(ctx, createdWebserver)).Should(Succeed())

			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				stdout, stderr, err := getLogFiles(createdWebserver)

				if err != nil || stderr != "" {
					thetest.Log("error: " + err.Error() + "\n")
					thetest.Log("stderr: " + stderr + "\n")
					return false
				}

				thetest.Log("stdout: " + stdout + "\n")

				for _, pod := range createdWebserver.Status.Pods {
					thetest.Log("pod: " + pod.Name + "\n")
					if !strings.Contains(stdout, "access-"+pod.Name+".log") || strings.Contains(stdout, "catalina-"+pod.Name) {
						return false
					}
				}
				return true
			}, time.Second*120, time.Millisecond*500).Should(BeTrue())
		})

		It("BothLogsAvailability", func() {
			createdWebserver := getWebServer(name)

			createdWebserver.Spec.PersistentLogsConfig.AccessLogs = true
			createdWebserver.Spec.PersistentLogsConfig.CatalinaLogs = true

			Expect(k8sClient.Update(ctx, createdWebserver)).Should(Succeed())

			Eventually(func() bool {
				createdWebserver = getWebServer(name)
				stdout, stderr, err := getLogFiles(createdWebserver)

				if err != nil || stderr != "" {
					return false
				}

				for _, pod := range createdWebserver.Status.Pods {
					if strings.Contains(stdout, "catalina-"+pod.Name) && strings.Contains(stdout, "access-"+pod.Name+".log") {
						return false
					}
				}

				return true
			}, time.Second*120, time.Millisecond*500).Should(BeTrue())
		})

	})
})

func getLogFiles(createdWebserver *webserversv1alpha1.WebServer) (string, string, error) {
	if len(createdWebserver.Status.Pods) == 0 {
		return "", "", errors.New("Unexpected number of pods")
	}

	statusPod := createdWebserver.Status.Pods[0]

	pod := &corev1.Pod{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: statusPod.Name, Namespace: namespace}, pod)

	if err != nil {
		return "", "", errors.New("Not able to find pod")
	}

	if len(pod.Spec.Containers) == 0 {
		return "", "", errors.New("Unexpected number of pods")
	}

	container := pod.Spec.Containers[0]

	command := []string{"ls", "/opt/tomcat_logs"}
	stdout, stderr, err := executeCommandOnPod(pod.Name, container.Name, command)

	if err != nil || stderr != "" {
		return stdout, stderr, err
	}

	return stdout, stderr, nil
}
