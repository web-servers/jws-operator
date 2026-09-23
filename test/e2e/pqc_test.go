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
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "pqc-key-exchange-test"
	appName := "pqc-key-exchange"
	testURI := "/health"
	imageStreamName := "pqc-test"
	imageStreamNamespace := namespace

	helperRouteName := "pqc-domain-helper"
	host := ""

	imgStream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imageStreamName,
			Namespace: namespace,
		},
		Spec: imagev1.ImageStreamSpec{
			Tags: []imagev1.TagReference{
				{
					Name: "latest",
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: testImg,
					},
				},
			},
		},
	}

	helperRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helperRouteName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			Subdomain: "pqc",
			To: routev1.RouteTargetReference{
				Name: "pqc-dummy",
				Kind: "Service",
			},
		},
	}

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
			ApplicationName:      appName,
			Replicas:             1,
			UseSessionClustering: false,
			TLSConfig: webserversv1alpha1.TLSConfig{
				RouteHostname: "Will be set later",
				TLSSecret:     "test-tls-secret",
			},
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
			},
		},
	}

	BeforeAll(func() {
		createImageStream(imgStream)

		Expect(k8sClient.Create(ctx, helperRoute)).Should(Succeed())

		foundRoute := &routev1.Route{}
		Eventually(func() bool {
			if k8sClient.Get(ctx, types.NamespacedName{Name: helperRouteName, Namespace: namespace}, foundRoute) != nil {
				return false
			}
			host = utils.GetHost(foundRoute)
			return host != ""
		}, "1m", "1s").Should(BeTrue())

		webserver.Spec.TLSConfig.RouteHostname = "tls:pqctest-" + namespace + "." + host[4:]

		createWebServer(webserver)
	})

	AfterAll(func() {
		deleteWebServer(webserver)

		Expect(k8sClient.Delete(ctx, helperRoute)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: helperRouteName, Namespace: namespace}, &routev1.Route{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the helper route should be deleted")

		deleteImageStream(imgStream)
	})

	// The goal of the test is to check if JWS supports PQC key exchange, and it can be done without a change to the Operator.
	// To forces the client to request PQC key during each handshake is to set X25519MLKEM768 flag
	Context("PQCKeyExchangeTest", func() {
		It("TLS handshake over PQC key exchange with openssl", func() {
			routeHost := webserver.Spec.TLSConfig.RouteHostname[4:]

			var output string
			Eventually(func() bool {
				cmd := exec.Command("openssl", "s_client",
					"-connect", routeHost+":443",
					"-groups", "X25519MLKEM768",
					"-servername", routeHost,
				)
				cmd.Stdin = strings.NewReader("")

				out, err := cmd.CombinedOutput()
				output = string(out)
				if err != nil {
					thetest.Logf("openssl s_client error: %v", err)
					return false
				}
				return strings.Contains(output, "X25519MLKEM768")
			}, "3m", "10s").Should(BeTrue(),
				"Expected X25519MLKEM768 in TLS negotiation. openssl output: "+output)
		})

		It("curl HTTP GET to /health over PQC", func() {
			routeHost := webserver.Spec.TLSConfig.RouteHostname[4:]

			var output string
			Eventually(func() bool {
				cmd := exec.Command("curl", "-k", "-s", "-o", "/dev/null", "-w", "%{http_code}",
					"--curves", "X25519MLKEM768",
					"https://"+routeHost+testURI,
				)

				out, err := cmd.CombinedOutput()
				output = string(out)
				if err != nil {
					thetest.Logf("curl PQC error: %v", err)
					return false
				}
				return strings.TrimSpace(output) == "200"
			}, "3m", "10s").Should(BeTrue(),
				"Expected HTTP 200 over PQC connection, got: "+output)
		})
	})
})
