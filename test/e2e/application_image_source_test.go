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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	//	kbappsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "app-img-source-test"
	appName := "source-test"
	pkgName := "demo" + utils.UnixEpoch()
	testURI := "/" + pkgName + "/demo"
	image := "quay.io/web-servers/tomcat10:latest"
	sourceRepositoryURL := "https://github.com/web-servers/demo-webapp"
	sourceRepositoryRef := "main"
	pushsecret := "secretfortests"
	pushedimage := "quay.io/" + os.Getenv("USER") + "/test"
	imagebuilder := "quay.io/web-servers/tomcat10-buildah"
	//	newImage := "quay.io/web-servers/tomcat10update:latest"

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
				WebApp: &webserversv1alpha1.WebAppSpec{
					Name:                     pkgName + ".war",
					SourceRepositoryURL:      sourceRepositoryURL,
					SourceRepositoryRef:      sourceRepositoryRef,
					WebAppWarImagePushSecret: pushsecret,
					WebAppWarImage:           pushedimage,
					Builder: &webserversv1alpha1.BuilderSpec{
						Image: imagebuilder,
					},
				},
			},
		},
	}

	BeforeAll(func() {
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
	})

	AfterAll(func() {
		k8sClient.Delete(context.Background(), webserver)
		webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, webserverLookupKey, &webserversv1alpha1.WebServer{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the webserver should be deleted")
	})

	Context("ApplicationImageSourceTest", func() {
		It("Basic Test", func() {
			_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
			Expect(err).Should(Succeed())
		})
		/*
			It("Update Test", func() {
				createdWebserver := &webserversv1alpha1.WebServer{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, createdWebserver)
					if err != nil {
						return false
					}
					return true
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())

				createdWebserver.Spec.WebImage.ApplicationImage = newImage

				Eventually(func() bool {
					err := k8sClient.Update(ctx, createdWebserver)
					if err != nil {
						return false
					}
					thetest.Logf("WebServer %s updated\n", name)
					return true
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())

				foundDeployment := &kbappsv1.Deployment{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundDeployment)
					if err != nil {
						return false
					}
					return true
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())

				foundImage := foundDeployment.Spec.Template.Spec.Containers[0].Image
				Expect(foundImage == newImage).Should(BeTrue(), "Image Update Test: image check failed")

				// Wait until the replicas are available
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: namespace}, foundDeployment)
					if err != nil {
						thetest.Fatalf("can't read Deployment")
						return false
					}

					if int32(createdWebserver.Spec.Replicas) == int32(foundDeployment.Status.AvailableReplicas) {
						return true
					} else {
						return false
					}
				}, time.Second*420, time.Second*30).Should(BeTrue(), "Image Update Test: Required amount of replicas were not achieved")

				_, err := utils.WebServerRouteTest(k8sClient, ctx, thetest, webserver, testURI, false, nil, false)
				Expect(err).Should(Succeed())
			})
		*/
	})
})
