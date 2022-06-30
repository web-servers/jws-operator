package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	webserverstests "github.com/web-servers/jws-operator/test/framework"
	"k8s.io/client-go/tools/clientcmd"
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
				//This code works fine on user side, it it is run outside the cluster. https://stackoverflow.com/a/65661997
			} else {
				namespace = SetupTest(ctx).Name
			}

			if noskip {
				Expect(webserverstests.WebServerApplicationImageSourcesScriptBasicTest(k8sClient, ctx, thetest, namespace, "sourcesscriptbasictest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageBasicTest(k8sClient, ctx, thetest, namespace, "rhregistrybasictest", "registry.redhat.io/jboss-webserver-5/webserver54-openjdk8-tomcat9-openshift-rhel8", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageBasicTest(k8sClient, ctx, thetest, namespace, "basictest", "quay.io/jfclere/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageScaleTest(k8sClient, ctx, thetest, namespace, "scaletest", "quay.io/jfclere/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageUpdateTest(k8sClient, ctx, thetest, namespace, "updatetest", "quay.io/jfclere/tomcat10:latest", "quay.io/pitprok/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageSourcesBasicTest(k8sClient, ctx, thetest, namespace, "sourcesbasictest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageSourcesScaleTest(k8sClient, ctx, thetest, namespace, "sourcesscaletest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				isopenshift := webserverstests.WebServerHaveRoutes(k8sClient, ctx, thetest)
				if isopenshift {
					Expect(webserverstests.WebServerImageStreamBasicTest(k8sClient, ctx, thetest, namespace, "imagestreambasictest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamScaleTest(k8sClient, ctx, thetest, namespace, "imagestreamscaletest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesBasicTest(k8sClient, ctx, thetest, namespace, "imagestreamsourcesbasictest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesScaleTest(k8sClient, ctx, thetest, namespace, "imagestreamsourcesscaletest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
				}
			}

		})
	})
})
