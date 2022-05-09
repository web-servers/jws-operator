package controllers

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	webserverstests "github.com/web-servers/jws-operator/test/framework"
)

var _ = Describe("WebServer controller", func() {
	Context("WebServerApplicationImageBasicTest", func() {
		It("WebServerApplicationImageBasicTest", func() {
			By("By creating a new WebServer")
			fmt.Printf("By creating a new WebServer\n")
			ctx := context.Background()
			randemo := "demo" + webserverstests.UnixEpoch()

			if noskip {
				Expect(webserverstests.WebServerApplicationImageSourcesScriptBasicTest(k8sClient, ctx, thetest, "default", "sourcesscriptbasictest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageBasicTest(k8sClient, ctx, thetest, "default", "rhregistrybasictest", "registry.redhat.io/jboss-webserver-5/webserver54-openjdk8-tomcat9-openshift-rhel8", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageBasicTest(k8sClient, ctx, thetest, "default", "basictest", "quay.io/jfclere/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageScaleTest(k8sClient, ctx, thetest, "default", "scaletest", "quay.io/jfclere/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageUpdateTest(k8sClient, ctx, thetest, "default", "updatetest", "quay.io/jfclere/tomcat10:latest", "quay.io/vmouriki/tomcat10:latest", "/health")).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageSourcesBasicTest(k8sClient, ctx, thetest, "default", "sourcesbasictest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				Expect(webserverstests.WebServerApplicationImageSourcesScaleTest(k8sClient, ctx, thetest, "default", "sourcesscaletest", "quay.io/jfclere/tomcat10:latest", "https://github.com/jfclere/demo-webapp", "jakartaEE", "quay.io/"+username+"/test", "secretfortests", "quay.io/jfclere/tomcat10-buildah", randemo)).Should(Succeed())
				isopenshift := webserverstests.WebServerHaveRoutes(k8sClient, ctx, thetest)
				if isopenshift {
					Expect(webserverstests.WebServerImageStreamBasicTest(k8sClient, ctx, thetest, "default", "imagestreambasictest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamScaleTest(k8sClient, ctx, thetest, "default", "imagestreamscaletest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "/health")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesBasicTest(k8sClient, ctx, thetest, "default", "imagestreamsourcesbasictest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
					Expect(webserverstests.WebServerImageStreamSourcesScaleTest(k8sClient, ctx, thetest, "default", "imagestreamsourcesscaletest", "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "https://github.com/jfclere/demo-webapp", "", "/demo-1.0/demo")).Should(Succeed())
				}
			}

		})
	})
})
