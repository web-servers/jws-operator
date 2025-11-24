package controller

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"

	buildv1 "github.com/openshift/api/build/v1"
	imagestreamv1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	kbappsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ownerUIDIndex = ".metadata.ownerReference.uid"
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
	err := r.Get(ctx, request.NamespacedName, webServer)
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

func (r *WebServerReconciler) webImageConfiguration(ctx context.Context, webServer *webserversv1alpha1.WebServer) (ctrl.Result, error) {
	var result ctrl.Result
	var err error

	// Check if a webapp needs to be built
	if webServer.Spec.WebImage.WebApp != nil && webServer.Spec.WebImage.WebApp.SourceRepositoryURL != "" && webServer.Spec.WebImage.WebApp.Builder != nil && webServer.Spec.WebImage.WebApp.Builder.Image != "" {

		// Create a ConfigMap for custom build script
		if webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript != "" {
			configMap := r.generateConfigMapForCustomBuildScript(webServer)
			result, err = r.createConfigMap(ctx, configMap, configMap.Name, configMap.Namespace)
			if err != nil || result != (ctrl.Result{}) {
				return result, err
			}
			// Check the script has changed, if yes delete it and requeue
			if configMap.Data["build.sh"] != webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript {
				// Just Delete and requeue
				err = r.Delete(ctx, configMap)
				if err != nil && errors.IsNotFound(err) {
					return ctrl.Result{}, nil
				}
				log.Info("Webserver hash changed: Delete Builder ConfigMap and requeue reconciliation")
				return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
			}
		}

		// Check if a build Pod for the webapp already exists, and if not create a new one
		buildPod := r.generateBuildPod(webServer)
		log.Info("WebServe createBuildPod: " + buildPod.Name + " in " + buildPod.Namespace + " using: " + buildPod.Spec.Volumes[0].Secret.SecretName + " and: " + buildPod.Spec.Containers[0].Image)
		result, err = r.createBuildPod(ctx, buildPod, buildPod.Name, buildPod.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			return result, err
		}

		// Check if we need to delete it and recreate it.
		currentHash := r.getWebServerHash(webServer)
		if buildPod.Labels["webserver-hash"] != currentHash {
			// Just Delete and requeue
			err = r.Delete(ctx, buildPod)
			if err != nil && errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			log.Info("Webserver hash changed: Delete BuildPod and requeue reconciliation")
			return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
		}

		// Is the build pod ready.
		result = r.checkBuildPodPhase(buildPod)
		if result != (ctrl.Result{}) {
			return result, nil
		}

	}

	applicationImage := webServer.Spec.WebImage.ApplicationImage

	if webServer.Spec.WebImage.WebApp != nil {
		applicationImage = webServer.Spec.WebImage.WebApp.WebAppWarImage
	}

	if webServer.Spec.Volume != nil && len(webServer.Spec.Volume.VolumeClaimTemplates) > 0 {
		return r.continueWithStatefulSet(ctx, webServer, applicationImage)
	} else {
		return r.continueWithDeployment(ctx, webServer, applicationImage)
	}
}

func (r *WebServerReconciler) continueWithDeployment(ctx context.Context, webServer *webserversv1alpha1.WebServer, image string) (ctrl.Result, error) {
	updateDeployment := false

	// Check if a Deployment already exists, and if not create a new one
	deployment := r.generateDeployment(webServer, image)
	log.Info("WebServe createDeployment: " + deployment.Name + " in " + deployment.Namespace + " using: " + deployment.Spec.Template.Spec.Containers[0].Image)
	result, err := r.createDeployment(ctx, webServer, deployment, deployment.Name, deployment.Namespace)
	if err != nil || result != (ctrl.Result{}) {
		if err != nil {
			log.Info("WebServer can't create deployment")
		}
		return result, err
	}

	// Check if we need to update it.
	currentHash := r.getWebServerHash(webServer)
	if deployment.Labels["webserver-hash"] == "" {
		deployment.Labels["webserver-hash"] = currentHash
		updateDeployment = true
	} else {
		if deployment.Labels["webserver-hash"] != currentHash {
			// Just Update and requeue
			r.generateUpdatedDeployment(webServer, deployment, image)
			deployment.Labels["webserver-hash"] = currentHash
			err = r.Update(ctx, deployment)
			if err != nil {
				log.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
				if errors.IsConflict(err) {
					log.V(1).Info(err.Error())
				} else {
					return ctrl.Result{}, nil
				}
			}
			log.Info("Webserver hash changed: Update Deployment and requeue reconciliation")
			return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
		}
	}

	foundImage := deployment.Spec.Template.Spec.Containers[0].Image
	if webServer.Spec.WebImage != nil && webServer.Spec.WebImage.ApplicationImage != "" && webServer.Spec.WebImage.ApplicationImage != foundImage {
		// if we are using a builder that it normal otherwise we need to redeploy.
		if webServer.Spec.WebImage.WebApp == nil {
			log.Info("WebServer application image change detected. Deployment update scheduled")
			deployment.Spec.Template.Spec.Containers[0].Image = webServer.Spec.WebImage.ApplicationImage
			updateDeployment = true
		}
	}

	// Handle Scaling
	foundReplicas := *deployment.Spec.Replicas
	replicas := webServer.Spec.Replicas
	if foundReplicas != replicas {
		log.Info("Deployment replicas number does not match the WebServer specification. Deployment update scheduled")
		deployment.Spec.Replicas = &replicas
		updateDeployment = true
	}

	if updateDeployment {
		err = r.Update(ctx, deployment)
		if err != nil {
			log.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			if errors.IsConflict(err) {
				log.V(1).Info(err.Error())
			} else {
				return ctrl.Result{}, err
			}
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	return result, err
}

func (r *WebServerReconciler) continueWithStatefulSet(ctx context.Context, webServer *webserversv1alpha1.WebServer, image string) (ctrl.Result, error) {
	updateStatefulSet := false

	statefulset := r.generateStatefulSet(webServer, image)
	log.Info("WebServe createStatefulSet: " + statefulset.Name + " in " + statefulset.Namespace + " using: " + statefulset.Spec.Template.Spec.Containers[0].Image)
	result, err := r.createStatefulSet(ctx, webServer, statefulset)
	if err != nil || result != (ctrl.Result{}) {
		if err != nil {
			log.Info("WebServer can't create deployment")
		}
		return result, err
	}

	// Check if we need to update it.
	currentHash := r.getWebServerHash(webServer)
	if statefulset.Labels["webserver-hash"] == "" {
		statefulset.Labels["webserver-hash"] = currentHash
		updateStatefulSet = true
	} else {
		if statefulset.Labels["webserver-hash"] != currentHash {
			// Just Update and requeue
			r.generateUpdatedStatefulSet(webServer, statefulset, image)
			statefulset.Labels["webserver-hash"] = currentHash
			err = r.Update(ctx, statefulset)
			if err != nil {
				log.Error(err, "Failed to update:", "Namespace", statefulset.Namespace, "Name", statefulset.Name)
				if errors.IsConflict(err) {
					log.V(1).Info(err.Error())
				} else {
					return ctrl.Result{}, nil
				}
			}
			log.Info("Webserver hash changed: Update Deployment and requeue reconciliation")
			return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
		}
	}

	foundImage := statefulset.Spec.Template.Spec.Containers[0].Image
	if webServer.Spec.WebImage.ApplicationImage != "" && webServer.Spec.WebImage.ApplicationImage != foundImage {
		// if we are using a builder that it normal otherwise we need to redeploy.
		if webServer.Spec.WebImage.WebApp == nil {
			log.Info("WebServer application image change detected. Deployment update scheduled")
			statefulset.Spec.Template.Spec.Containers[0].Image = webServer.Spec.WebImage.ApplicationImage
			updateStatefulSet = true
		}
	}

	// Handle Scaling
	foundReplicas := *statefulset.Spec.Replicas
	replicas := webServer.Spec.Replicas
	if foundReplicas != replicas {
		log.Info("Deployment replicas number does not match the WebServer specification. Deployment update scheduled")
		statefulset.Spec.Replicas = &replicas
		updateStatefulSet = true
	}

	if updateStatefulSet {
		err = r.Update(ctx, statefulset)
		if err != nil {
			log.Error(err, "Failed to update:", "Namespace", statefulset.Namespace, "Name", statefulset.Name)
			if errors.IsConflict(err) {
				log.V(1).Info(err.Error())
			} else {
				return ctrl.Result{}, err
			}
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	return result, err
}

//nolint:gocyclo
func (r *WebServerReconciler) webImageSourceConfiguration(ctx context.Context, webServer *webserversv1alpha1.WebServer) (ctrl.Result, error) {
	var result ctrl.Result
	var err error = nil

	imageStreamName := webServer.Spec.WebImageStream.ImageStreamName
	imageStreamNamespace := webServer.Spec.WebImageStream.ImageStreamNamespace

	// Check if we need to build the webapp from sources
	if webServer.Spec.WebImageStream.WebSources != nil {

		// Check if an Image Stream already exists, and if not create a new one
		imageStream := r.generateImageStream(webServer)
		result, err = r.createImageStream(ctx, imageStream, imageStream.Name, imageStream.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			return result, err
		}

		// Change the Image Stream that the deployment config will use later to deploy the webserver
		imageStreamName = imageStream.Name
		imageStreamNamespace = imageStream.Namespace

		is := &imagestreamv1.ImageStream{}

		err = r.Get(ctx, client.ObjectKey{
			Namespace: imageStreamNamespace,
			Name:      imageStreamName,
		}, is)

		if errors.IsNotFound(err) {
			log.Error(err, "Namespace/ImageStream doesn't exist.")
			return ctrl.Result{}, nil
		}
		dockerImageRepository := is.Status.DockerImageRepository
		log.Info("Using " + dockerImageRepository + " as applicationImage")

		// Check if a BuildConfig already exists, and if not create a new one
		buildConfig := r.generateBuildConfig(webServer)
		result, err = r.createBuildConfig(ctx, buildConfig, buildConfig.Name, buildConfig.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			return result, err
		}

		updateBuildConfig := false
		startNewBuild := false

		if buildConfig.Spec.Source.Git.URI != webServer.Spec.WebImageStream.WebSources.SourceRepositoryURL {
			buildConfig.Spec.Source.Git.URI = webServer.Spec.WebImageStream.WebSources.SourceRepositoryURL
			updateBuildConfig = true
			startNewBuild = true
		}

		if buildConfig.Spec.Source.Git.Ref != webServer.Spec.WebImageStream.WebSources.SourceRepositoryRef {
			buildConfig.Spec.Source.Git.Ref = webServer.Spec.WebImageStream.WebSources.SourceRepositoryRef
			updateBuildConfig = true
			startNewBuild = true
		}

		if buildConfig.Spec.Source.ContextDir != webServer.Spec.WebImageStream.WebSources.ContextDir {
			buildConfig.Spec.Source.ContextDir = webServer.Spec.WebImageStream.WebSources.ContextDir
			updateBuildConfig = true
			startNewBuild = true
		}

		if buildConfig.Spec.Strategy.SourceStrategy != nil && buildConfig.Spec.Strategy.SourceStrategy.From.Namespace != webServer.Spec.WebImageStream.ImageStreamNamespace {
			buildConfig.Spec.Strategy.SourceStrategy.From.Namespace = webServer.Spec.WebImageStream.ImageStreamNamespace
			updateBuildConfig = true
			startNewBuild = true
		}

		if buildConfig.Spec.Strategy.SourceStrategy != nil && buildConfig.Spec.Strategy.SourceStrategy.From.Name != webServer.Spec.WebImageStream.ImageStreamName+":latest" {
			buildConfig.Spec.Strategy.SourceStrategy.From.Name = webServer.Spec.WebImageStream.ImageStreamName + ":latest"
			updateBuildConfig = true
			startNewBuild = true
		}

		if webServer.Spec.WebImageStream.WebSources.WebhookSecrets != nil && webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Generic != "" {
			triggers := buildConfig.Spec.Triggers
			found := false

			for i := 0; i < len(triggers); i++ {
				if triggers[i].GenericWebHook != nil && triggers[i].GenericWebHook.SecretReference != nil {
					found = true

					if triggers[i].GenericWebHook.SecretReference.Name != webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Generic {
						triggers[i].GenericWebHook.SecretReference.Name = webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Generic
						updateBuildConfig = true
					}
				}
			}

			if !found {
				buildConfig.Spec.Triggers = append(triggers, buildv1.BuildTriggerPolicy{
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Generic,
					},
				})
				updateBuildConfig = true
			}
		} else {
			triggers := buildConfig.Spec.Triggers

			for i := 0; i < len(triggers); i++ {
				if triggers[i].GenericWebHook != nil && triggers[i].GenericWebHook.SecretReference != nil {
					buildConfig.Spec.Triggers = append(triggers[:i], triggers[i+1:]...)
					updateBuildConfig = true
				}
			}

		}

		if webServer.Spec.WebImageStream.WebSources.WebhookSecrets != nil && webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Github != "" {
			triggers := buildConfig.Spec.Triggers
			found := false

			for i := 0; i < len(triggers); i++ {
				if triggers[i].GitHubWebHook != nil && triggers[i].GitHubWebHook.SecretReference != nil {
					found = true
					if triggers[i].GitHubWebHook.SecretReference.Name != webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Github {
						triggers[i].GitHubWebHook.SecretReference.Name = webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Github
						updateBuildConfig = true
					}
				}
			}

			if !found {
				buildConfig.Spec.Triggers = append(triggers, buildv1.BuildTriggerPolicy{
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Github,
					},
				})
				updateBuildConfig = true
			}
		} else {
			triggers := buildConfig.Spec.Triggers

			for i := 0; i < len(triggers); i++ {
				if triggers[i].GitHubWebHook != nil && triggers[i].GitHubWebHook.SecretReference != nil {
					buildConfig.Spec.Triggers = append(triggers[:i], triggers[i+1:]...)
					updateBuildConfig = true
				}
			}
		}

		if webServer.Spec.WebImageStream.WebSources.WebhookSecrets != nil && webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Gitlab != "" {
			triggers := buildConfig.Spec.Triggers
			found := false

			for i := 0; i < len(triggers); i++ {
				if triggers[i].GitLabWebHook != nil && triggers[i].GitLabWebHook.SecretReference != nil {
					found = true

					if triggers[i].GitLabWebHook.SecretReference.Name != webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Gitlab {
						triggers[i].GitLabWebHook.SecretReference.Name = webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Gitlab
						updateBuildConfig = true
					}
				}
			}

			if !found {
				buildConfig.Spec.Triggers = append(triggers, buildv1.BuildTriggerPolicy{
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: webServer.Spec.WebImageStream.WebSources.WebhookSecrets.Gitlab,
					},
				})
				updateBuildConfig = true
			}
		} else {
			triggers := buildConfig.Spec.Triggers

			for i := 0; i < len(triggers); i++ {
				if triggers[i].GitLabWebHook != nil && triggers[i].GitLabWebHook.SecretReference != nil {
					buildConfig.Spec.Triggers = append(triggers[:i], triggers[i+1:]...)
					updateBuildConfig = true
				}
			}
		}

		if webServer.Spec.WebImageStream.WebSources.WebSourcesParams != nil {
			artifactDir := webServer.Spec.WebImageStream.WebSources.WebSourcesParams.ArtifactDir

			if artifactDir != "" {
				env := buildConfig.Spec.Strategy.SourceStrategy.Env
				found := false

				for i := 0; i < len(env); i++ {
					if env[i].Name == "ARTIFACT_DIR" {
						found = true

						if env[i].Value != artifactDir {
							env[i].Value = artifactDir
							updateBuildConfig = true
							startNewBuild = true
						}

						break
					}
				}

				if !found {
					buildConfig.Spec.Strategy.SourceStrategy.Env = append(env, corev1.EnvVar{
						Name:  "ARTIFACT_DIR",
						Value: artifactDir,
					})

					updateBuildConfig = true
					startNewBuild = true
				}
			} else {
				env := buildConfig.Spec.Strategy.SourceStrategy.Env

				for i := 0; i < len(env); i++ {
					if env[i].Name == "ARTIFACT_DIR" {
						buildConfig.Spec.Strategy.SourceStrategy.Env = append(env[:i], env[i+1:]...)
						updateBuildConfig = true
						startNewBuild = true
						break
					}
				}
			}

			mavenUrl := webServer.Spec.WebImageStream.WebSources.WebSourcesParams.MavenMirrorURL

			if mavenUrl != "" {
				env := buildConfig.Spec.Strategy.SourceStrategy.Env
				found := false

				for i := 0; i < len(env); i++ {
					if env[i].Name == "MAVEN_MIRROR_URL" {
						found = true

						if env[i].Value != mavenUrl {
							env[i].Value = mavenUrl
							updateBuildConfig = true
							startNewBuild = true
						}

						break
					}
				}

				if !found {
					buildConfig.Spec.Strategy.SourceStrategy.Env = append(env, corev1.EnvVar{
						Name:  "MAVEN_MIRROR_URL",
						Value: mavenUrl,
					})

					updateBuildConfig = true
					startNewBuild = true
				}
			} else {
				env := buildConfig.Spec.Strategy.SourceStrategy.Env

				for i := 0; i < len(env); i++ {
					if env[i].Name == "MAVEN_MIRROR_URL" {
						buildConfig.Spec.Strategy.SourceStrategy.Env = append(env[:i], env[i+1:]...)
						updateBuildConfig = true
						startNewBuild = true
						break
					}
				}
			}
		} else {
			if len(buildConfig.Spec.Strategy.SourceStrategy.Env) != 0 {
				buildConfig.Spec.Strategy.SourceStrategy.Env = []corev1.EnvVar{}
				updateBuildConfig = true
				startNewBuild = true
			}
		}

		if startNewBuild {
			buildVersion := buildConfig.Status.LastVersion + 1
			buildConfig.Status.LastVersion = buildVersion
		}

		if updateBuildConfig {
			log.Info("Update Build Config")
			err = r.Update(ctx, buildConfig)
			if err != nil {
				log.Error(err, "Failed to update BuildConfig.", "BuildConfig.Namespace", buildConfig.Namespace, "BuildConfig.Name", buildConfig.Name)
				if errors.IsConflict(err) {
					log.V(1).Info(err.Error())
				} else {
					return ctrl.Result{}, err
				}
			}
		}

		// Check if a Build has been created by the BuildConfig
		log.Info("Checking build version - " + strconv.FormatInt(buildConfig.Status.LastVersion, 10))
		buildVersion := strconv.FormatInt(buildConfig.Status.LastVersion, 10)

		build := &buildv1.Build{}
		err = r.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName + "-" + buildVersion, Namespace: webServer.Namespace}, build)

		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating new build")

			build := &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      buildConfig.Name + "-" + buildVersion,
					Namespace: buildConfig.Namespace,
					Labels: map[string]string{
						"buildconfig":                     buildConfig.Name,
						"openshift.io/build-config.name":  buildConfig.Name,
						"openshift.io/build.start-policy": "Serial",
					},
					Annotations: map[string]string{
						"openshift.io/build-config.name": buildConfig.Name,
						"openshift.io/build.number":      buildVersion,
						"openshift.io/build.pod-name":    buildConfig.Name + "-" + buildVersion + "-build",
					},
				},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildConfig.Spec.CommonSpec, // Copy the common spec from the BuildConfig
				},
			}

			err = ctrl.SetControllerReference(webServer, build, r.Scheme)
			if err != nil {
				log.Error(err, "Failed to set owner reference")
				return reconcile.Result{}, err
			}

			err = r.Create(ctx, build)
			if err != nil {
				if errors.IsAlreadyExists(err) {
					// Build already exists, do nothing
					return reconcile.Result{}, nil
				}

				log.Error(err, "Failed to create build")
				return reconcile.Result{}, err
			}
		} else if err != nil && !errors.IsNotFound(err) {
			log.Info("Failed to get the Build")
			return ctrl.Result{}, err
		}

		// If the Build was unsuccessful, stop the operator
		switch build.Status.Phase {

		case buildv1.BuildPhaseFailed:
			log.Info("Application build failed: " + build.Status.Message)
			return ctrl.Result{}, nil
		case buildv1.BuildPhaseError:
			log.Info("Application build failed: " + build.Status.Message)
			return ctrl.Result{}, nil
		case buildv1.BuildPhaseCancelled:
			log.Info("Application build canceled")
			return ctrl.Result{}, nil
		case buildv1.BuildPhaseRunning:
			log.Info("Waiting for build to be completed: requeue reconciliation")
			return ctrl.Result{Requeue: true}, nil
		case buildv1.BuildPhasePending:
			log.Info("Waiting for build to be completed: requeue reconciliation")
			return ctrl.Result{Requeue: true}, nil
		case buildv1.BuildPhaseNew:
			log.Info("Waiting for build to be completed: requeue reconciliation")
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		buildConfig := &buildv1.BuildConfig{}

		err = r.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, buildConfig)

		if err == nil {
			err = r.Delete(ctx, buildConfig)

			if err != nil {
				log.Error(err, "BuildConfig was not properly deleted")
			}
		}
	}

	is := &imagestreamv1.ImageStream{}

	err = r.Get(ctx, client.ObjectKey{
		Namespace: imageStreamNamespace,
		Name:      imageStreamName,
	}, is)

	if errors.IsNotFound(err) {
		log.Error(err, "Namespace/ImageStream doesn't exist.")
		return ctrl.Result{}, nil
	}
	dockerImageRepository := is.Status.DockerImageRepository
	log.Info("Using " + dockerImageRepository + " as applicationImage")

	if webServer.Spec.Volume != nil && len(webServer.Spec.Volume.VolumeClaimTemplates) > 0 {
		return r.continueWithStatefulSet(ctx, webServer, dockerImageRepository)
	} else {
		return r.continueWithDeployment(ctx, webServer, dockerImageRepository)
	}
}

func (r *WebServerReconciler) useSessionClusteringConfig(ctx context.Context, webServer *webserversv1alpha1.WebServer) (ctrl.Result, error) {
	result := ctrl.Result{}
	var err error = nil

	if r.needgetUseKUBEPing(webServer) {
		// Check if a RoleBinding for the KUBEPing exists, and if not create one.
		rolename := "view-kubeping-" + webServer.Name
		rolebinding := r.generateRoleBinding(webServer, rolename)
		//
		// The example in docs seems to use view (name: view and roleRef ClusterRole/view for our ServiceAccount)
		// like:
		// oc policy add-role-to-user view system:serviceaccount:tomcat-in-the-cloud:default -n tomcat-in-the-cloud
		useKUBEPing, update, err := r.createRoleBinding(ctx, webServer, rolebinding, rolename, rolebinding.Namespace)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !useKUBEPing {
			// Update the webServer annotation to prevent retrying
			log.Info("Won't use KUBEPing missing view permissions")
		} else {
			log.Info("Will use KUBEPing")
			if update {
				// We have created the Role Binding
				log.Info("We have created the Role Binding")
				return ctrl.Result{Requeue: update}, nil
			}
		}
		update, err = r.setUseKUBEPing(ctx, webServer, useKUBEPing)
		if err != nil {
			log.Error(err, "Failed to add a new Annotations")
			return reconcile.Result{}, err
		} else {
			if update {
				log.Info("Add a new Annotations or need UPDATE")
				return ctrl.Result{Requeue: update}, nil
			}
		}
	}
	if !r.getUseKUBEPing(webServer) {

		// Check if a Service for DNSPing already exists, and if not create a new one
		dnsService := r.generateServiceForDNS(webServer)
		result, err = r.createService(ctx, dnsService, dnsService.Name, dnsService.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			return result, err
		}
	}

	return result, err
}

func (r *WebServerReconciler) createService(ctx context.Context, resource *corev1.Service, resourceName, resourceNamespace string) (ctrl.Result, error) {
	err := r.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Service: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
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
	err := r.List(ctx, roleBindingList, listOpts...)
	if err != nil {
		log.Error(err, "checkRoleBinding")
		return false
	}
	var roleBindings = roleBindingList.Items
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
//
//nolint:unparam
func (r *WebServerReconciler) createRoleBinding(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *rbac.RoleBinding, resourceName string, resourceNamespace string) (bool, bool, error) {
	// First try to check if there is a roleBinding that allows KUBEPing
	checked := r.checkRoleBinding(ctx, webServer)
	if checked {
		return true, false, nil
	}

	// Then try to create it.
	err := r.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new RoleBinding: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			if errors.IsForbidden(err) {
				log.Info("No permission to create a new RoleBinding: " + resourceName + " Namespace: " + resourceNamespace)
				return false, false, nil
			} else {
				log.Error(err, "Failed to create a new RoleBinding: "+resourceName+" Namespace: "+resourceNamespace)
				return false, false, err
			}
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

func (r *WebServerReconciler) createConfigMap(ctx context.Context, resource *corev1.ConfigMap, resourceName, resourceNamespace string) (ctrl.Result, error) {
	err := r.Update(ctx, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new ConfigMap: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
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

	log.Info("ConfigMap updated")
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createPersistentVolumeClaim(ctx context.Context, resource *corev1.PersistentVolumeClaim, resourceName, resourceNamespace string) (ctrl.Result, error) {
	err := r.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new PersistentVolumeClaim: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new PersistentVolumeClaim: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get PersistentVolumeClaim "+resourceName)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createBuildPod(ctx context.Context, resource *corev1.Pod, resourceName, resourceNamespace string) (ctrl.Result, error) {
	err := r.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Build Pod: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new Pod: "+resourceName+" Namespace: "+resourceNamespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		log.Info("Created new Build Pod: " + resourceName + " Namespace: " + resourceNamespace)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Build Pod: "+resourceName)
		return reconcile.Result{}, err
	}
	log.Info("Have Build Pod: " + resourceName + " Namespace: " + resourceNamespace)
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createDeployment(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *kbappsv1.Deployment, resourceName string, resourceNamespace string) (ctrl.Result, error) {
	err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: resourceNamespace}, resource)

	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Deployment: " + resourceName + " Namespace: " + resourceNamespace)
		resource.Labels["webserver-hash"] = r.getWebServerHash(webServer)
		err = r.Create(ctx, resource)
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

func (r *WebServerReconciler) createStatefulSet(ctx context.Context, webServer *webserversv1alpha1.WebServer, resource *kbappsv1.StatefulSet) (ctrl.Result, error) {
	name := resource.Name
	namespace := resource.Namespace

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, resource)

	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new StatefulSet: " + name + " Namespace: " + namespace)
		resource.Labels["webserver-hash"] = r.getWebServerHash(webServer)
		err = r.Create(ctx, resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create a new StatefulSet: "+name+" Namespace: "+namespace)
			return reconcile.Result{}, err
		}
		// Resource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get StatefulSet: "+name)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func (r *WebServerReconciler) createImageStream(ctx context.Context, resource *imagestreamv1.ImageStream, resourceName, resourceNamespace string) (ctrl.Result, error) {
	err := r.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new ImageStream: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
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

func (r *WebServerReconciler) createBuildConfig(ctx context.Context, resource *buildv1.BuildConfig, resourceName, resourceNamespace string) (ctrl.Result, error) {
	err := r.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new BuildConfig: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
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

func (r *WebServerReconciler) createRoute(ctx context.Context, resource *routev1.Route, resourceName, resourceNamespace string) (ctrl.Result, error) {
	err := r.Get(ctx, client.ObjectKey{
		Namespace: resourceNamespace,
		Name:      resourceName,
	}, resource)
	if err != nil && errors.IsNotFound(err) {
		// Create a new resource
		log.Info("Creating a new Route: " + resourceName + " Namespace: " + resourceNamespace)
		err = r.Create(ctx, resource)
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

func (r *WebServerReconciler) checkBuildPodPhase(buildPod *corev1.Pod) reconcile.Result {
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
		return reconcile.Result{RequeueAfter: (5 * time.Second)}
	}
	return reconcile.Result{}
}

// getPodList lists pods which belongs to the Web server
// the pods are differentiated based on the selectors
func (r *WebServerReconciler) getPodList(ctx context.Context, webServer *webserversv1alpha1.WebServer) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.InNamespace(webServer.Namespace),
		client.MatchingLabels(r.generateLabelsForWeb(webServer)),
	}
	err := r.List(ctx, podList, listOpts...)

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
		"WebServer":   webServer.Name,
		"application": webServer.Spec.ApplicationName,
		// app.kubernetes.io/name is used for HPA selector like in wildfly
		"app.kubernetes.io/name": webServer.Name,
	}

	if webServer.Spec.Volume != nil && len(webServer.Spec.Volume.VolumeClaimTemplates) > 0 {
		labels["statefulset"] = webServer.Spec.ApplicationName
	} else {
		labels["deployment"] = webServer.Spec.ApplicationName
	}

	// Those are from the wildfly operator (in their Dockerfile)
	// labels["app.kubernetes.io/managed-by"] = os.Getenv("LABEL_APP_MANAGED_BY")
	// labels["app.openshift.io/runtime"] = os.Getenv("LABEL_APP_RUNTIME")
	if webServer.Labels != nil {
		for labelKey, labelValue := range webServer.Labels {
			labels[labelKey] = labelValue
		}
	}
	return labels
}

// generateSelectorLabelsForWeb return a map of labels that are used for identification
// of objects belonging to the particular WebServer instance
// NOTE: that is ONLY for the Selector of the Deployment
// the other labels might change and that is NOT allowed when updating a Deployment
func (r *WebServerReconciler) generateSelectorLabelsForWeb(webServer *webserversv1alpha1.WebServer) map[string]string {
	labels := map[string]string{
		"WebServer":   webServer.Name,
		"application": webServer.Spec.ApplicationName,
		// app.kubernetes.io/name is used for HPA selector like in wildfly
		"app.kubernetes.io/name": webServer.Name,
	}

	if webServer.Spec.Volume != nil && len(webServer.Spec.Volume.VolumeClaimTemplates) > 0 {
		labels["statefulset"] = webServer.Spec.ApplicationName
	} else {
		labels["deployment"] = webServer.Spec.ApplicationName
	}

	return labels
}

// sortPodListByName sorts the pod list by number in the name
//
//	expecting the format which the StatefulSet works with which is `<podname>-<number>`
func (r *WebServerReconciler) sortPodListByName(podList *corev1.PodList) *corev1.PodList {
	sort.SliceStable(podList.Items, func(i, j int) bool {
		return podList.Items[i].Name < podList.Items[j].Name
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

// Calculate a hash of the Spec (configuration) to redeploy/rebuild if needed.
func (r *WebServerReconciler) getWebServerHash(webServer *webserversv1alpha1.WebServer) string {
	h := sha256.New()
	h.Write([]byte("ApplicationName:" + webServer.Spec.ApplicationName))
	// No need to recreate h.Write([]byte("Replicas:" + fmt.Sprint(webServer.Spec.Replicas)))
	h.Write([]byte("UseSessionClustering:" + fmt.Sprint(webServer.Spec.UseSessionClustering)))
	h.Write([]byte("UseInsightsClient:" + fmt.Sprint(webServer.Spec.UseInsightsClient)))
	h.Write([]byte("IsNotJWS:" + fmt.Sprint(webServer.Spec.IsNotJWS)))

	/* add the labels */
	if webServer.Labels != nil {
		keys := make([]string, len(webServer.Labels))
		i := 0
		for k := range webServer.Labels {
			keys[i] = k
			i++
		}
		sort.Strings(keys)

		// To perform the opertion you want
		for _, k := range keys {
			h.Write([]byte(k + ":" + webServer.Labels[k]))
		}

	}

	data, err := json.Marshal(webServer.Spec.WebImage)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - WebImage")
		return ""
	}
	h.Write(data)

	data, err = json.Marshal(webServer.Spec.WebImageStream)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - WebImage")
		return ""
	}
	h.Write(data)

	data, err = json.Marshal(webServer.Spec.TLSConfig)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - TLSConfig")
		return ""
	}
	h.Write(data)

	data, err = json.Marshal(webServer.Spec.EnvironmentVariables)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - EnvironmentVariables")
		return ""
	}
	h.Write(data)

	data, err = json.Marshal(webServer.Spec.PersistentLogsConfig)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - PersistentLogsConfig")
		return ""
	}
	h.Write(data)

	data, err = json.Marshal(webServer.Spec.PodResources)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - PodResources")
		return ""
	}
	h.Write(data)

	data, err = json.Marshal(webServer.Spec.SecurityContext)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - SecurityContext")
		return ""
	}
	h.Write(data)

	data, err = json.Marshal(webServer.Spec.Volume)
	if err != nil {
		log.Error(err, "WebServer hash sum calculation failed - Volume")
		return ""
	}
	h.Write(data)

	/* rules for labels: '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"} */
	enc := base64.NewEncoding("qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM_.0123456789")
	enc = enc.WithPadding(base64.NoPadding)
	return "A" + enc.EncodeToString(h.Sum(nil)) + "A"
}

// Calculate a hash of the Spec (configuration) to redeploy/rebuild if needed.
func (r *WebServerReconciler) getReplicaStatus(ctx context.Context, webServer *webserversv1alpha1.WebServer) int32 {
	if webServer.Spec.Volume != nil && len(webServer.Spec.Volume.VolumeClaimTemplates) > 0 {
		statefulset := &kbappsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, statefulset)

		if err != nil {
			log.Error(err, "Failed to get StatefulSet: "+webServer.Spec.ApplicationName)
			return 0
		}

		return statefulset.Status.Replicas
	} else {
		deployment := &kbappsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, deployment)
		if err != nil {
			log.Error(err, "Failed to get Deployment: "+webServer.Spec.ApplicationName)
			return 0
		}

		return deployment.Status.Replicas
	}
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
		err := r.Update(ctx, webServer)
		if err != nil {
			if errors.IsConflict(err) {
				log.Info("setUseKUBEPing needs webServer UPDATE!!!")
				return true, nil
			} else {
				log.Error(err, "Failed to update WebServer UseKUBEPing annotation")
			}
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
		log.Info("needgetUseKUBEPing: annotations")
		skubeping := annotations["UseKUBEPing"]
		if skubeping != "" {
			log.Info("needgetUseKUBEPing: annotations skubeping")
			return false
		} else {
			log.Info("needgetUseKUBEPing: annotations NO skubeping")
		}
	}
	return true
}

// CustomResourceDefinitionExists returns true if the CRD exists in the cluster
func CustomResourceDefinitionExists(gvk schema.GroupVersionKind, c *rest.Config) bool {

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		return false
	}
	api, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
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

func (r *WebServerReconciler) checkOwnedObjects(ctx context.Context, webServer *webserversv1alpha1.WebServer) error {
	log.Info("CheckOwnedObjects")

	ownedDeployments := &kbappsv1.DeploymentList{}
	err := r.List(ctx, ownedDeployments,
		client.InNamespace(webServer.Namespace),
		client.MatchingFields{ownerUIDIndex: string(webServer.GetUID())},
	)
	if err != nil {
		log.Error(err, "unable to list owned Deployments")
		return err
	}

	ownedStatefulSets := &kbappsv1.StatefulSetList{}
	err = r.List(ctx, ownedStatefulSets,
		client.InNamespace(webServer.Namespace),
		client.MatchingFields{ownerUIDIndex: string(webServer.GetUID())},
	)
	if err != nil {
		log.Error(err, "unable to list owned StatefulSets")
		return err
	}

	if webServer.Spec.Volume != nil && len(webServer.Spec.Volume.VolumeClaimTemplates) > 0 {
		log.Info("CheckOwnedObjects: statefulset case")
		for _, deployment := range ownedDeployments.Items {
			err = r.Delete(ctx, &deployment)
			if err != nil {
				log.Error(err, "Failed to delete Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
				return err
			}
		}
		for _, statefulset := range ownedStatefulSets.Items {
			if statefulset.Name != webServer.Spec.ApplicationName {
				err = r.Delete(ctx, &statefulset)
				if err != nil {
					log.Error(err, "Failed to delete StatefulSet.", "StatefulSet.Namespace", statefulset.Namespace, "StatefulSet.Name", statefulset.Name)
					return err
				}
			}
		}
	} else {
		log.Info("CheckOwnedObjects: deployment case")
		for _, statefulset := range ownedStatefulSets.Items {
			err = r.Delete(ctx, &statefulset)
			if err != nil {
				log.Error(err, "Failed to delete StatefulSet.", "StatefulSet.Namespace", statefulset.Namespace, "StatefulSet.Name", statefulset.Name)
				return err
			}
		}
		for _, deployment := range ownedDeployments.Items {
			if deployment.Name != webServer.Spec.ApplicationName {
				err = r.Delete(ctx, &deployment)
				if err != nil {
					log.Error(err, "Failed to delete Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
					return err
				}
			}
		}
	}

	return nil
}
