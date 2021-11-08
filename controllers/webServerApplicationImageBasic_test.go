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

			Expect(webserverstests.WebServerApplicationImageBasicTest(k8sClient, ctx, thetest, "default", "jfctest", "quay.io/jfclere/tomcat10:latest", "/health")).Should(Succeed())

		})
	})
})
