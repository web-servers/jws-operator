package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	webserverstests "github.com/web-servers/jws-operator/test/framework"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WebServer controller", func() {
	Context("WebServerApplicationImageBasicTest", func() {
		It("WebServerApplicationImageBasicTest", func() {
			By("By creating a new WebServer")
			fmt.Printf("By creating a new WebServer\n")
			ctx := context.Background()
			randemo := "demo" + webserverstests.UnixEpoch()
			var namespace string
			if noskip {
				clientCfg, _ := clientcmd.NewDefaultClientConfigLoadingRules().Load()
				namespace = clientCfg.Contexts[clientCfg.CurrentContext].Namespace
				//This code works fine on user side, if it is run outside the cluster. https://stackoverflow.com/a/65661997
			} else {
				namespace = SetupTest(ctx).Name
			}

			if noskip {

				sec := &corev1.Secret{}

				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: namespace,
					Name:      "secretfortests",
				}, sec)
				if err != nil {
					thetest.Fatal(err)
				}

				err = k8sClient.Get(context.Background(), client.ObjectKey{
					Namespace: namespace,
					Name:      "test-tls-secret",
				}, sec)
				if err != nil {
					thetest.Fatal(err)
				}
				Expect(webserverstests.WebServerApplicationImageSourcesScriptBasicTest(k8sClient, ctx, thetest, namespace, "sourcesscriptbasictest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageBasicTest(k8sClient, ctx, thetest, namespace, "rhregistrybasictest", "registry.redhat.io/jboss-webserver-5/jws56-openjdk11-openshift-rhel8", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageBasicTest(k8sClient, ctx, thetest, namespace, "basictest", "quay.io/jfclere/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageScaleTest(k8sClient, ctx, thetest, namespace, "scaletest", "quay.io/jfclere/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageUpdateTest(k8sClient, ctx, thetest, namespace, "updatetest", "quay.io/jfclere/tomcat10:latest", "quay.io/vmouriki/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageSourcesBasicTest(k8sClient, ctx, thetest, namespace, "sourcesbasictest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageSourcesScaleTest(k8sClient, ctx, thetest, namespace, "sourcesscaletest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				isopenshift := webserverstests.WebServerHaveRoutes(k8sClient, ctx, thetest)
				if isopenshift {
					// Using a unstructured object.
					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   "image.openshift.io",
						Kind:    "ImageStream",
						Version: "v1",
					})
					err := k8sClient.Get(context.Background(), client.ObjectKey{
						Namespace: namespace,
						Name:      "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8",
					}, u)
					if errors.IsNotFound(err) {
						thetest.Fatal(err)
					}

					Expect(webserverstests.WebServerImageStreamBasicTest(k8sClient, ctx, thetest, namespace, "imagestreambasictest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamScaleTest(k8sClient, ctx, thetest, namespace, "imagestreamscaletest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesBasicTest(k8sClient, ctx, thetest, namespace, "imagestreamsourcesbasictest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesScaleTest(k8sClient, ctx, thetest, namespace, "imagestreamsourcesscaletest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
					Expect(webserverstests.WebServerSecureRouteTest(k8sClient, ctx, thetest, namespace, "secureroutetest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "/health")).Should(Succeed()) //tests if the created pod is accessible via the tls route created by the operator
				}
			}

		})
	})
})

func GetResourcesDynamically(dynamic dynamic.Interface, ctx context.Context,
	group string, version string, resource string, namespace string) (
	[]unstructured.Unstructured, error) {

	resourceId := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	list, err := dynamic.Resource(resourceId).Namespace(namespace).
		List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}
