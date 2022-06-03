package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	kbappsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func isOpenShift(c *rest.Config) bool {
	var err error
	var dcclient *discovery.DiscoveryClient
	dcclient, err = discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		log.Info("isOpenShift discovery.NewDiscoveryClientForConfig has encountered a problem")
		return false
	}
	apiList, err := dcclient.ServerGroups()
	if err != nil {
		log.Info("isOpenShift client.ServerGroups has encountered a problem")
		return false
	}
	for _, v := range apiList.Groups {
		log.Info(v.Name)
		if v.Name == "route.openshift.io" {

			log.Info("route.openshift.io was found in apis, platform is OpenShift")
			return true
		}
	}
	return false
}

func (r *WebServerReconciler) getWebServer(ctx context.Context, request reconcile.Request) (*webserversv1alpha1.WebServer, error) {
	webServer := &webserversv1alpha1.WebServer{}
	err := r.Client.Get(ctx, request.NamespacedName, webServer)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("WebServer resource not found. Ignoring since object must have been deleted")
			return webServer, err
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get WebServer resource")
		return webServer, err
	}
	return webServer, nil
}

func (r *WebServerReconciler) setDefaultValues(webServer *webserversv1alpha1.WebServer) *webserversv1alpha1.WebServer {

	if webServer.Spec.WebImage != nil && webServer.Spec.WebImage.WebApp != nil {
		webApp := webServer.Spec.WebImage.WebApp
		if webApp.Name == "" {
			log.Info("WebServer.Spec.Image.WebApp.Name is not set, setting value to 'ROOT.war'")
			webApp.Name = "ROOT.war"
		}
		if webApp.Builder.ApplicationBuildScript == "" {
			log.Info("WebServer.Spec.Image.WebApp.Builder.ApplicationBuildScript is not set, will use the default build script")
		}
		if webApp.WebAppWarImagePushSecret == "" {
			log.Info("WebServer.Spec.Image.WebApp.WebAppWarImagePushSecret is not set!!!")
		}
	}

	return webServer

}

func (r *WebServerReconciler) generateWebAppBuildScript(webServer *webserversv1alpha1.WebServer) string {
	webApp := webServer.Spec.WebImage.WebApp
	webAppWarFileName := webApp.Name + ".war"
	webAppSourceRepositoryURL := webApp.SourceRepositoryURL
	webAppSourceRepositoryRef := webApp.SourceRepositoryRef
	webAppSourceRepositoryContextDir := webApp.SourceRepositoryContextDir

	return fmt.Sprintf(`
		webAppWarFileName=%s;
		webAppSourceRepositoryURL=%s;
		webAppSourceRepositoryRef=%s;
		webAppSourceRepositoryContextDir=%s;

		# Some pods don't have root privileges, so the build takes place in /tmp
		cd tmp;

		# Create a custom .m2 repo in a location where no root privileges are required
		mkdir -p /tmp/.m2/repo;

		# Create custom maven settings that change the location of the .m2 repo
		echo '<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"' >> /tmp/.m2/settings.xml
		echo 'xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 https://maven.apache.org/xsd/settings-1.0.0.xsd">' >> /tmp/.m2/settings.xml
		echo '<localRepository>/tmp/.m2/repo</localRepository>' >> /tmp/.m2/settings.xml
		echo '</settings>' >> /tmp/.m2/settings.xml

		if [ -z ${webAppSourceRepositoryURL} ]; then
			echo "Need an URL like https://github.com/jfclere/demo-webapp.git";
			exit 1;
		fi;

		git clone ${webAppSourceRepositoryURL};
		if [ $? -ne 0 ]; then
			echo "Can't clone ${webAppSourceRepositoryURL}";
			exit 1;
		fi;

		# Get the name of the source code directory
		DIR=$(echo ${webAppSourceRepositoryURL##*/});
		DIR=$(echo ${DIR%%.*});

		cd ${DIR};

		if [ ! -z ${webAppSourceRepositoryRef} ]; then
			git checkout ${webAppSourceRepositoryRef};
		fi;

		if [ ! -z ${webAppSourceRepositoryContextDir} ]; then
			cd ${webAppSourceRepositoryContextDir};
		fi;

		# Builds the webapp using the custom maven settings
		mvn clean install -gs /tmp/.m2/settings.xml;
		if [ $? -ne 0 ]; then
			echo "mvn install failed please check the pom.xml in ${webAppSourceRepositoryURL}";
			exit 1;
		fi

		# Copies the resulting war to the mounted persistent volume
		cp target/*.war /mnt/${webAppWarFileName};`,
		webAppWarFileName,
		webAppSourceRepositoryURL,
		webAppSourceRepositoryRef,
		webAppSourceRepositoryContextDir,
	)
}

func (r *WebServerReconciler) createService(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *corev1.Service, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Service: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new Service: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service: "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

// Look for a rolebinding that allows KUBEPing

func (r *WebServerReconciler) checkRoleBinding(ctx context.Context, webServer *webserversv1alpha1.WebServer) bool {
	roleBindingList := &rbac.RoleBindingList{}
	listOpts := []client.ListOption{
		client.InNamespace(webServer.Namespace),
	}
	err := r.Client.List(ctx, roleBindingList, listOpts...)
	if err != nil {
		log.Error(err, "checkRoleBinding")
		return false
	}
	var roleBindings []rbac.RoleBinding = roleBindingList.Items
	for _, rolebinding := range roleBindings {
		// Look for ServiceAccount / default in subjects:
		for _, subject := range rolebinding.Subjects {
			if subject.Kind == "ServiceAccount" && subject.Name == "default" && subject.Namespace == webServer.Namespace {
				// now check the roleRef for ClusterRole/view
				if rolebinding.RoleRef.Kind == "ClusterRole" && rolebinding.RoleRef.Name == "view" {
					log.Info("checkRoleBinding bingo: " + rolebinding.Name + " should allow KUBEPing")
					if rolebinding.Name == "view-kubeping-"+webServer.Name {
						// We already created it for this webserver
						return true
					}
					if !strings.HasPrefix(rolebinding.Name, "view-kubeping-") {
						// it was create by the admin via oc command something like view-nn or by hands.
						return true
					}
					// Here was created for another webserver, the operator will create one for this webserver
					// remember the RoleBinding is removed if the webserver is deleted.
					// Note that removing it when the cluster is using KUBEPing will cause 403 in the tomcat pods.
				}
			}
		}
	}
	return false
}

// Test for the "view" RoleBinding and if not existing try to create it, if that fails we can't use useKUBEPing
// first bool = Role Binding exist
// second bool = We need to requeue or not...

func (r *WebServerReconciler) createRoleBinding(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *rbac.RoleBinding, resourceName string, resourceNamespace string) (bool, bool, error) {
	// First try to check if there is a roleBinding that allows KUBEPing
	checked := r.checkRoleBinding(ctx, webServer)
	if checked {
		return true, false, nil
	}

	// Then try to create it.
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new RoleBinding: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new RoleBinding: "+resourceName+" Namespace: "+resourceNamespace)
			return false, false, err
		}
		// Resource created successfully - return and requeue
		// return true, true, nil
		return true, false, nil
	} else if err != nil {
		log.Error(err, "Failed to get RoleBinding "+resourceName)
		return false, false, err
	}
	return true, false, nil
}

func (r *WebServerReconciler) createConfigMap(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *corev1.ConfigMap, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new ConfigMap: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new ConfigMap: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createBuildPod(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *corev1.Pod, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Pod: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new Pod: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Pod: "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createDeployment(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *kbappsv1.Deployment, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: resourceNamespace}, resource)

	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Deployment: " + resourceName + " Namespace: " + resourceNamespace)
		resource.ObjectMeta.Labels["webserver-hash"] = r.getWebServerHash(webServer)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new Deployment: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment: "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createImageStream(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *imagev1.ImageStream, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new ImageStream: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new ImageStream: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ImageStream: "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createBuildConfig(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *buildv1.BuildConfig, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new BuildConfig: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new BuildConfig: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get BuildConfig: "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createDeploymentConfig(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *appsv1.DeploymentConfig, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new DeploymentConfig: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new DeploymentConfig: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get DeploymentConfig: "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createRoute(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *routev1.Route, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Route: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Client.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new Route: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Route: "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) checkBuildPodPhase(buildPod *corev1.Pod) (reconcile.Result, error) {
	if buildPod.Status.Phase != corev1.PodSucceeded {
		switch buildPod.Status.Phase {
		case corev1.PodFailed:
			log.Info("Application build failed: " + buildPod.Status.Message)
		case corev1.PodPending:
			log.Info("Application build pending")
		case corev1.PodRunning:
			log.Info("Application is still being built")
		default:
			log.Info("Unknown build pod status")
		}
		return reconcile.Result{RequeueAfter: (5 * time.Second)}, nil
	}
	return reconcile.Result{}, nil
}

// getPodList lists pods which belongs to the Web server
// the pods are differentiated based on the selectors
func (r *WebServerReconciler) getPodList(ctx context.Context, webServer *webserversv1alpha1.WebServer) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.InNamespace(webServer.Namespace),
		client.MatchingLabels(r.generateLabelsForWeb(webServer)),
	}
	err := r.Client.List(ctx, podList, listOpts...)

	if err == nil {
		// sorting pods by number in the name
		r.sortPodListByName(podList)
	}
	return podList, err
}

// generateLabelsForWeb return a map of labels that are used for identification
// of objects belonging to the particular WebServer instance
// NOTE: that is ONLY for application pods! (not for the builder or any helpers
func (r *WebServerReconciler) generateLabelsForWeb(webServer *webserversv1alpha1.WebServer) map[string]string {
	labels := map[string]string{
		"deploymentConfig": webServer.Spec.ApplicationName,
		"WebServer":        webServer.Name,
		"application":      webServer.Spec.ApplicationName,
		// app.kubernetes.io/name is used for HPA selector like in wildfly
		"app.kubernetes.io/name": webServer.Name,
	}
	// Those are from the wildfly operator (in their Dockerfile)
	// labels["app.kubernetes.io/managed-by"] = os.Getenv("LABEL_APP_MANAGED_BY")
	// labels["app.openshift.io/runtime"] = os.Getenv("LABEL_APP_RUNTIME")
	if webServer.Labels != nil {
		for labelKey, labelValue := range webServer.Labels {
			log.Info("labels: " + labelKey + " : " + labelValue)
			labels[labelKey] = labelValue
		}
	}
	return labels
}

// sortPodListByName sorts the pod list by number in the name
//  expecting the format which the StatefulSet works with which is `<podname>-<number>`
func (r *WebServerReconciler) sortPodListByName(podList *corev1.PodList) *corev1.PodList {
	sort.SliceStable(podList.Items, func(i, j int) bool {
		return podList.Items[i].ObjectMeta.Name < podList.Items[j].ObjectMeta.Name
	})
	return podList
}

// getPodStatus returns the pod names of the array of pods passed in
func (r *WebServerReconciler) getPodStatus(pods []corev1.Pod) ([]webserversv1alpha1.PodStatus, bool) {
	var requeue = false
	var podStatuses []webserversv1alpha1.PodStatus
	for _, pod := range pods {
		podState := webserversv1alpha1.PodStateFailed

		switch pod.Status.Phase {
		case corev1.PodPending:
			podState = webserversv1alpha1.PodStatePending
		case corev1.PodRunning:
			podState = webserversv1alpha1.PodStateActive
		}

		podStatuses = append(podStatuses, webserversv1alpha1.PodStatus{
			Name:  pod.Name,
			PodIP: pod.Status.PodIP,
			State: podState,
		})
		if pod.Status.PodIP == "" {
			requeue = true
		}
	}
	if requeue {
		log.Info("Some pods don't have an IP address yet, reconciliation requeue scheduled")
	}
	return podStatuses, requeue
}

// updateWebServerStatus updates status of the WebServer resource.
func (r *WebServerReconciler) updateWebServerStatus(webServer *webserversv1alpha1.WebServer, client client.Client, ctx context.Context) error {
	log.Info("Updating the status of WebServer")

	if err := r.Status().Update(ctx, webServer); err != nil {
		log.Error(err, "Failed to update the status of WebServer")
		return err
	}

	log.Info("The status of WebServer was updated successfully")
	return nil
}

// Calculate a hash of the Spec (configuration) to redeploy/rebuild if needed.
func (r *WebServerReconciler) getWebServerHash(webServer *webserversv1alpha1.WebServer) string {
	h := sha256.New()
	h.Write([]byte("ApplicationName:" + webServer.Spec.ApplicationName))
	// No need to recreate h.Write([]byte("Replicas:" + fmt.Sprint(webServer.Spec.Replicas)))
	h.Write([]byte("UseSessionClustering:" + fmt.Sprint(webServer.Spec.UseSessionClustering)))

	/* add the labels */
	if webServer.ObjectMeta.Labels != nil {
		for labelKey, labelValue := range webServer.ObjectMeta.Labels {
			h.Write([]byte(labelKey + ":" + labelValue))
		}
	}
	if webServer.Spec.WebImage != nil {
		/* Same for WebImage */
		h.Write([]byte("ApplicationImage:" + webServer.Spec.WebImage.ApplicationImage))
		if webServer.Spec.WebImage.ImagePullSecret != "" {
			h.Write([]byte("ImagePullSecret:" + webServer.Spec.WebImage.ImagePullSecret))
		}
		if webServer.Spec.WebImage.WebApp != nil {
			/* Same for WebApp */
			if webServer.Spec.WebImage.WebApp.Name != "" {
				h.Write([]byte("Name:" + webServer.Spec.WebImage.WebApp.Name))
			}
			h.Write([]byte("SourceRepositoryURL:" + webServer.Spec.WebImage.WebApp.SourceRepositoryURL))
			if webServer.Spec.WebImage.WebApp.SourceRepositoryRef != "" {
				h.Write([]byte("SourceRepositoryRef:" + webServer.Spec.WebImage.WebApp.SourceRepositoryRef))
			}
			if webServer.Spec.WebImage.WebApp.SourceRepositoryContextDir != "" {
				h.Write([]byte("SourceRepositoryContextDir:" + webServer.Spec.WebImage.WebApp.SourceRepositoryContextDir))
			}
			h.Write([]byte("WebAppWarImage:" + webServer.Spec.WebImage.WebApp.WebAppWarImage))
			h.Write([]byte("WebAppWarImagePushSecret:" + webServer.Spec.WebImage.WebApp.WebAppWarImagePushSecret))
			if webServer.Spec.WebImage.WebApp.Builder != nil {
				/* Same for Builder */
				h.Write([]byte("Image:" + webServer.Spec.WebImage.WebApp.Builder.Image))
				h.Write([]byte("ApplicationBuildScript:" + webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript))
			}
		}
		if webServer.Spec.WebImage.WebServerHealthCheck != nil {
			/* Same for WebServerHealthCheck */
			h.Write([]byte("ServerReadinessScript:" + webServer.Spec.WebImage.WebServerHealthCheck.ServerReadinessScript))
			if webServer.Spec.WebImage.WebServerHealthCheck.ServerLivenessScript != "" {
				h.Write([]byte("ServerLivenessScript:" + webServer.Spec.WebImage.WebServerHealthCheck.ServerLivenessScript))
			}

		}
	}
	if webServer.Spec.WebImageStream != nil {
		/* Same for WebImageStream */
		h.Write([]byte("ImageStreamName:" + webServer.Spec.WebImageStream.ImageStreamName))
		h.Write([]byte("ImageStreamNamespace:" + webServer.Spec.WebImageStream.ImageStreamNamespace))
		if webServer.Spec.WebImageStream.WebSources != nil {
			h.Write([]byte("SourceRepositoryURL:" + webServer.Spec.WebImageStream.WebSources.SourceRepositoryURL))
			if webServer.Spec.WebImageStream.WebSources.SourceRepositoryRef != "" {
				h.Write([]byte("SourceRepositoryRef:" + webServer.Spec.WebImageStream.WebSources.SourceRepositoryRef))
			}
			if webServer.Spec.WebImageStream.WebSources.ContextDir != "" {
				h.Write([]byte("SourceRepositoryContextDir:" + webServer.Spec.WebImageStream.WebSources.ContextDir))
			}
			if webServer.Spec.WebImageStream.WebSources.WebSourcesParams != nil {
				if webServer.Spec.WebImageStream.WebSources.WebSourcesParams.MavenMirrorURL != "" {
					h.Write([]byte("MavenMirrorURL:" + webServer.Spec.WebImageStream.WebSources.WebSourcesParams.MavenMirrorURL))
				}
				if webServer.Spec.WebImageStream.WebSources.WebSourcesParams.ArtifactDir != "" {
					h.Write([]byte("ArtifactDir:" + webServer.Spec.WebImageStream.WebSources.WebSourcesParams.ArtifactDir))
				}
				if webServer.Spec.WebImageStream.WebSources.WebSourcesParams.GenericWebhookSecret != "" {
					h.Write([]byte("GenericWebhookSecret:" + webServer.Spec.WebImageStream.WebSources.WebSourcesParams.GenericWebhookSecret))
				}
				if webServer.Spec.WebImageStream.WebSources.WebSourcesParams.GithubWebhookSecret != "" {
					h.Write([]byte("GithubWebhookSecret:" + webServer.Spec.WebImageStream.WebSources.WebSourcesParams.GithubWebhookSecret))
				}
			}
		}
	}
	/* rules for labels: '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"} */
	enc := base64.NewEncoding("qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM_.0123456789")
	enc = enc.WithPadding(base64.NoPadding)
	return "A" + enc.EncodeToString(h.Sum(nil)) + "A"
}

// Add an annotation to the webServer for the KUBEPing

func (r *WebServerReconciler) setUseKUBEPing(ctx context.Context, webServer *webserversv1alpha1.WebServer, kubeping bool) (bool, error) {
	skubeping := "false"
	if kubeping {
		skubeping = "true"
	}
	needUpdate := false
	annotations := webServer.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
		webServer.Annotations = annotations
		needUpdate = true
		annotations["UseKUBEPing"] = skubeping
	} else {
		if strings.Compare(skubeping, annotations["UseKUBEPing"]) != 0 {
			annotations["UseKUBEPing"] = skubeping
			needUpdate = true
		}
	}
	if needUpdate {
		log.Info("The UseKUBEPing annotation is being updated")
		err := r.Client.Update(ctx, webServer)
		if err != nil {
			log.Error(err, "Failed to update WebServer UseKUBEPing annotation")
		}
		return true, err
	}
	return false, nil
}
func (r *WebServerReconciler) getUseKUBEPing(webServer *webserversv1alpha1.WebServer) bool {
	annotations := webServer.Annotations
	if annotations != nil {
		skubeping := annotations["UseKUBEPing"]
		if skubeping != "" {
			if strings.Compare(skubeping, "false") == 0 {
				return false
			}
		}
	}
	return true
}
func (r *WebServerReconciler) needgetUseKUBEPing(webServer *webserversv1alpha1.WebServer) bool {
	annotations := webServer.Annotations
	if annotations != nil {
		skubeping := annotations["UseKUBEPing"]
		if skubeping != "" {
			return false
		}
	}
	return true
}

// CustomResourceDefinitionExists returns true if the CRD exists in the cluster
func CustomResourceDefinitionExists(gvk schema.GroupVersionKind, c *rest.Config) bool {

	client, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		return false
	}
	api, err := client.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return false
	}
	for _, a := range api.APIResources {
		if a.Kind == gvk.Kind {
			return true
		}
	}
	return false
}
