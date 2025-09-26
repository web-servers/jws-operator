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

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"github.com/web-servers/jws-operator/test/utils"
	v2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()
	name := "hpa-test"
	appName := "tomcat-demo-test"
	testURI := "/health"
	autoscalerName := "hpatest-hpa"

	percentage := int32(4)

	webserver := &webserversv1alpha1.WebServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			ApplicationName: appName,
			Replicas:        int32(4),
			WebImage: &webserversv1alpha1.WebImageSpec{
				ApplicationImage: testImg,
			},
			PodResources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		},
	}

	hpa := &v2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      autoscalerName,
			Namespace: namespace,
		},
		Spec: v2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: v2.CrossVersionObjectReference{
				APIVersion: "web.servers.org/v1alpha1",
				Kind:       "WebServer",
				Name:       name,
			},
			MinReplicas: nil,
			MaxReplicas: 5,
		},
	}

	metric := &v2.MetricSpec{
		Type: v2.ResourceMetricSourceType,
		Resource: &v2.ResourceMetricSource{
			Name: corev1.ResourceCPU,
			Target: v2.MetricTarget{
				Type:               v2.UtilizationMetricType,
				AverageUtilization: &percentage,
			},
		},
	}

	BeforeAll(func() {
		createWebServer(webserver)

		metrics := make([]v2.MetricSpec, 0, 4)
		metrics = append(metrics, *metric)
		hpa.Spec.Metrics = metrics

		Eventually(func() bool {
			err := k8sClient.Create(ctx, hpa)
			if err != nil {
				thetest.Logf("Error: %s", err)
				return false
			}
			thetest.Logf("HPA was created\n")
			return true
		}, time.Second*30, time.Millisecond*250).Should(BeTrue())
	})

	AfterAll(func() {
		deleteWebServer(webserver)

		k8sClient.Delete(ctx, hpa)
		hpaLookupKey := types.NamespacedName{Name: autoscalerName, Namespace: namespace}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, hpaLookupKey, &v2.HorizontalPodAutoscaler{})
			return apierrors.IsNotFound(err)
		}, "2m", "5s").Should(BeTrue(), "the route should be deleted")
	})

	Context("HPATest", func() {

		It("Basic Test", func() {
			err := utils.AutoScalingTest(k8sClient, ctx, thetest, webserver, testURI, hpa)
			Expect(err).Should(Succeed())
		})
	})
})
