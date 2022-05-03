package controllers

import (
	"context"
	// "errors"
	"fmt"
	// "testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// corev1 "k8s.io/api/core/v1"
	// appsv1 "github.com/openshift/api/apps/v1"
	kbappsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	// "k8s.io/kubectl/pkg/util/podutils"
	// podv1 "k8s.io/kubernetes/pkg/api/v1/pod"
	// apierrors "k8s.io/apimachinery/pkg/api/errors"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/tools/clientcmd"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	// webserverstests "github.com/web-servers/jws-operator/test/framework"
)

var _ = Describe("WebServer controller", func() {
	Context("First Test", func() {
		It("Label test", func() {
			By("By creating a new WebServer")
			fmt.Printf("By creating a new WebServer\n")
			name := "label-test"

			ctx := context.Background()
			var namespace string
			if noskip {
				clientCfg, _ := clientcmd.NewDefaultClientConfigLoadingRules().Load()
				namespace = clientCfg.Contexts[clientCfg.CurrentContext].Namespace
				//This code works fine on user side, it it is run outside the cluster. https://stackoverflow.com/a/65661997
			} else {
				namespace = SetupTest(ctx).Name
			}
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: name,
					Replicas:        int32(2),
					WebImage: &webserversv1alpha1.WebImageSpec{
						ApplicationImage: "quay.io/jfclere/tomcat-demo",
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
			// deployment := &appsv1.DeploymentConfig{}
			deploymentookupKey := types.NamespacedName{Name: name, Namespace: namespace}
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

			// remove the created webserver
			Expect(k8sClient.Delete(ctx, webserver)).Should(Succeed())

		})
	})
})
