package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"

	imagestreamv1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"

	. "github.com/onsi/gomega"
	webserverstests "github.com/web-servers/jws-operator/test/framework"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

					is := &imagestreamv1.ImageStream{}

					err := k8sClient.Get(context.Background(), client.ObjectKey{
						Namespace: namespace,
						Name:      "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8",
					}, is)
					if errors.IsNotFound(err) {
						thetest.Fatal(err)
					}

					Expect(webserverstests.WebServerImageStreamBasicTest(k8sClient, ctx, thetest, namespace, "imagestreambasictest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamScaleTest(k8sClient, ctx, thetest, namespace, "imagestreamscaletest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesBasicTest(k8sClient, ctx, thetest, namespace, "imagestreamsourcesbasictest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesScaleTest(k8sClient, ctx, thetest, namespace, "imagestreamsourcesscaletest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
					//procedure to find defaultIngressDomain
					service := &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "def",
							Namespace: namespace,
						},
						Spec: corev1.ServiceSpec{
							Ports: []corev1.ServicePort{{
								Name:       "def",
								Port:       int32(8080),
								TargetPort: intstr.FromInt(8080),
							}},
						},
					}

					Expect(k8sClient.Create(ctx, service)).Should(Succeed())

					route := &routev1.Route{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "def",
							Namespace: namespace,
						},
						Spec: routev1.RouteSpec{
							Subdomain: "sub",
							To: routev1.RouteTargetReference{
								Name: "def",
							},
						},
					}

					Expect(k8sClient.Create(ctx, route)).Should(Succeed())

					Eventually(func() bool {
						err := k8sClient.Get(ctx, client.ObjectKey{
							Namespace: namespace,
							Name:      "def",
						}, route)
						if err != nil {
							return false
						}
						return true
					}, time.Second*10, time.Millisecond*250).Should(BeTrue())

					Expect(k8sClient.Delete(ctx, route)).Should(Succeed())
					Expect(k8sClient.Delete(ctx, service)).Should(Succeed())
					//procedure to find defaultIngressDomain

					Expect(webserverstests.WebServerSecureRouteTest(k8sClient, ctx, thetest, namespace, "secureroutetest", "jboss-webserver56-openjdk8-tomcat9-openshift-ubi8", "/health", route.Spec.Host[5+len(namespace):])).Should(Succeed()) //tests if the created pod is accessible via the tls route created by the operator
					Expect(webserverstests.HPATest(k8sClient, ctx, thetest, namespace, "hpatest", "")).Should(Succeed())
					Expect(webserverstests.PersistentLogsTest(k8sClient, ctx, thetest, namespace, "persistentlogstest", "")).Should(Succeed())
					//check if servicemonitor crd exists b/c only then the feature works
					crd := &apiextensionsv1.CustomResourceDefinition{}
					err = k8sClient.Get(context.TODO(), client.ObjectKey{Name: "servicemonitors.monitoring.coreos.com"}, crd)
					if err != nil {
						fmt.Printf("servicemonitor CRD not found skipping prometheus Test")
					} else {
						Expect(webserverstests.PrometheusTest(k8sClient, ctx, thetest, namespace, "prometheustest")).Should(Succeed())
					}
				}
			}

		})
	})
})
