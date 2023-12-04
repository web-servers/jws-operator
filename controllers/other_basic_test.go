package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	webserverstests "github.com/web-servers/jws-operator/test/framework"
)

var _ = Describe("WebServer controller", func() {

	Context("First Test", func() {
		It("Other Basic test", func() {
			By("By creating a new WebServer")
			fmt.Printf("By creating a new WebServer\n")
			if !noskip {
				fmt.Printf("other_basic_test skipped\n")
				return
			}
			ctx := context.Background()
			name := "other-basic-test"
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
					ApplicationName: "test-tomcat-demo",
					Replicas:        int32(2),
					WebImage: &webserversv1alpha1.WebImageSpec{
						// ApplicationImage: "quay.io/jfclere/tomcat-demo",
						ApplicationImage: "registry.redhat.io/jboss-webserver-5/webserver54-openjdk8-tomcat9-openshift-rhel8",
						ImagePullSecret:  "secretfortests",
					},
				},
			}

			// make sure we cleanup
			defer func() {
				k8sClient.Delete(context.Background(), webserver)
				time.Sleep(time.Second * 5)
			}()

			// create the webserver
			fmt.Printf("create WebServer Name: %s Namespace: %s\n", webserver.ObjectMeta.Name, webserver.ObjectMeta.Namespace)
			Expect(k8sClient.Create(ctx, webserver)).Should(Succeed())

			// Check it is started.
			webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
			createdWebserver := &webserversv1alpha1.WebServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
				if err != nil {
					log.Info("k8sClient.Get failed")
					return false
				}
				return true
			}, time.Second*20, time.Millisecond*250).Should(BeTrue())
			fmt.Printf("new WebServer Name: %s Namespace: %s\n", createdWebserver.ObjectMeta.Name, createdWebserver.ObjectMeta.Namespace)

			// are the corresponding pods ready?
			Eventually(func() bool {
				err := webserverstests.WaitUntilReady(k8sClient, ctx, thetest, createdWebserver)
				if err != nil {
					log.Info("Not ready")
					return false
				}
				return true
			}, timeout, retryInterval).Should(BeTrue())

		})
	})
})
