package framework

import (
	"context"
	goctx "context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Minute * 5
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

// WebServerBasicTest runs basic operator tests
func WebServerBasicTest(t *testing.T, applicationTag string) {
	ctx, f := webServerTestSetup(t)
	defer ctx.Cleanup()

	if err := webServerBasicServerScaleTest(t, f, ctx, applicationTag); err != nil {
		t.Fatal(err)
	}
}

func webServerTestSetup(t *testing.T) (*framework.Context, *framework.Framework) {
	ctx := framework.NewContext(t)
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		defer ctx.Cleanup()
		t.Fatalf("Failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		defer ctx.Cleanup()
		t.Fatalf("Failed to get namespace for testing context '%v': %v", ctx, err)
	}
	t.Logf("Testing in namespace %s", namespace)
	// get global framework variables
	f := framework.Global
	return ctx, f
}

func webServerBasicServerScaleTest(t *testing.T, f *framework.Framework, ctx *framework.Context, applicationTag string) error {
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	name := "example-webserver-" + unixEpoch()
	// create webServer custom resource
	// webServer := MakeBasicWebServer(namespace, name, "quay.io/jws-quickstarts/jws-operator-quickstart:"+applicationTag, 1)
	webServer := MakeBasicWebServer(namespace, name, "quay.io/jfclere/jws-image:5.4", 1)
	err = CreateAndWaitUntilReady(f, ctx, t, webServer)
	if err != nil {
		return err
	}

	t.Logf("Application %s is deployed with %d instance\n", name, 1)

	context := goctx.TODO()

	// update the size to 2
	err = f.Client.Get(context, types.NamespacedName{Name: name, Namespace: namespace}, webServer)
	if err != nil {
		return err
	}
	webServer.Spec.Replicas = 2
	err = f.Client.Update(context, webServer)
	if err != nil {
		return err
	}
	t.Logf("Updated application %s size to %d\n", name, webServer.Spec.Replicas)

	// check that the resource have been updated
	return WaitUntilReady(f, t, webServer)
}

func unixEpoch() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// IsOperatorLocal returns true if the LOCAL_OPERATOR env var is set to true.
func IsOperatorLocal() bool {
	val, ok := os.LookupEnv("LOCAL_OPERATOR")
	if !ok {
		return false
	}
	local, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return local
}

// CreateAndWaitUntilReady creates a WebServer resource and wait until it is ready
func CreateAndWaitUntilReady(f *framework.Framework, ctx *framework.Context, t *testing.T, server *webserversv1alpha1.WebServer) error {
	// use Context's create helper to create the object and add a cleanup function for the new object
	err := f.Client.Create(goctx.TODO(), server, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	// removing finalizers explicitly otherwise the removal could hang
	ctx.AddCleanupFn(
		func() error {
			// Removing deployment for not putting finalizers back to the WebServer
			name := server.ObjectMeta.Name
			namespace := server.ObjectMeta.Namespace
			deployment, err := f.KubeClient.AppsV1().Deployments(namespace).Get("jws-operator", metav1.GetOptions{})
			if err == nil && deployment != nil {
				t.Logf("Cleaning deployment '%v'\n", deployment.Name)
				f.Client.Delete(goctx.TODO(), deployment)
			}
			// Cleaning finalizer
			return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
				foundServer := &webserversv1alpha1.WebServer{}
				namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
				if errPoll := f.Client.Get(context.TODO(), namespacedName, foundServer); errPoll != nil {
					if apierrors.IsNotFound(errPoll) {
						t.Logf("No WebServer object '%v' to remove the finalizer at. Probably all cleanly finished before.\n", name)
						return true, nil
					}
					t.Logf("Cannot obtain object of the WebServer '%v', cause: %v\n", name, errPoll)
					return false, nil
				}
				foundServer.SetFinalizers([]string{})
				if errPoll := f.Client.Update(context.TODO(), foundServer); errPoll != nil {
					t.Logf("Cannot update WebServer '%v' with empty finalizers array, cause: %v\n", name, errPoll)
					return false, nil
				}
				t.Logf("Finalizer definition succesfully removed from the WebServer '%v'\n", name)
				return true, nil
			})
		},
	)

	return WaitUntilReady(f, t, server)
}

// WaitUntilReady waits until the number of pods matches the server spec size.
func WaitUntilReady(f *framework.Framework, t *testing.T, server *webserversv1alpha1.WebServer) error {
	name := server.ObjectMeta.Name
	size := server.Spec.Replicas

	t.Logf("Waiting until pods for %s are ready with size of %v", name, size)

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {

		podList := &corev1.PodList{}
		listOpts := []client.ListOption{
			client.InNamespace(server.Namespace),
			client.MatchingLabels(LabelsForWeb(server)),
		}
		err = f.Client.List(context.TODO(), podList, listOpts...)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("List of pods %s not found", name)

				return false, nil
			}
			t.Logf("Got error when getting pod list %s: %s", name, err)
			return false, err
		}

		// Testing for Ready?
		if int32(len(podList.Items)) == size {
			return true, nil
		}

		t.Logf("Waiting for full availability of %s pod list (%d/%d)\n", name, podList.Items, size)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("pods available (%d/%d)\n", size, size)

	return nil
}

// LabelsForWeb return a map of labels that are used for identification
//  of objects belonging to the particular WebServer instance
func LabelsForWeb(j *webserversv1alpha1.WebServer) map[string]string {
	labels := map[string]string{
		"deploymentConfig": j.Spec.ApplicationName,
		"WebServer":        j.Name,
	}
	// labels["app.kubernetes.io/name"] = j.Name
	// labels["app.kubernetes.io/managed-by"] = os.Getenv("LABEL_APP_MANAGED_BY")
	// labels["app.openshift.io/runtime"] = os.Getenv("LABEL_APP_RUNTIME")
	if j.Labels != nil {
		for labelKey, labelValue := range j.Labels {
			labels[labelKey] = labelValue
		}
	}
	return labels
}
