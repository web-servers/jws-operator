package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	imagev1 "github.com/openshift/api/image/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createWebServer(webServer *webserversv1alpha1.WebServer) {
	// create the webserver
	Expect(k8sClient.Create(ctx, webServer)).Should(Succeed())
	createdWebserver := getWebServer(webServer.Name)
	fmt.Printf("new WebServer Name: %s Namespace: %s\n", createdWebserver.Name, createdWebserver.Namespace)
}

func deleteWebServer(webServer *webserversv1alpha1.WebServer) {
	Expect(k8sClient.Delete(ctx, webServer)).Should(Succeed())
	webserverLookupKey := types.NamespacedName{Name: webServer.Name, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, webserverLookupKey, &webserversv1alpha1.WebServer{})
		return apierrors.IsNotFound(err)
	}, "2m", "5s").Should(BeTrue(), "the webserver should be deleted")
}

func getWebServer(name string) *webserversv1alpha1.WebServer {
	createdWebserver := &webserversv1alpha1.WebServer{}
	webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
		return err == nil
	}, time.Second*10, time.Millisecond*250).Should(BeTrue())

	return createdWebserver
}

func createImageStream(imgStream *imagev1.ImageStream) {
	Eventually(func() bool {
		err := k8sClient.Create(ctx, imgStream)
		if err != nil {
			thetest.Logf("Error: %s", err)
			return false
		}
		thetest.Logf("Image stream %s was created\n", imgStream.Name)
		return true
	}, time.Second*30, time.Millisecond*250).Should(BeTrue())
}

func deleteImageStream(imgStream *imagev1.ImageStream) {
	Expect(k8sClient.Delete(ctx, imgStream)).Should(Succeed())
	imgStreamLookupKey := types.NamespacedName{Name: imgStream.Name, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, imgStreamLookupKey, &imagev1.ImageStream{})
		return apierrors.IsNotFound(err)
	}, "2m", "5s").Should(BeTrue(), "the image stream should be deleted")
}

func createSecret(secret *corev1.Secret) {
	Eventually(func() bool {
		err := k8sClient.Create(ctx, secret)
		if err != nil {
			thetest.Logf("Error: %s", err)
			return false
		}
		thetest.Logf("Secret %s was created\n", secret.Name)
		return true
	}, time.Second*30, time.Millisecond*250).Should(BeTrue())
}

func deleteSecret(secret *corev1.Secret) {
	Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
	imgStreamLookupKey := types.NamespacedName{Name: secret.Name, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, imgStreamLookupKey, &corev1.Secret{})
		return apierrors.IsNotFound(err)
	}, "2m", "5s").Should(BeTrue(), "the secret should be deleted")
}

func executeCommandOnPod(podName string, containerName string, command []string) (string, string, error) {
	var stdout, stderr bytes.Buffer

	req := restClient.
		Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		return "", "", err
	}

	return stdout.String(), stderr.String(), nil
}

func getURL(name string, testURI string, expectedOutput []byte) []byte {
	var body []byte
	var res *http.Response

	Eventually(func() bool {
		createdWebServer := getWebServer(name)

		if len(createdWebServer.Status.Hosts) == 0 {
			return false
		}

		URL := "http://" + createdWebServer.Status.Hosts[0] + testURI

		fmt.Printf("GET request: %s \n", URL)
		req, err := http.NewRequest("GET", URL, nil)
		if err != nil {
			return false
		}

		httpClient := &http.Client{}
		res, err = httpClient.Do(req)

		if err != nil {
			fmt.Printf("Error: %s; \n", err.Error())
			return false
		}

		if res.StatusCode != http.StatusOK {
			fmt.Printf("StatusCode: %d \n", res.StatusCode)
			return false
		}

		body, err = io.ReadAll(res.Body)
		Expect(res.Body.Close()).Should(Succeed())

		if err != nil {
			fmt.Printf("BodyError: %s\n", err.Error())
			return false
		}

		if len(expectedOutput) > 0 {
			return bytes.Contains(body, expectedOutput)
		}

		return true
	}, time.Minute*5, time.Second*1).Should(BeTrue(), "URL testing failed")

	return body
}

func checkOperatorLogs() {
	container := "manager"
	podList := &corev1.PodList{}

	labels := map[string]string{
		"app.kubernetes.io/name": "jws-operator",
		"control-plane":          "controller-manager",
	}

	listOpts := []client.ListOption{
		client.MatchingLabels(labels),
		client.InNamespace(namespace),
	}
	Expect(k8sClient.List(ctx, podList, listOpts...)).Should(Succeed())
	Expect(podList.Items).Should(HaveLen(1), fmt.Sprintf("Not able to find operator pod: len(podList.Items) = %d", len(podList.Items)))

	operatorPod := podList.Items[0]
	podLogOptions := &corev1.PodLogOptions{
		Container: container,
	}

	podLogs, err := clientset.CoreV1().Pods(operatorPod.Namespace).GetLogs(operatorPod.Name, podLogOptions).Stream(ctx)
	Expect(err).ShouldNot(HaveOccurred())

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, podLogs)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(podLogs.Close()).ShouldNot(HaveOccurred())
	Expect(buffer.Bytes()).ShouldNot(BeEmpty(), "Not able to read controller-manager's logs.")

	if bytes.Contains(bytes.ToLower(buffer.Bytes()), []byte("error")) {
		fmt.Println(">>>> Pod's log <<<<")
		fmt.Println(buffer.String())
		fmt.Println(">>>> Pod's log <<<<")
		Expect(true).Should(BeFalse())
	}
}

// getPodLogs returns the feed logs as a string.
// Returns an empty string in case of an error, so that it can Eventually try again.
func getPodLogs(namespace string, podName string) string {
	podLogOptions := &corev1.PodLogOptions{}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions)

	stream, err := req.Stream(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to open stream for pod %s: %v\n", podName, err)
		return ""
	}

	defer func(stream io.ReadCloser) {
		err := stream.Close()
		if err != nil {
			fmt.Printf("Warning: failed to close stream for pod %s: %v\n", podName, err)
		}
	}(stream)

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, stream)
	if err != nil {
		fmt.Printf("Warning: failed to copy logs for pod %s: %v\n", podName, err)
		return ""
	}

	return buffer.String()
}

func forcePodRestart(namespace string, podName string) {
	oldPod := &corev1.Pod{}
	key := types.NamespacedName{Name: podName, Namespace: namespace}

	err := k8sClient.Get(context.Background(), key, oldPod)
	if apierrors.IsNotFound(err) {
		fmt.Printf("Pod %s already deleted\n", podName)
		return
	}
	oldUID := oldPod.UID

	fmt.Printf("Deleting pod %s (UID: %s)\n", podName, oldUID)

	err = k8sClient.Delete(context.Background(), oldPod)
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred(), "Failed to trigger pod deletion")
	}

	Eventually(func() bool {
		newPod := &corev1.Pod{}
		err := k8sClient.Get(context.Background(), key, newPod)

		if apierrors.IsNotFound(err) {
			return true
		}

		if err != nil {
			return false
		}

		if newPod.UID != oldUID && newPod.Status.Phase == corev1.PodRunning {
			fmt.Printf("Pod recreated successfully with same name: %s (New UID: %s)\n", podName, newPod.UID)
			return true
		}

		return false
	}, time.Minute*3, time.Second*2).Should(BeTrue(), "Pod did not vanish or restart within timeout")
}

func createPVC(pvc *corev1.PersistentVolumeClaim) {
	Expect(k8sClient.Create(ctx, pvc)).Should(Succeed())
	currentPvc := getPVC(pvc.Name)
	fmt.Printf("new PVC Name: %s Namespace: %s\n", currentPvc.Name, currentPvc.Namespace)
}

func getPVC(name string) *corev1.PersistentVolumeClaim {
	currentPvc := &corev1.PersistentVolumeClaim{}
	pvcLookupKey := types.NamespacedName{Name: name, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, pvcLookupKey, currentPvc)
		return err == nil
	}, time.Second*10, time.Millisecond*250).Should(BeTrue())

	return currentPvc
}

func deleteAllPVCs(namespace string) {
	pvcList := &corev1.PersistentVolumeClaimList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := k8sClient.List(context.Background(), pvcList, listOpts...)
	if err != nil {
		fmt.Printf("Error listing PVCs for cleanup: %v\n", err)
		return
	}

	if len(pvcList.Items) == 0 {
		fmt.Printf("No PVCs found in namespace %s to clean up.\n", namespace)
		return
	}

	fmt.Printf("Cleaning up %d PVC(s) in namespace %s...\n", len(pvcList.Items), namespace)

	for _, pvc := range pvcList.Items {
		pvcToDelete := pvc
		err := k8sClient.Delete(context.Background(), &pvcToDelete)

		if err != nil && !apierrors.IsNotFound(err) {
			fmt.Printf("Failed to delete PVC %s: %v\n", pvcToDelete.Name, err)
		} else {
			fmt.Printf("Triggered deletion for PVC: %s\n", pvcToDelete.Name)
		}
	}
}

func getPodContainerName(podName string) string {
	pod := &corev1.Pod{}
	Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: namespace}, pod)).To(Succeed())
	Expect(pod.Spec.Containers).NotTo(BeEmpty())
	return pod.Spec.Containers[0].Name
}

func waitForBuildPodsToSucceed() {
	// Get all the pods
	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	// Check that all the building pods succeeded
	Eventually(func() bool {
		if k8sClient.List(ctx, podList, listOpts...) != nil {
			return false
		}

		for _, pod := range podList.Items {
			if strings.HasSuffix(pod.Name, "-build") {
				// Check if Phase is "Succeeded"
				if pod.Status.Phase != "Succeeded" {
					fmt.Printf("Pod %s is currently: %s\n", pod.Name, pod.Status.Phase)
					return false
				}
			}
		}
		return true
	}, time.Minute*5, time.Second*5).Should(BeTrue(), "Building pods took too long time.")
}
