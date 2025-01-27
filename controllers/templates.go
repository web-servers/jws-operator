package controllers

import (
	"os"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"

	kbappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *WebServerReconciler) generateObjectMeta(webServer *webserversv1alpha1.WebServer, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: webServer.Namespace,
	}
}

func (r *WebServerReconciler) generateRoutingService(webServer *webserversv1alpha1.WebServer, port int) *corev1.Service {

	service := &corev1.Service{
		ObjectMeta: r.generateObjectMeta(webServer, webServer.Spec.ApplicationName),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "ui",
				Port:       int32(port),
				TargetPort: intstr.FromInt(port),
			}},
			// Don't forget to check generateLabelsForWeb before changing this...
			// there are more Labels but we only use those for the Route.
			Selector: map[string]string{
				"deploymentConfig": webServer.Spec.ApplicationName,
				"WebServer":        webServer.Name,
			},
		},
	}
	controllerutil.SetControllerReference(webServer, service, r.Scheme)
	return service

}

// Create something like:
// oc policy add-role-to-user view system:serviceaccount:tomcat-in-the-cloud:default -n tomcat-in-the-cloud
// does:
// apiVersion: rbac.authorization.k8s.io/v1
// kind: RoleBinding
// metadata:
//   name: view
//   namespace: tomcat-in-the-cloud
// roleRef:
//   apiGroup: rbac.authorization.k8s.io
//   kind: ClusterRole
//   name: view
// subjects:
// - kind: ServiceAccount
//   name: default
//   namespace: tomcat-in-the-cloud

func (r *WebServerReconciler) generateRoleBinding(webServer *webserversv1alpha1.WebServer, rolename string) *rbac.RoleBinding {
	rolebinding := &rbac.RoleBinding{
		ObjectMeta: r.generateObjectMeta(webServer, rolename),
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
		Subjects: []rbac.Subject{{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: webServer.Namespace,
		}},
	}

	controllerutil.SetControllerReference(webServer, rolebinding, r.Scheme)
	return rolebinding
}

func (r *WebServerReconciler) generateServiceForDNS(webServer *webserversv1alpha1.WebServer) *corev1.Service {

	service := &corev1.Service{
		ObjectMeta: r.generateObjectMeta(webServer, "webserver-"+webServer.Name),
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"application": webServer.Spec.ApplicationName,
			},
		},
	}

	controllerutil.SetControllerReference(webServer, service, r.Scheme)
	return service
}

// Script for the cluster and tls in server.xml
func (r *WebServerReconciler) generateConfigMapForDNSTLS(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		ObjectMeta: r.generateObjectMeta(webServer, "webserver-"+webServer.Name),
		Data:       r.generateCommandForServerXml(webServer),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

// Script for asf images that misses start.sh
func (r *WebServerReconciler) generateConfigMapForASFStart(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		ObjectMeta: r.generateObjectMeta(webServer, "start-sh-webserver-"+webServer.Name),
		Data:       r.generateCommandForASFStart(webServer),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

func (r *WebServerReconciler) generateConfigMapForReadinessProbe(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		ObjectMeta: r.generateObjectMeta(webServer, "readinessprobescript-sh-webserver-"+webServer.Name),
		Data:       r.generateReadinessProbeScript(webServer),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

func (r *WebServerReconciler) generateConfigMapForLivenessProbe(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		ObjectMeta: r.generateObjectMeta(webServer, "livenessprobescript-sh-webserver-"+webServer.Name),
		Data:       r.generateLivenessProbeScript(webServer),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

// logging.properties for saving logs to catalina.out inside the pod
func (r *WebServerReconciler) generateConfigMapForLoggingProperties(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		ObjectMeta: r.generateObjectMeta(webServer, "config-volume"),
		Data:       r.generateLoggingProperties(webServer),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

// configMap for Prometheus
func (r *WebServerReconciler) generateConfigMapForPrometheus(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-monitoring-config",
			Namespace: "openshift-monitoring",
		},
	}
	//tricky workaround to make the yaml as needed
	value := map[string]interface{}{
		"enableUserWorkload": true,
	}
	marshaled, _ := yaml.Marshal(value)
	cmap.Data = map[string]string{
		"config.yaml": string(marshaled),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

// pvc for saving logs
func (r *WebServerReconciler) generatePersistentVolumeClaimForLogging(webServer *webserversv1alpha1.WebServer) *corev1.PersistentVolumeClaim {

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: r.generateObjectMeta(webServer, "volume-pvc-"+webServer.Name),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, //works only if you remove "default" from StorageClass
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("1Gi"),
				},
			},
		},
	}

	if webServer.Spec.PersistentLogsConfig.VolumeName != "" {
		pvc.Spec.VolumeName = webServer.Spec.PersistentLogsConfig.VolumeName
	}

	if webServer.Spec.PersistentLogsConfig.StorageClass != "" {
		pvc.Spec.StorageClassName = &webServer.Spec.PersistentLogsConfig.StorageClass
	}

	controllerutil.SetControllerReference(webServer, pvc, r.Scheme)
	return pvc
}

// Custom build script for the pod builder
func (r *WebServerReconciler) generateConfigMapForCustomBuildScript(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		ObjectMeta: r.generateObjectMeta(webServer, "webserver-bd-"+webServer.Name),
		Data:       r.generateCommandForBuider(webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

func (r *WebServerReconciler) generateBuildPod(webServer *webserversv1alpha1.WebServer) *corev1.Pod {
	command := []string{}
	args := []string{}
	if webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript != "" {
		command = []string{"/bin/sh"}
		args = []string{"/build/my-files/build.sh"}
	}
	name := webServer.Spec.ApplicationName + "-build"
	objectMeta := r.generateObjectMeta(webServer, name)
	// Don't use r.generateLabelsForWeb(webServer) here, that is ONLY for applicaion pods.
	objectMeta.Labels = map[string]string{
		"webserver-hash": r.getWebServerHash(webServer),
	}
	terminationGracePeriodSeconds := int64(60)
	serviceAccountName := ""
	var securityContext *corev1.SecurityContext
	if r.isOpenShift {
		// RunAsUser must correspond to the USER in the docker image.
		serviceAccountName = "builder"
		securityContext = &corev1.SecurityContext{
			RunAsUser: &[]int64{1000}[0],
			/*
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{
						// "CAP_SETGID", "CAP_SETUID",
						// "SETGID", "SETUID", "SYS_ADMIN", "SYS_CHROOT",
						"SYS_ADMIN", "SYS_CHROOT",
					},
				},
			*/
		}
	} else {
		securityContext = &corev1.SecurityContext{
			Privileged: &[]bool{true}[0],
		}
	}
	pod := &corev1.Pod{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			RestartPolicy:                 "OnFailure",
			Volumes:                       r.generateVolumePodBuilder(webServer),
			/* from openshift BuildConfig: Use ServiceAccountName: "builder", */
			ServiceAccountName: serviceAccountName,
			/* secret to pull the image */
			ImagePullSecrets: r.generateimagePullSecrets(webServer),
			/* Problems: SeccompProfileTypeUnconfined, SeccompProfileTypeLocalhost */
			/*
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			*/
			Containers: []corev1.Container{
				{
					Name:  "war",
					Image: webServer.Spec.WebImage.WebApp.Builder.Image,
					// Default uses the default build.sh file in image
					Command: command,
					Args:    args,
					// Actually the SA doesn't have that permission :( so that won't work with giving permissions.
					// Doing the following allows it:
					// oc adm policy add-scc-to-group privileged system:serviceaccounts:tomcat-in-the-cloud
					/*
						SecurityContext: &corev1.SecurityContext{
							Privileged: &[]bool{true}[0],
						},
					*/
					// here the permissions have to be added in a SecurityContextConstraint
					// for example https://github.com/jfclere/tomcat-kubernetes/blob/main/scc.yaml
					// kubectl create -f scc.yaml
					// oc adm policy add-scc-to-group scc-jws system:serviceaccounts:tomcat-in-the-cloud
					/*
						SecurityContext: &corev1.SecurityContext{
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									// "CAP_SETGID", "CAP_SETUID",
									"SETGID", "SETUID",
								},
							},
						},
					*/
					SecurityContext: securityContext,
					Env:             r.generateEnvBuild(webServer),
					VolumeMounts:    r.generateVolumeMountPodBuilder(webServer),
				},
			},
		},
	}

	controllerutil.SetControllerReference(webServer, pod, r.Scheme)
	return pod
}

func (r *WebServerReconciler) generateAnnotationsDeployment(webServer *webserversv1alpha1.WebServer) map[string]string {
	ann := make(map[string]string)
	ann["image.openshift.io/triggers"] = "[{\"from\": {" +
		"\"kind\":\"ImageStreamTag\"," +
		"\"name\":\"" + webServer.Spec.ApplicationName + ":latest\"" +
		"}," +
		"\"fieldPath\":\"spec.template.spec.containers[?(@.name==\\\"" + webServer.Spec.ApplicationName + "\\\")].image\"}]"
	return ann
}

func (r *WebServerReconciler) generateDeployment(webServer *webserversv1alpha1.WebServer, applicationImage string) *kbappsv1.Deployment {

	replicas := int32(webServer.Spec.Replicas)
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName)
	objectMeta.Labels = r.generateLabelsForWeb(webServer)

	if webServer.Spec.WebImage == nil {
		objectMeta.Annotations = r.generateAnnotationsDeployment(webServer)
	}

	podTemplateSpec := r.generatePodTemplate(webServer, applicationImage)
	deployment := &kbappsv1.Deployment{
		ObjectMeta: objectMeta,
		Spec: kbappsv1.DeploymentSpec{
			Strategy: kbappsv1.DeploymentStrategy{
				Type: kbappsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: r.generateSelectorLabelsForWeb(webServer),
			},
			Template: podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(webServer, deployment, r.Scheme)
	return deployment
}
func (r *WebServerReconciler) generateUpdatedDeployment(webServer *webserversv1alpha1.WebServer, deployment *kbappsv1.Deployment, applicationImage string) {

	replicas := int32(webServer.Spec.Replicas)
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName)
	objectMeta.Labels = r.generateLabelsForWeb(webServer)

	if webServer.Spec.WebImage == nil {
		objectMeta.Annotations = r.generateAnnotationsDeployment(webServer)
	}

	log.Info("generateUpdatedDeployment")
	podTemplateSpec := r.generatePodTemplate(webServer, applicationImage)
	deployment.ObjectMeta = objectMeta
	spec := kbappsv1.DeploymentSpec{
		Strategy: kbappsv1.DeploymentStrategy{
			Type: kbappsv1.RecreateDeploymentStrategyType,
		},
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: r.generateSelectorLabelsForWeb(webServer),
		},
		Template: podTemplateSpec,
	}
	deployment.Spec = spec
	controllerutil.SetControllerReference(webServer, deployment, r.Scheme)
}

func (r *WebServerReconciler) generateImageStream(webServer *webserversv1alpha1.WebServer) *imagev1.ImageStream {

	imageStream := &imagev1.ImageStream{
		ObjectMeta: r.generateObjectMeta(webServer, webServer.Spec.ApplicationName),
	}

	controllerutil.SetControllerReference(webServer, imageStream, r.Scheme)
	return imageStream
}

func (r *WebServerReconciler) generateBuildConfig(webServer *webserversv1alpha1.WebServer) *buildv1.BuildConfig {

	buildConfig := &buildv1.BuildConfig{
		ObjectMeta: r.generateObjectMeta(webServer, webServer.Spec.ApplicationName),
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Type: "Git",
					Git: &buildv1.GitBuildSource{
						URI: webServer.Spec.WebImageStream.WebSources.SourceRepositoryURL,
						Ref: webServer.Spec.WebImageStream.WebSources.SourceRepositoryRef,
					},
					ContextDir: webServer.Spec.WebImageStream.WebSources.ContextDir,
				},
				Strategy: buildv1.BuildStrategy{
					Type: "Source",
					SourceStrategy: &buildv1.SourceBuildStrategy{
						Env:       r.generateEnvBuild(webServer),
						ForcePull: true,
						From: corev1.ObjectReference{
							Kind:      "ImageStreamTag",
							Namespace: webServer.Spec.WebImageStream.ImageStreamNamespace,
							Name:      webServer.Spec.WebImageStream.ImageStreamName + ":latest",
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: webServer.Spec.ApplicationName + ":latest",
					},
				},
			},
			Triggers: r.generateBuildTriggerPolicy(webServer),
		},
	}

	controllerutil.SetControllerReference(webServer, buildConfig, r.Scheme)
	return buildConfig
}

// Create the env for the maven build and the pod builder
func (r *WebServerReconciler) generateEnvBuild(webServer *webserversv1alpha1.WebServer) []corev1.EnvVar {
	var env []corev1.EnvVar
	var sources *webserversv1alpha1.WebSourcesSpec
	var webApp *webserversv1alpha1.WebAppSpec
	if webServer.Spec.WebImageStream != nil {
		sources = webServer.Spec.WebImageStream.WebSources
	}
	// BuildConfig EnvVar
	if sources != nil {
		params := sources.WebSourcesParams
		if params != nil {
			if params.MavenMirrorURL != "" {
				env = append(env, corev1.EnvVar{
					Name:  "MAVEN_MIRROR_URL",
					Value: params.MavenMirrorURL,
				})
			}
			if params.ArtifactDir != "" {
				env = append(env, corev1.EnvVar{
					Name:  "ARTIFACT_DIR",
					Value: params.ArtifactDir,
				})
			}
		}
	}

	// pod builder EnvVar
	if webServer.Spec.WebImage != nil {
		webApp = webServer.Spec.WebImage.WebApp
	}
	if webApp != nil {
		// Name of the web application (default: ROOT.war)
		if webApp.Name != "" {
			env = append(env, corev1.EnvVar{
				Name:  "webAppWarFileName",
				Value: webApp.Name,
			})
		}
		// URL for the repository of the application sources
		if webApp.SourceRepositoryURL != "" {
			env = append(env, corev1.EnvVar{
				Name:  "webAppSourceRepositoryURL",
				Value: webApp.SourceRepositoryURL,
			})
		}
		// Branch in the source repository
		if webApp.SourceRepositoryRef != "" {
			env = append(env, corev1.EnvVar{
				Name:  "webAppSourceRepositoryRef",
				Value: webApp.SourceRepositoryRef,
			})
		}
		// Subdirectory in the source repository
		if webApp.SourceRepositoryContextDir != "" {
			env = append(env, corev1.EnvVar{
				Name:  "webAppSourceRepositoryContextDir",
				Value: webApp.SourceRepositoryContextDir,
			})
		}
		// Docker repository to push the built image
		if webApp.WebAppWarImage != "" {
			env = append(env, corev1.EnvVar{
				Name:  "webAppWarImage",
				Value: webApp.WebAppWarImage,
			})
		}
		// Docker repository to pull the base image
		env = append(env, corev1.EnvVar{
			Name:  "webAppSourceImage",
			Value: webServer.Spec.WebImage.ApplicationImage,
		})
	}
	return env
}

// Create the BuildTriggerPolicy
func (r *WebServerReconciler) generateBuildTriggerPolicy(webServer *webserversv1alpha1.WebServer) []buildv1.BuildTriggerPolicy {
	buildTriggerPolicies := []buildv1.BuildTriggerPolicy{
		{
			Type:        "ImageChange",
			ImageChange: &buildv1.ImageChangeTrigger{},
		},
		{
			Type: "ConfigChange",
		},
	}
	sources := webServer.Spec.WebImageStream.WebSources
	if sources != nil {
		params := sources.WebSourcesParams

		if sources.WebhookSecrets != nil {
			if sources.WebhookSecrets.Generic != "" {
				buildTriggerPolicies = append(buildTriggerPolicies, buildv1.BuildTriggerPolicy{
					Type: "Generic",
					GenericWebHook: &buildv1.WebHookTrigger{
						SecretReference: &buildv1.SecretLocalReference{
							Name: sources.WebhookSecrets.Generic,
						},
					},
				})
			}
			if sources.WebhookSecrets.Github != "" {
				buildTriggerPolicies = append(buildTriggerPolicies, buildv1.BuildTriggerPolicy{
					Type: "GitHub",
					GitHubWebHook: &buildv1.WebHookTrigger{
						SecretReference: &buildv1.SecretLocalReference{
							Name: sources.WebhookSecrets.Github,
						},
					},
				})
			}
			if sources.WebhookSecrets.Gitlab != "" {
				buildTriggerPolicies = append(buildTriggerPolicies, buildv1.BuildTriggerPolicy{
					Type: "GitLab",
					GitLabWebHook: &buildv1.WebHookTrigger{
						SecretReference: &buildv1.SecretLocalReference{
							Name: sources.WebhookSecrets.Gitlab,
						},
					},
				})
			}
		} else if params != nil {
			if params.GithubWebhookSecret != "" {
				buildTriggerPolicies = append(buildTriggerPolicies, buildv1.BuildTriggerPolicy{
					Type: "GitHub",
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: params.GithubWebhookSecret,
					},
				})
			}
			if params.GenericWebhookSecret != "" {
				buildTriggerPolicies = append(buildTriggerPolicies, buildv1.BuildTriggerPolicy{
					Type: "Generic",
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: params.GenericWebhookSecret,
					},
				})
			}
		}
	}
	return buildTriggerPolicies
}

func (r *WebServerReconciler) generateRoute(webServer *webserversv1alpha1.WebServer) *routev1.Route {
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName)
	objectMeta.Annotations = map[string]string{
		"description": "Route for application's http service.",
	}
	route := &routev1.Route{}
	if webServer.Spec.TLSConfig.RouteHostname == "" {
		route = &routev1.Route{
			ObjectMeta: objectMeta,
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Name: webServer.Spec.ApplicationName,
				},
			},
		}
	} else {
		route = &routev1.Route{
			ObjectMeta: objectMeta,
			Spec: routev1.RouteSpec{
				Host: webServer.Spec.TLSConfig.RouteHostname,
				To: routev1.RouteTargetReference{
					Name: webServer.Spec.ApplicationName,
				},
			},
		}
	}

	controllerutil.SetControllerReference(webServer, route, r.Scheme)
	return route
}

func (r *WebServerReconciler) generateSecureRoute(webServer *webserversv1alpha1.WebServer) *routev1.Route {
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName)
	objectMeta.Annotations = map[string]string{
		"description": "Route for application's https service.",
	}
	route := &routev1.Route{}
	if len(webServer.Spec.TLSConfig.RouteHostname) <= 3 {
		route = &routev1.Route{
			ObjectMeta: objectMeta,
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Name: webServer.Spec.ApplicationName,
				},
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationPassthrough,
				},
			},
		}
	} else {
		route = &routev1.Route{
			ObjectMeta: objectMeta,
			Spec: routev1.RouteSpec{
				Host: webServer.Spec.TLSConfig.RouteHostname[4:],
				To: routev1.RouteTargetReference{
					Name: webServer.Spec.ApplicationName,
				},
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationPassthrough,
				},
			},
		}
	}

	controllerutil.SetControllerReference(webServer, route, r.Scheme)
	return route
}

// generate loadbalancer on no openshift clusters
func (r *WebServerReconciler) generateLoadBalancer(webServer *webserversv1alpha1.WebServer) *corev1.Service {
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName+"-lb")
	objectMeta.Annotations = map[string]string{
		"description": "LoadBalancer for application's http service.",
	}
	service := &corev1.Service{
		ObjectMeta: objectMeta,
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			}},
			// Don't forget to check generateLabelsForWeb before changing this...
			// there are more Labels but we only use those for the Route.
			Selector: map[string]string{
				"deploymentConfig": webServer.Spec.ApplicationName,
				"WebServer":        webServer.Name,
			},
			Type: "LoadBalancer",
		},
	}

	controllerutil.SetControllerReference(webServer, service, r.Scheme)
	return service
}

// Note that the pod template are common to Deployment (kubernetes) and DeploymentConfig (openshift)
// be careful: the imagePullSecret uses ImagePullSecret not webAppWarImagePushSecret
func (r *WebServerReconciler) generatePodTemplate(webServer *webserversv1alpha1.WebServer, image string) corev1.PodTemplateSpec {
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName)
	objectMeta.Labels = r.generateLabelsForWeb(webServer)
	objectMeta.Labels["webserver-hash"] = r.getWebServerHash(webServer)
	var health *webserversv1alpha1.WebServerHealthCheckSpec = &webserversv1alpha1.WebServerHealthCheckSpec{}
	if webServer.Spec.WebImage != nil {
		health = webServer.Spec.WebImage.WebServerHealthCheck
	} else {
		health = webServer.Spec.WebImageStream.WebServerHealthCheck
	}
	terminationGracePeriodSeconds := int64(60)

	template := corev1.PodTemplateSpec{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{{
				Name:            webServer.Spec.ApplicationName,
				Image:           image,
				ImagePullPolicy: "Always",
				ReadinessProbe:  r.generateReadinessProbe(webServer, health),
				LivenessProbe:   r.generateLivenessProbe(webServer, health),
				Resources:       webServer.Spec.PodResources,
				Ports: []corev1.ContainerPort{{
					Name:          "jolokia",
					ContainerPort: 8778,
					Protocol:      corev1.ProtocolTCP,
				}, {
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				}, {
					Name:          "admin",
					ContainerPort: 9404,
					Protocol:      corev1.ProtocolTCP,
				}, {
					Name:          "https",
					ContainerPort: 8443,
					Protocol:      corev1.ProtocolTCP,
				}},
				SecurityContext: generateSecurityContext(webServer.Spec.SecurityContext),
				Env:             r.generateEnvVars(webServer),
				VolumeMounts:    r.generateVolumeMounts(webServer),
			}},
			Volumes: r.generateVolumes(webServer),
			// Add the imagePullSecret to imagePullSecrets
			ImagePullSecrets: r.generateimagePullSecrets(webServer),
		},
	}
	if webServer.Spec.IsNotJWS {
		template.Spec.Containers[0].Command = append(template.Spec.Containers[0].Command, "/bin/sh")
		template.Spec.Containers[0].Args = append(template.Spec.Containers[0].Args, "-c", "/opt/start/start.sh")
	}
	// if the user specified the resources directive propagate it to the container (required for HPA).
	template.Spec.Containers[0].Resources = webServer.Spec.PodResources
	return template
}

// generateimagePullSecrets
func (r *WebServerReconciler) generateimagePullSecrets(webServer *webserversv1alpha1.WebServer) []corev1.LocalObjectReference {
	if webServer.Spec.WebImage != nil && webServer.Spec.WebImage.ImagePullSecret != "" {
		imgps := make([]corev1.LocalObjectReference, 0)
		imgps = append(imgps, corev1.LocalObjectReference{Name: webServer.Spec.WebImage.ImagePullSecret})
		return imgps
	}
	return nil
}

// generateLivenessProbe returns a custom probe if the serverLivenessScript string is defined and not empty in the Custom Resource.
// Otherwise, it uses the default /health Valve via curl.
//
// If defined, serverLivenessScript must be a shell script that
// complies to the Kubernetes probes requirements and use the following format
// shell -c "command"
func (r *WebServerReconciler) generateLivenessProbe(webServer *webserversv1alpha1.WebServer, health *webserversv1alpha1.WebServerHealthCheckSpec) *corev1.Probe {
	livenessProbeScript := ""
	if health != nil {
		livenessProbeScript = health.ServerLivenessScript
	}
	if livenessProbeScript != "" {
		return r.generateCustomProbe(webServer, "livenessProbeScript")
	} else {
		/* Use the default one */
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(8080),
				},
			},
		}
	}
}

// generateReadinessProbe returns a custom probe if the serverReadinessScript string is defined and not empty in the Custom Resource.
// Otherwise, it uses the default /health Valve via curl.
//
// If defined, serverReadinessScript must be a shell script that
// complies to the Kubernetes probes requirements and use the following format
// shell -c "command"
func (r *WebServerReconciler) generateReadinessProbe(webServer *webserversv1alpha1.WebServer, health *webserversv1alpha1.WebServerHealthCheckSpec) *corev1.Probe {
	readinessProbeScript := ""
	if health != nil {
		readinessProbeScript = health.ServerReadinessScript
	}
	if readinessProbeScript != "" {
		return r.generateCustomProbe(webServer, "readinessProbeScript")
	} else {
		/* Use the default one */
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(8080),
				},
			},
		}
	}
}

func (r *WebServerReconciler) generateCustomProbe(webServer *webserversv1alpha1.WebServer, probeType string) *corev1.Probe {
	// If the script has the following format: shell -c "command"
	// we create the slice ["shell", "-c", "command"]
	probeScriptSlice := []string{"/bin/bash", "/opt/probe/" + probeType + ".sh"}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: probeScriptSlice,
			},
		},
	}
}

// Create the env for the pods we are starting.
func (r *WebServerReconciler) generateEnvVars(webServer *webserversv1alpha1.WebServer) []corev1.EnvVar {
	value := "webserver-" + webServer.Name
	if r.getUseKUBEPing(webServer) && webServer.Spec.UseSessionClustering {
		value = webServer.Namespace
	}
	env := []corev1.EnvVar{
		{
			Name:  "KUBERNETES_NAMESPACE",
			Value: value,
		},
	}
	if webServer.Spec.PersistentLogsConfig.AccessLogs {
		env = append(env, corev1.EnvVar{
			Name:  "ENABLE_ACCESS_LOG",
			Value: "true",
		})
	}
	if strings.HasPrefix(webServer.Spec.TLSConfig.RouteHostname, "tls") || webServer.Spec.UseSessionClustering || webServer.Spec.PersistentLogsConfig.AccessLogs {
		env = append(env, corev1.EnvVar{
			Name:  "ENV_FILES",
			Value: "/env/my-files/test.sh",
		})
	}
	if webServer.Spec.PersistentLogsConfig.CatalinaLogs {
		//custum logging.properties path
		env = append(env, corev1.EnvVar{
			Name:  "CATALINA_LOGGING_CONFIG",
			Value: "-Djava.util.logging.config.file=/opt/operator_conf/logging.properties",
		})
	}

	env = append(env, webServer.Spec.EnvironmentVariables...)

	if webServer.Spec.UseInsightsClient {
		javaToolOptions := " -javaagent:/opt/runtimes-agent.jar=name=" + webServer.Spec.ApplicationName
		javaToolOptions = javaToolOptions + ";is_ocp=true;token=dummy;debug=true;base_url="
		javaToolOptions = javaToolOptions + "http://insights-proxy." + os.Getenv("OPERATOR_NAMESPACE") + ".svc.cluster.local:8080"
		updated := false

		for i := 0; i < len(env); i++ {
			if env[i].Name == "JAVA_TOOL_OPTIONS" {
				env[i].Value = env[i].Value + javaToolOptions
				updated = true
			}
		}
		if !updated {
			env = append(env, corev1.EnvVar{
				Name:  "JAVA_TOOL_OPTIONS",
				Value: javaToolOptions,
			})
		}
	}

	return env
}

// Create the VolumeMounts
func (r *WebServerReconciler) generateVolumeMounts(webServer *webserversv1alpha1.WebServer) []corev1.VolumeMount {
	var volm []corev1.VolumeMount

	if webServer.Spec.PersistentLogsConfig.CatalinaLogs || webServer.Spec.PersistentLogsConfig.AccessLogs {
		volm = append(volm, corev1.VolumeMount{
			Name:      "config-volume",
			MountPath: "/opt/operator_conf/logging.properties",
			SubPath:   "logging.properties",
		})
		volm = append(volm, corev1.VolumeMount{
			Name:      "volume-pvc-" + webServer.Name,
			MountPath: "/opt/tomcat_logs",
		})
	}

	if strings.HasPrefix(webServer.Spec.TLSConfig.RouteHostname, "tls") || webServer.Spec.UseSessionClustering || webServer.Spec.PersistentLogsConfig.AccessLogs {
		volm = append(volm, corev1.VolumeMount{
			Name:      "webserver-" + webServer.Name,
			MountPath: "/env/my-files",
		})
	}

	if webServer.Spec.IsNotJWS {
		volm = append(volm, corev1.VolumeMount{
			Name:      "start-sh-webserver-" + webServer.Name,
			MountPath: "/opt/start",
		})
	}

	var health *webserversv1alpha1.WebServerHealthCheckSpec = &webserversv1alpha1.WebServerHealthCheckSpec{}
	if webServer.Spec.WebImage != nil {
		health = webServer.Spec.WebImage.WebServerHealthCheck
	} else {
		health = webServer.Spec.WebImageStream.WebServerHealthCheck
	}
	if health != nil {
		if health.ServerLivenessScript != "" {
			volm = append(volm, corev1.VolumeMount{
				Name:      "livenessprobescript-sh-webserver-" + webServer.Name,
				MountPath: "/opt/probe/livenessProbeScript.sh",
				SubPath:   "livenessProbeScript.sh",
			})
		}
		if health.ServerReadinessScript != "" {
			volm = append(volm, corev1.VolumeMount{
				Name:      "readinessprobescript-sh-webserver-" + webServer.Name,
				MountPath: "/opt/probe/readinessProbeScript.sh",
				SubPath:   "readinessProbeScript.sh",
			})
		}
	}

	if webServer.Spec.TLSConfig.TLSSecret != "" {
		volm = append(volm, corev1.VolumeMount{
			Name:      "webserver-tls" + webServer.Name,
			MountPath: "/tls",
			ReadOnly:  true,
		})
	}

	return volm
}

// Create the Volumes
func (r *WebServerReconciler) generateVolumes(webServer *webserversv1alpha1.WebServer) []corev1.Volume {
	var vol []corev1.Volume
	if webServer.Spec.PersistentLogsConfig.CatalinaLogs || webServer.Spec.PersistentLogsConfig.AccessLogs {
		vol = append(vol, corev1.Volume{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "config-volume",
					},
				},
			},
		})

		vol = append(vol, corev1.Volume{
			Name: "volume-pvc-" + webServer.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "volume-pvc-" + webServer.Name,
				},
			},
		})
	}

	if strings.HasPrefix(webServer.Spec.TLSConfig.RouteHostname, "tls") || webServer.Spec.UseSessionClustering || webServer.Spec.PersistentLogsConfig.AccessLogs {
		vol = append(vol, corev1.Volume{
			Name: "webserver-" + webServer.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "webserver-" + webServer.Name,
					},
				},
			},
		})

	}

	if webServer.Spec.IsNotJWS {
		executeMode := int32(0777)
		vol = append(vol, corev1.Volume{
			Name: "start-sh-webserver-" + webServer.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "start-sh-webserver-" + webServer.Name,
					},
					DefaultMode: &executeMode,
				},
			},
		})
	}

	var health *webserversv1alpha1.WebServerHealthCheckSpec = &webserversv1alpha1.WebServerHealthCheckSpec{}
	if webServer.Spec.WebImage != nil {
		health = webServer.Spec.WebImage.WebServerHealthCheck
	} else {
		health = webServer.Spec.WebImageStream.WebServerHealthCheck
	}
	if health != nil {
		if health.ServerReadinessScript != "" {
			executeMode := int32(0777)
			vol = append(vol, corev1.Volume{
				Name: "readinessprobescript-sh-webserver-" + webServer.Name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "readinessprobescript-sh-webserver-" + webServer.Name,
						},
						DefaultMode: &executeMode,
					},
				},
			})
		}
		if health.ServerLivenessScript != "" {
			executeMode := int32(0777)
			vol = append(vol, corev1.Volume{
				Name: "livenessprobescript-sh-webserver-" + webServer.Name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "livenessprobescript-sh-webserver-" + webServer.Name,
						},
						DefaultMode: &executeMode,
					},
				},
			})
		}
	}

	if webServer.Spec.TLSConfig.TLSSecret != "" {
		vol = append(vol, corev1.Volume{
			Name: "webserver-tls" + webServer.Name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: webServer.Spec.TLSConfig.TLSSecret,
				},
			},
		})
	}
	return vol
}

// Create the VolumeMount for the pod builder
func (r *WebServerReconciler) generateVolumeMountPodBuilder(webServer *webserversv1alpha1.WebServer) []corev1.VolumeMount {
	volm := []corev1.VolumeMount{{
		Name:      "app-volume",
		MountPath: "/auth",
		ReadOnly:  true,
	}}
	if webServer.Spec.WebImage != nil && webServer.Spec.WebImage.WebApp != nil && webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript != "" {
		volm = append(volm, corev1.VolumeMount{
			Name:      "webserver-bd-" + webServer.Name,
			MountPath: "/build/my-files",
		})
	}

	return volm
}

// create volums for secret and custom script builder
func (r *WebServerReconciler) generateVolumePodBuilder(webServer *webserversv1alpha1.WebServer) []corev1.Volume {
	vol := []corev1.Volume{{
		Name: "app-volume",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: webServer.Spec.WebImage.WebApp.WebAppWarImagePushSecret},
		},
	}}
	if webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript != "" {
		vol = append(vol, corev1.Volume{
			Name: "webserver-bd-" + webServer.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "webserver-bd-" + webServer.Name,
					},
				},
			},
		})
	}

	return vol
}

func (r *WebServerReconciler) generateCommandForASFStart(webServer *webserversv1alpha1.WebServer) map[string]string {
	cmd := make(map[string]string)
	cmd["start.sh"] = "#!/bin/sh\n" +

		"# This script executes any script in the list ENV_FILES\n" +
		"# the ENV_FILES can be created by the operator\n" +
		"# and are sources here.\n" +
		"# The entry point is modified...\n" +
		"# ENTRYPOINT [ \"sh\", \"-c\", \"java $JAVA_OPTS -jar app.jar\" ]\n" +
		"# ENTRYPOINT [ \"sh\", \"-c\", \"/opt/start.sh\" ]\n" +

		"if [ -n \"$ENV_FILES\" ]; then\n" +
		"(\n" +
		"for prop_file_arg in $(echo $ENV_FILES | sed \"s/,/ /g\"); do\n" +
		"for prop_file in $(find $prop_file_arg -maxdepth 0 2>/dev/null); do\n" +
		"(\n" +
		"if [ -f $prop_file ]; then\n" +
		"echo \"Run: $prop_file\"\n" +
		"sh $prop_file\n" +
		"else\n" +
		"echo \"Could not process environment for $prop_file.  File does not exist.\"\n" +
		"fi\n" +
		")\n" +
		"done\n" +
		"done\n" +
		")\n" +
		"fi\n" +

		"# Copy the war in webapps (probably we can use a ENV_FILES for that)\n" +
		"cp /deployments/*.war /deployments/webapps/ || true\n"
	if webServer.Spec.PersistentLogsConfig.CatalinaLogs || webServer.Spec.PersistentLogsConfig.AccessLogs {
		cmd["start.sh"] = cmd["start.sh"] + "#operator's configuration for logging\n" +
			"export JAVA_OPTS=\"-Dcatalina.base=. -Djava.security.egd=file:/dev/urandom -Djava.util.logging.manager=org.apache.juli.ClassLoaderLogManager -Djava.util.logging.config.file=/opt/operator_conf/logging.properties -Dpod_name=\"$HOSTNAME\"\"\n"
	}
	cmd["start.sh"] = cmd["start.sh"] + "# start the tomcat\n" +
		"java $JAVA_OPTS -jar app.jar\n"
	return cmd
}

func (r *WebServerReconciler) generateReadinessProbeScript(webServer *webserversv1alpha1.WebServer) map[string]string {
	cmd := make(map[string]string)
	cmd["readinessProbeScript.sh"] = "#!/bin/sh\n"
	var health *webserversv1alpha1.WebServerHealthCheckSpec = &webserversv1alpha1.WebServerHealthCheckSpec{}
	if webServer.Spec.WebImage != nil {
		health = webServer.Spec.WebImage.WebServerHealthCheck
	} else {
		health = webServer.Spec.WebImageStream.WebServerHealthCheck
	}
	cmd["readinessProbeScript.sh"] = cmd["readinessProbeScript.sh"] + health.ServerReadinessScript
	return cmd
}

func (r *WebServerReconciler) generateLivenessProbeScript(webServer *webserversv1alpha1.WebServer) map[string]string {
	cmd := make(map[string]string)
	cmd["livenessProbeScript.sh"] = "#!/bin/sh\n"
	var health *webserversv1alpha1.WebServerHealthCheckSpec = &webserversv1alpha1.WebServerHealthCheckSpec{}
	if webServer.Spec.WebImage != nil {
		health = webServer.Spec.WebImage.WebServerHealthCheck
	} else {
		health = webServer.Spec.WebImageStream.WebServerHealthCheck
	}
	cmd["livenessProbeScript.sh"] = cmd["livenessProbeScript.sh"] + health.ServerLivenessScript
	return cmd
}

// create the shell script to modify server.xml
func (r *WebServerReconciler) generateCommandForServerXml(webServer *webserversv1alpha1.WebServer) map[string]string {
	cmd := make(map[string]string)
	connector := ""
	if strings.HasPrefix(webServer.Spec.TLSConfig.RouteHostname, "tls") {
		// "/tls" is the dir in which the secret's contents are mounted to the pod
		connector +=
			"https=\"<!-- No HTTPS configuration discovered -->\"\n" +
				"if [ -f \"/tls/server.crt\" -a -f \"/tls/server.key\" -a -f \"/tls/ca.crt\" ] ; then\n" +

				"https=\"" +
				"<Connector port=\\\"8443\\\" protocol=\\\"HTTP/1.1\\\" " +
				"maxThreads=\\\"200\\\" SSLEnabled=\\\"true\\\"> "
		if webServer.Spec.TLSConfig.CertificateVerification == "required" || webServer.Spec.TLSConfig.CertificateVerification == "optional" {
			connector += "<SSLHostConfig caCertificateFile=\\\"/tls/ca.crt\\\" certificateVerification=\\\"" + webServer.Spec.TLSConfig.CertificateVerification + "\\\"> "
		} else {
			connector += "<SSLHostConfig caCertificateFile=\\\"/tls/ca.crt\\\"> "
		}
		connector += "<Certificate certificateFile=\\\"/tls/server.crt\\\" " +
			"certificateKeyFile=\\\"/tls/server.key\\\"/> " +
			"</SSLHostConfig> " +
			"</Connector>\"\n" +
			"elif [ -d \"/tls\" -a -f \"/tls/server.crt\" -a -f \"/tls/server.key\" ] ; then\n" +
			"https=\"" +
			"<Connector port=\\\"8443\\\" protocol=\\\"HTTP/1.1\\\" " +
			"maxThreads=\\\"200\\\" SSLEnabled=\\\"true\\\"> "
		if webServer.Spec.TLSConfig.CertificateVerification == "required" || webServer.Spec.TLSConfig.CertificateVerification == "optional" {
			connector += "<SSLHostConfig " + "certificateVerification=\\\"" + webServer.Spec.TLSConfig.CertificateVerification + "\\\"> "
		} else {
			connector += "<SSLHostConfig> "
		}
		connector += "<Certificate certificateFile=\\\"/tls/server.crt\\\" " +
			"certificateKeyFile=\\\"/tls/server.key\\\"/> " +
			"</SSLHostConfig> " +
			"</Connector>\"\n" +
			"elif [ ! -f \"/tls/server.crt\" -o ! -f \"/tls/server.key\" ] ; then \n" +
			"log_warning \"Partial HTTPS configuration, the https connector WILL NOT be configured.\" \n" +
			"fi \n" +
			"sed \"/<Service name=/a ${https}\" ${FILE}> /tmp/tmp.xml; cat /tmp/tmp.xml > ${FILE}; rm /tmp/tmp.xml\n"
	}

	cmd["test.sh"] = "FILE=`find /opt -name server.xml`\n" +
		"if [ -z \"${FILE}\" ]; then\n" +
		"  FILE=`find /deployments -name server.xml`\n" +
		"fi\n"
	if webServer.Spec.UseSessionClustering {
		cmd["test.sh"] = cmd["test.sh"] +
			"grep -q MembershipProvider ${FILE}\n" +
			"if [ $? -ne 0 ]; then\n"
		if r.getUseKUBEPing(webServer) {
			cmd["test.sh"] = cmd["test.sh"] + "  sed '/cluster.html/a        <Cluster className=\"org.apache.catalina.ha.tcp.SimpleTcpCluster\" channelSendOptions=\"6\">\\n <Channel className=\"org.apache.catalina.tribes.group.GroupChannel\">\\n <Membership className=\"org.apache.catalina.tribes.membership.cloud.CloudMembershipService\" membershipProviderClassName=\"org.apache.catalina.tribes.membership.cloud.KubernetesMembershipProvider\"/>\\n </Channel>\\n </Cluster>\\n' ${FILE}> /tmp/tmp.xml; cat /tmp/tmp.xml > ${FILE}; rm /tmp/tmp.xml\n" +
				"fi\n"
			cmd["test.sh"] = cmd["test.sh"] + connector
		} else {
			cmd["test.sh"] = cmd["test.sh"] + "  sed '/cluster.html/a        <Cluster className=\"org.apache.catalina.ha.tcp.SimpleTcpCluster\" channelSendOptions=\"6\">\\n <Channel className=\"org.apache.catalina.tribes.group.GroupChannel\">\\n <Membership className=\"org.apache.catalina.tribes.membership.cloud.CloudMembershipService\" membershipProviderClassName=\"org.apache.catalina.tribes.membership.cloud.DNSMembershipProvider\"/>\\n </Channel>\\n </Cluster>\\n' ${FILE}> /tmp/tmp.xml; cat /tmp/tmp.xml > ${FILE}; rm /tmp/tmp.xml\n" +
				"fi\n"
			cmd["test.sh"] = cmd["test.sh"] + connector
		}
	} else {
		cmd["test.sh"] = cmd["test.sh"] + connector
	}
	if webServer.Spec.PersistentLogsConfig.AccessLogs {
		cmd["test.sh"] = cmd["test.sh"] + "grep -q directory='\"/proc/self/fd\"' ${FILE}\n" +
			"if [ $? -eq 0 ]; then\n" +
			"sed -i 's|directory=\"/proc/self/fd\"|directory=\"/opt/tomcat_logs\"|g' ${FILE}\n" +
			"sed -i \"s|prefix=\\\"1\\\"|prefix=\\\"access-$HOSTNAME\\\"|g\" ${FILE}\n" +
			"sed -i 's|suffix=\"\"|suffix=\".log\"|g' ${FILE}\n" +
			"else\n" +
			"sed 's|directory=\"logs\"|directory=\"/opt/tomcat_logs\"|g' ${FILE}> /tmp/tmp.xml; cat /tmp/tmp.xml > ${FILE}; rm /tmp/tmp.xml\n" +
			"sed \"s|prefix=\\\"localhost_access_log\\\"|prefix=\\\"access-$HOSTNAME\\\"|g\" ${FILE}> /tmp/tmp.xml; cat /tmp/tmp.xml > ${FILE}; rm /tmp/tmp.xml\n" +
			"sed 's|suffix=\".txt\"|suffix=\".log\"|g' ${FILE}> /tmp/tmp.xml; cat /tmp/tmp.xml > ${FILE}; rm /tmp/tmp.xml\n" +
			"fi\n"
	}
	cmd["test.sh"] = cmd["test.sh"] + "FILE=`find /opt -name catalina.sh`\n" +
		"if [ -z \"${FILE}\" ]; then\n" +
		"  echo \"JAVA_OPS configuration executed via /opt/start/start.sh\"\n" +
		"else\n" +
		"  sed -i 's|-Djava.io.tmpdir=\"\\\\\"$CATALINA_TMPDIR\\\\\"\" \\\\|-Djava.io.tmpdir=\"$CATALINA_TMPDIR\" \\\\\\n       -Dpod_name=\"$HOSTNAME\" \\\\|g' ${FILE}\n" +
		"fi\n"

	return cmd
}

// create the shell script to pod builder
func (r *WebServerReconciler) generateCommandForBuider(script string) map[string]string {
	cmd := make(map[string]string)
	cmd["build.sh"] = script
	return cmd
}

func (r *WebServerReconciler) generateLoggingProperties(webServer *webserversv1alpha1.WebServer) map[string]string {
	cmd := make(map[string]string)
	cmd["logging.properties"] = "handlers = java.util.logging.ConsoleHandler, 1catalina.org.apache.juli.AsyncFileHandler\n" +

		".handlers = java.util.logging.ConsoleHandler, 1catalina.org.apache.juli.AsyncFileHandler\n" +

		"java.util.logging.ConsoleHandler.level = FINE\n" +
		"java.util.logging.ConsoleHandler.formatter = org.apache.juli.OneLineFormatter\n" +

		"1catalina.org.apache.juli.AsyncFileHandler.level = FINE\n" +
		"1catalina.org.apache.juli.AsyncFileHandler.directory = /opt/tomcat_logs\n" +
		"1catalina.org.apache.juli.AsyncFileHandler.prefix = catalina-${pod_name}.\n" +
		"1catalina.org.apache.juli.AsyncFileHandler.maxDays = 90"
	return cmd
}

// generateSecurityContext supplements a default SecurityContext and returns it.
func generateSecurityContext(s *corev1.SecurityContext) *corev1.SecurityContext {
	allowPrivilegeEscalation := new(bool)
	*allowPrivilegeEscalation = false

	runAsNonRoot := new(bool)
	*runAsNonRoot = true

	sTemplate := &corev1.SecurityContext{
		AllowPrivilegeEscalation: allowPrivilegeEscalation,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		RunAsNonRoot: runAsNonRoot,
	}
	if s != nil {
		return s
	}

	return sTemplate
}
