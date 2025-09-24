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

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
				//				StorageClass: "nfs-client",
			},
			UseSessionClustering: true,
		},
	}

	BeforeAll(func() {
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
	})

	Context("Persistent Logs Test", func() {

		It("Basic Test", func() {
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})
	})
})
