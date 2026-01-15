package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WebServerControllerTest", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	const (
		PayloadAcceptedLog       = "Red Hat Insights - Payload was accepted for processing"
		AgentLoggerLog           = "com.redhat.insights.agent.AgentLogger"
		StartingInsightsAgentLog = "Starting Red Hat Insights agent"
		ServerStartupLog         = "org.apache.catalina.startup.Catalina.start Server startup in"
	)

	name := "insights-test"
	appName := "jws-img"

	var webserver *webserversv1alpha1.WebServer

	BeforeEach(func() {
		webserver = &webserversv1alpha1.WebServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: webserversv1alpha1.WebServerSpec{
				ApplicationName: appName,
				Replicas:        1,
				WebImage: &webserversv1alpha1.WebImageSpec{
					ApplicationImage: testImg,
				},
			},
		}
	})

	AfterEach(func() {
		if webserver != nil {
			deleteWebServer(webserver)
		}
		// give additional time to the pod to terminate
		time.Sleep(10 * time.Second)
	})

	// Waits for at least one pod to appear and returns its name.
	waitForPod := func() string {
		var podName string
		Eventually(func() bool {
			currentWebServer := getWebServer(name)
			if currentWebServer == nil || len(currentWebServer.Status.Pods) == 0 {
				return false
			}
			podName = currentWebServer.Status.Pods[0].Name
			return true
		}, time.Minute*2, time.Second).Should(BeTrue(), "Pod should appear in status")
		return podName
	}

	// Checks if the logs of the specified pod contain a substring.
	checkLogsContainString := func(podName, substring string) {
		Eventually(func() bool {
			logs := getPodLogs(namespace, podName)
			if len(logs) == 0 {
				return false
			}
			return strings.Contains(logs, substring)
		}, time.Minute*2, time.Second*5).Should(BeTrue(), "Log of pod %s must contain '%s'", podName, substring)
	}

	// Checks if the logs of the specified pod do NOT contain a substring.
	checkLogsNotContainString := func(podName, substring string) {
		var logs string
		Eventually(func() bool {
			logs = getPodLogs(namespace, podName)
			return strings.Contains(logs, ServerStartupLog)
		}, time.Minute*2, time.Second).Should(BeTrue(), "Tomcat server should finish startup (wait for '%s')", ServerStartupLog)

		Expect(logs).To(Not(ContainSubstring(substring)),
			"Log of pod %s must NOT contain '%s' after full startup", podName, substring)
	}

	Context("InsightsTests", func() {

		// Enabling the Insights client and debugging mode
		// Verifies that the log defined in [PayloadAcceptedLog] appears in the pod's log
		It("SimpleInsightsReportSendTest", func() {
			webserver.Spec.UseInsightsClient = true
			webserver.Spec.EnvironmentVariables = []corev1.EnvVar{
				{
					Name:  "INSIGHTS_DEBUG",
					Value: "true",
				},
			}

			createWebServer(webserver)

			podName := waitForPod()
			checkLogsContainString(podName, PayloadAcceptedLog)
		})

		// Enabling Insights, but disabling the INSIGHTS_DEBUG variable (false).
		// Verifies that technical messages [AgentLogger] DOESN'T appear in the pod's log
		It("DisabledDebugLogsTest", func() {
			webserver.Spec.UseInsightsClient = true
			webserver.Spec.EnvironmentVariables = []corev1.EnvVar{
				{
					Name:  "INSIGHTS_DEBUG",
					Value: "false",
				},
			}

			createWebServer(webserver)

			podName := waitForPod()
			checkLogsNotContainString(podName, AgentLoggerLog)
		})

		// Setting UseInsightsClient to false.
		// The Insights Agent should not be running at all. The logs should not contain [StartingInsightsAgentLog]
		It("NoAgentStartedWhenDisabledTest", func() {
			webserver.Spec.UseInsightsClient = false
			webserver.Spec.EnvironmentVariables = []corev1.EnvVar{
				{
					Name:  "INSIGHTS_DEBUG",
					Value: "true",
				},
			}

			createWebServer(webserver)

			podName := waitForPod()
			checkLogsNotContainString(podName, AgentLoggerLog)
		})

		// Launches the applicationThen then forcibly delete the pod
		// Kubernetes creates a new pod. The agent must start successfully and send the report in a NEW pod.
		It("InsightsAgentPersistsAfterPodRestart", func() {
			webserver.Spec.UseInsightsClient = true
			webserver.Spec.EnvironmentVariables = []corev1.EnvVar{
				{
					Name:  "INSIGHTS_DEBUG",
					Value: "true",
				},
			}

			createWebServer(webserver)

			// Create first pod
			initialPodName := waitForPod()
			checkLogsContainString(initialPodName, PayloadAcceptedLog)

			deletePod(namespace, initialPodName)

			// Create a new pod
			var newPodName string
			Eventually(func() bool {
				currentWebServer := getWebServer(name)
				if currentWebServer == nil || len(currentWebServer.Status.Pods) == 0 {
					return false
				}

				// Check directly against the struct field
				if currentWebServer.Status.Pods[0].Name != initialPodName {
					newPodName = currentWebServer.Status.Pods[0].Name
					return true
				}
				return false
			}, time.Minute*2, time.Second).Should(BeTrue(), "New pod should appear after deletion")

			checkLogsContainString(newPodName, PayloadAcceptedLog)
		})

		// Checks agent stability during scaling (Scaling & Restart)
		// The agent must start successfully and send a report in EACH time
		It("InsightsAgentPersistsAfterScalingTest", func() {
			webserver.Spec.UseInsightsClient = true
			webserver.Spec.EnvironmentVariables = []corev1.EnvVar{
				{
					Name:  "INSIGHTS_DEBUG",
					Value: "true",
				},
			}
			createWebServer(webserver)

			initialPodName := waitForPod()
			checkLogsContainString(initialPodName, PayloadAcceptedLog)

			scaleTo(name, 0)
			scaleTo(name, 1)

			newPodName := waitForPod()

			Expect(newPodName).ToNot(Equal(initialPodName), "Pod name should change after scaling down and up")
			checkLogsContainString(newPodName, PayloadAcceptedLog)

			scaleTo(name, 3)

			Eventually(func() int {
				currentWebServer := getWebServer(name)
				if currentWebServer == nil {
					return 0
				}
				return len(currentWebServer.Status.Pods)
			}, time.Minute*5, time.Second).Should(Equal(3), "WebServer should eventually have 3 pods running")

			for _, podStatus := range getWebServer(name).Status.Pods {
				podName := podStatus.Name
				fmt.Printf("Checking logs for pod: %s\n", podName)
				checkLogsContainString(podName, PayloadAcceptedLog)
			}
		})

		// Verifies that the operator handles case-insensitive label values correctly (e.g., "TRUE").
		It("CaseInSensitivityOfEnvTest", func() {
			webserver.Spec.UseInsightsClient = true
			webserver.Spec.EnvironmentVariables = []corev1.EnvVar{
				{
					Name:  "INSIGHTS_DEBUG",
					Value: "TRUE",
				},
			}

			createWebServer(webserver)

			podName := waitForPod()
			checkLogsContainString(podName, PayloadAcceptedLog)
		})

		// Checks stability after update
		// The agent must start successfully after pod update
		It("InsightsEnablementUpdateTest", func() {
			webserver.Spec.UseInsightsClient = false
			createWebServer(webserver)

			initialPodName := waitForPod()
			checkLogsNotContainString(initialPodName, StartingInsightsAgentLog)

			Eventually(func() error {
				currentWebServer := getWebServer(name)
				if currentWebServer == nil {
					return fmt.Errorf("WebServer not found")
				}

				currentWebServer.Spec.UseInsightsClient = true
				currentWebServer.Spec.EnvironmentVariables = []corev1.EnvVar{
					{
						Name:  "INSIGHTS_DEBUG",
						Value: "true",
					},
				}

				return k8sClient.Update(context.Background(), currentWebServer)
			}, time.Minute, time.Second).Should(Succeed(), "Failed to update WebServer spec to enable Insights")

			var newPodName string
			Eventually(func() bool {
				currentWebServer := getWebServer(name)
				if currentWebServer == nil || len(currentWebServer.Status.Pods) == 0 {
					return false
				}

				if currentWebServer.Status.Pods[0].Name != initialPodName {
					newPodName = currentWebServer.Status.Pods[0].Name
					return true
				}
				return false
			}, time.Minute*2, time.Second).Should(BeTrue(), "Pod should be recreated after enabling Insights")

			checkLogsContainString(newPodName, PayloadAcceptedLog)
		})
	})
})
