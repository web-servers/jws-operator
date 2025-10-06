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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "persistent-logs-test"
	appName := "jws-img"
	appImage := "registry.redhat.io/jboss-webserver-5/jws58-openjdk11-openshift-rhel8:latest"
	pullSecret := "secretfortests"
	testURI := "/health"

	webserver := &webserversv1alpha1.WebServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			ApplicationName: appName,
			Replicas:        int32(2),
			WebImage: &webserversv1alpha1.WebImageSpec{
				ApplicationImage: appImage,
				ImagePullSecret:  pullSecret,
				WebServerHealthCheck: &webserversv1alpha1.WebServerHealthCheckSpec{
					ServerReadinessScript: "if [ $(ls /opt/tomcat_logs |grep -c .log) != 4 ];then exit 1;fi",
				},
			},
			PersistentLogsConfig: webserversv1alpha1.PersistentLogs{
				CatalinaLogs: true,
				AccessLogs:   true,
				VolumeName:   "pv0002",
			},
			UseSessionClustering: true,
		},
	}

	BeforeAll(func() {
		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)
	})

	Context("PersistentLogsTest", func() {

		It("CheckLogsAvailability", func() {
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())

			createdWebserver := getWebServer(name)

			statusPod := createdWebserver.Status.Pods[0]
			Expect(statusPod).ShouldNot(BeNil())

			pod := getPod(statusPod.Name)
			container := pod.Spec.Containers[0]
			Expect(container).ShouldNot(BeNil())

			command := []string{"ls", "/opt/tomcat_logs"}
			stdout, stderr := executeCommandOnPod(pod.Name, container.Name, command)

			Expect(stderr).Should(BeEmpty())

			for _, pod := range createdWebserver.Status.Pods {
				Expect(strings.Contains(stdout, "access-"+pod.Name+".log")).Should(BeTrue())
				Expect(strings.Contains(stdout, "catalina-"+pod.Name)).Should(BeTrue())
			}
		})
	})
})
