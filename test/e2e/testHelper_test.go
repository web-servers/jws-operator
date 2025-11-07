package e2e

import (
	"bytes"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	. "github.com/onsi/gomega"
	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
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
	}, "2m", "5s").Should(BeTrue(), "the webserver should be deleted")
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
