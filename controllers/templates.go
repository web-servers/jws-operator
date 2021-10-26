package controllers

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"

	kbappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *WebServerReconciler) generateObjectMeta(webServer *webserversv1alpha1.WebServer, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: webServer.Namespace,
		Labels: map[string]string{
			"application": webServer.Spec.ApplicationName,
		},
	}
}

func (r *WebServerReconciler) generateRoutingService(webServer *webserversv1alpha1.WebServer) *corev1.Service {

	service := &corev1.Service{
		ObjectMeta: r.generateObjectMeta(webServer, webServer.Spec.ApplicationName),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "ui",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"deploymentConfig": webServer.Spec.ApplicationName,
				"WebServer":        webServer.Name,
			},
		},
	}

	controllerutil.SetControllerReference(webServer, service, r.Scheme)
	return service
}

func (r *WebServerReconciler) generateRoleBinding(webServer *webserversv1alpha1.WebServer) *rbac.RoleBinding {
	rolebinding := &rbac.RoleBinding{
		ObjectMeta: r.generateObjectMeta(webServer, "webserver-"+webServer.Name),
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
		Subjects: []rbac.Subject{{
			Kind: "ServiceAccount",
			Name: "default",
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

func (r *WebServerReconciler) generateConfigMapForDNS(webServer *webserversv1alpha1.WebServer) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		ObjectMeta: r.generateObjectMeta(webServer, "webserver-"+webServer.Name),
		Data:       r.generateCommandForServerXml(),
	}

	controllerutil.SetControllerReference(webServer, cmap, r.Scheme)
	return cmap
}

func (r *WebServerReconciler) generatePersistentVolumeClaim(webServer *webserversv1alpha1.WebServer) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: r.generateObjectMeta(webServer, webServer.Spec.ApplicationName),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse(webServer.Spec.WebImage.WebApp.ApplicationSizeLimit),
				},
			},
		},
	}

	controllerutil.SetControllerReference(webServer, pvc, r.Scheme)
	return pvc
}

func (r *WebServerReconciler) generateBuildPod(webServer *webserversv1alpha1.WebServer) *corev1.Pod {
	name := webServer.Spec.ApplicationName + "-build"
	objectMeta := r.generateObjectMeta(webServer, name)
	objectMeta.Labels["WebServer"] = webServer.Name
	terminationGracePeriodSeconds := int64(60)
	pod := &corev1.Pod{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			RestartPolicy:                 "OnFailure",
			Volumes: []corev1.Volume{
				{
					Name: "app-volume",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: webServer.Spec.ApplicationName},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "war",
					Image: webServer.Spec.WebImage.WebApp.Builder.Image,
					Command: []string{
						"/bin/sh",
						"-c",
					},
					Args: []string{
						webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "app-volume",
							MountPath: "/mnt",
						},
					},
				},
			},
		},
	}

	controllerutil.SetControllerReference(webServer, pod, r.Scheme)
	return pod
}

func (r *WebServerReconciler) generateDeployment(webServer *webserversv1alpha1.WebServer) *kbappsv1.Deployment {

	replicas := int32(webServer.Spec.Replicas)
	podTemplateSpec := r.generatePodTemplate(webServer, webServer.Spec.WebImage.ApplicationImage)
	deployment := &kbappsv1.Deployment{
		ObjectMeta: r.generateObjectMeta(webServer, webServer.Spec.ApplicationName),
		Spec: kbappsv1.DeploymentSpec{
			Strategy: kbappsv1.DeploymentStrategy{
				Type: kbappsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deploymentConfig": webServer.Spec.ApplicationName,
					"WebServer":        webServer.Name,
				},
			},
			Template: podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(webServer, deployment, r.Scheme)
	return deployment
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

// Create the env for the maven build
func (r *WebServerReconciler) generateEnvBuild(webServer *webserversv1alpha1.WebServer) []corev1.EnvVar {
	var env []corev1.EnvVar
	sources := webServer.Spec.WebImageStream.WebSources
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
		if params != nil {
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

func (r *WebServerReconciler) generateDeploymentConfig(webServer *webserversv1alpha1.WebServer, imageStreamName string, imageStreamNamespace string) *appsv1.DeploymentConfig {

	replicas := int32(1)
	podTemplateSpec := r.generatePodTemplate(webServer, webServer.Spec.ApplicationName)
	deploymentConfig := &appsv1.DeploymentConfig{
		ObjectMeta: r.generateObjectMeta(webServer, webServer.Spec.ApplicationName),
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyTypeRecreate,
			},
			Triggers: []appsv1.DeploymentTriggerPolicy{{
				Type: appsv1.DeploymentTriggerOnImageChange,
				ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{webServer.Spec.ApplicationName},
					From: corev1.ObjectReference{
						Kind:      "ImageStreamTag",
						Name:      imageStreamName + ":latest",
						Namespace: imageStreamNamespace,
					},
				},
			},
				{
					Type: appsv1.DeploymentTriggerOnConfigChange,
				}},
			Replicas: replicas,
			Selector: map[string]string{
				"deploymentConfig": webServer.Spec.ApplicationName,
				"WebServer":        webServer.Name,
			},
			Template: &podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(webServer, deploymentConfig, r.Scheme)
	return deploymentConfig
}

func (r *WebServerReconciler) generateRoute(webServer *webserversv1alpha1.WebServer) *routev1.Route {
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName)
	objectMeta.Annotations = map[string]string{
		"description": "Route for application's http service.",
	}
	route := &routev1.Route{
		ObjectMeta: objectMeta,
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Name: webServer.Spec.ApplicationName,
			},
		},
	}

	controllerutil.SetControllerReference(webServer, route, r.Scheme)
	return route
}

func (r *WebServerReconciler) generatePodTemplate(webServer *webserversv1alpha1.WebServer, image string) corev1.PodTemplateSpec {
	objectMeta := r.generateObjectMeta(webServer, webServer.Spec.ApplicationName)
	objectMeta.Labels["deploymentConfig"] = webServer.Spec.ApplicationName
	objectMeta.Labels["WebServer"] = webServer.Name
	var health *webserversv1alpha1.WebServerHealthCheckSpec = &webserversv1alpha1.WebServerHealthCheckSpec{}
	if webServer.Spec.WebImage != nil {
		health = webServer.Spec.WebImage.WebServerHealthCheck
	} else {
		health = webServer.Spec.WebImageStream.WebServerHealthCheck
	}
	terminationGracePeriodSeconds := int64(60)
	return corev1.PodTemplateSpec{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Containers: []corev1.Container{{
				Name:            webServer.Spec.ApplicationName,
				Image:           image,
				ImagePullPolicy: "Always",
				ReadinessProbe:  r.generateReadinessProbe(webServer, health),
				LivenessProbe:   r.generateLivenessProbe(webServer, health),
				Ports: []corev1.ContainerPort{{
					Name:          "jolokia",
					ContainerPort: 8778,
					Protocol:      corev1.ProtocolTCP,
				}, {
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				}},
				Env:          r.generateEnvVars(webServer),
				VolumeMounts: r.generateVolumeMounts(webServer),
			}},
			Volumes: r.generateVolumes(webServer),
		},
	}
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
		return r.generateCustomProbe(webServer, livenessProbeScript)
	} else {
		/* Use the default one */
		return &corev1.Probe{
			Handler: corev1.Handler{
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
		return r.generateCustomProbe(webServer, readinessProbeScript)
	} else {
		/* Use the default one */
		return &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(8080),
				},
			},
		}
	}
}

func (r *WebServerReconciler) generateCustomProbe(webServer *webserversv1alpha1.WebServer, probeScript string) *corev1.Probe {
	// If the script has the following format: shell -c "command"
	// we create the slice ["shell", "-c", "command"]
	probeScriptSlice := make([]string, 0)
	pos := strings.Index(probeScript, "\"")
	if pos != -1 {
		probeScriptSlice = append(strings.Split(probeScript[0:pos], " "), probeScript[pos:])
	} else {
		probeScriptSlice = strings.Split(probeScript, " ")
	}
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: probeScriptSlice,
			},
		},
	}
}

// Create the env for the pods we are starting.
func (r *WebServerReconciler) generateEnvVars(webServer *webserversv1alpha1.WebServer) []corev1.EnvVar {
	value := "webserver-" + webServer.Name
	if r.useKUBEPing && webServer.Spec.UseSessionClustering {
		value = webServer.Namespace
	}
	env := []corev1.EnvVar{
		{
			Name:  "KUBERNETES_NAMESPACE",
			Value: value,
		},
	}
	if webServer.Spec.UseSessionClustering {
		// Add parameter USE_SESSION_CLUSTERING
		env = append(env, corev1.EnvVar{
			Name:  "ENV_FILES",
			Value: "/test/my-files/test.sh",
		})
	}
	return env
}

// Create the VolumeMounts
func (r *WebServerReconciler) generateVolumeMounts(webServer *webserversv1alpha1.WebServer) []corev1.VolumeMount {
	var volm []corev1.VolumeMount
	if webServer.Spec.UseSessionClustering {
		volm = append(volm, corev1.VolumeMount{
			Name:      "webserver-" + webServer.Name,
			MountPath: "/test/my-files",
		})
	}
	if webServer.Spec.WebImage != nil && webServer.Spec.WebImage.WebApp != nil {
		webAppWarFileName := webServer.Spec.WebImage.WebApp.Name + ".war"
		volm = append(volm, corev1.VolumeMount{
			Name:      "app-volume",
			MountPath: webServer.Spec.WebImage.WebApp.DeployPath + webAppWarFileName,
			SubPath:   webAppWarFileName,
		})
	}
	return volm
}

// Create the Volumes
func (r *WebServerReconciler) generateVolumes(webServer *webserversv1alpha1.WebServer) []corev1.Volume {
	var vol []corev1.Volume
	if webServer.Spec.UseSessionClustering {
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
	if webServer.Spec.WebImage != nil && webServer.Spec.WebImage.WebApp != nil {
		vol = append(vol, corev1.Volume{
			Name: "app-volume",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: webServer.Spec.ApplicationName,
					ReadOnly:  true,
				},
			},
		})
	}
	return vol
}

// create the shell script to modify server.xml
//
func (r *WebServerReconciler) generateCommandForServerXml() map[string]string {
	cmd := make(map[string]string)
	if r.useKUBEPing {
		cmd["test.sh"] = "FILE=`find /opt -name server.xml`\n" +
			"grep -q MembershipProvider ${FILE}\n" +
			"if [ $? -ne 0 ]; then\n" +
			"  sed -i '/cluster.html/a        <Cluster className=\"org.apache.catalina.ha.tcp.SimpleTcpCluster\" channelSendOptions=\"6\">\\n <Channel className=\"org.apache.catalina.tribes.group.GroupChannel\">\\n <Membership className=\"org.apache.catalina.tribes.membership.cloud.CloudMembershipService\" membershipProviderClassName=\"org.apache.catalina.tribes.membership.cloud.KubernetesMembershipProvider\"/>\\n </Channel>\\n </Cluster>\\n' ${FILE}\n" +
			"fi\n"
	} else {
		cmd["test.sh"] = "FILE=`find /opt -name server.xml`\n" +
			"grep -q MembershipProvider ${FILE}\n" +
			"if [ $? -ne 0 ]; then\n" +
			"  sed -i '/cluster.html/a        <Cluster className=\"org.apache.catalina.ha.tcp.SimpleTcpCluster\" channelSendOptions=\"6\">\\n <Channel className=\"org.apache.catalina.tribes.group.GroupChannel\">\\n <Membership className=\"org.apache.catalina.tribes.membership.cloud.CloudMembershipService\" membershipProviderClassName=\"org.apache.catalina.tribes.membership.cloud.DNSMembershipProvider\"/>\\n </Channel>\\n </Cluster>\\n' ${FILE}\n" +
			"fi\n"
	}
	return cmd
}
