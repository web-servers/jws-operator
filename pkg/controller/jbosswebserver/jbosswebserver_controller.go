package jbosswebserver

import (
	"context"
	//	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	jwsserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/jwsservers/v1alpha1"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	kbappsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log = logf.Log.WithName("controller_jbosswebserver")
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new JBossWebServer Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileJBossWebServer{client: mgr.GetClient(), scheme: mgr.GetScheme(), isOpenShift: isOpenShift(mgr.GetConfig())}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("jbosswebserver-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource JBossWebServer
	err = c.Watch(&source.Kind{Type: &jwsserversv1alpha1.JBossWebServer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner JBossWebServer
	enqueueRequestForOwner := handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jwsserversv1alpha1.JBossWebServer{},
	}
	for _, obj := range []runtime.Object{&appsv1.DeploymentConfig{}, &kbappsv1.Deployment{}, &corev1.Service{}} {
		if err = c.Watch(&source.Kind{Type: obj}, &enqueueRequestForOwner); err != nil {
			return err
		}
	}
	if isOpenShift(mgr.GetConfig()) {
		if err = c.Watch(&source.Kind{Type: &routev1.Route{}}, &enqueueRequestForOwner); err != nil {
			return err
		}
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileJBossWebServer{}

// ReconcileJBossWebServer reconciles a JBossWebServer object
type ReconcileJBossWebServer struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client      client.Client
	scheme      *runtime.Scheme
	isOpenShift bool
}

// Reconcile reads that state of the cluster for a JBossWebServer object and makes changes based on the state read
// and what is in the JBossWebServer.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileJBossWebServer) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling JBossWebServer")
	updateJBossWebServer := false

	// Fetch the JBossWebServer tomcat
	jbosswebserver := &jwsserversv1alpha1.JBossWebServer{}
	err := r.client.Get(context.TODO(), request.NamespacedName, jbosswebserver)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("JBossWebServer resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get JBossWebServer")
		return reconcile.Result{}, err
	}

	ser := r.serviceForJBossWebServer(jbosswebserver)
	// Check if the Service for the Route exists
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ser.Name, Namespace: ser.Namespace}, &corev1.Service{})
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		reqLogger.Info("Creating a new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
		err = r.client.Create(context.TODO(), ser)
		if err != nil && !errors.IsAlreadyExists(err) {
			reqLogger.Error(err, "Failed to create new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
			return reconcile.Result{}, err
		}
		// Service created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service.")
		return reconcile.Result{}, err
	}

	ser1 := r.serviceForJBossWebServerDNS(jbosswebserver)
	// Check if the Service for DNSPing exists
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ser1.Name, Namespace: ser1.Namespace}, &corev1.Service{})
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		reqLogger.Info("Creating a new Service.", "Service.Namespace", ser1.Namespace, "Service.Name", ser1.Name)
		err = r.client.Create(context.TODO(), ser1)
		if err != nil && !errors.IsAlreadyExists(err) {
			reqLogger.Error(err, "Failed to create new Service.", "Service.Namespace", ser1.Namespace, "Service.Name", ser1.Name)
			return reconcile.Result{}, err
		}
		// Service created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service.")
		return reconcile.Result{}, err
	}

	// Check if the Route already exists, if not create a new one
	if r.isOpenShift {
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbosswebserver.Spec.ApplicationName, Namespace: jbosswebserver.Namespace}, &routev1.Route{})
		if err != nil && errors.IsNotFound(err) {
			// Define a new Route
			rou := r.routeForJBossWebServer(jbosswebserver)
			reqLogger.Info("Creating a new Route.", "Route.Namespace", rou.Namespace, "Route.Name", rou.Name)
			err = r.client.Create(context.TODO(), rou)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create new Route.", "Route.Namespace", rou.Namespace, "Route.Name", rou.Name)
				return reconcile.Result{}, err
			}
			// Route created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get Route.")
			return reconcile.Result{}, err
		}
	}

	foundReplicas := int32(-1) // we need the foundDeployment.Spec.Replicas which is &appsv1.DeploymentConfig{} or &kbappsv1.Deployment{}
	if jbosswebserver.Spec.ApplicationImage == "" {

		// Check if the ImageStream already exists, if not create a new one
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbosswebserver.Spec.ApplicationName, Namespace: jbosswebserver.Namespace}, &imagev1.ImageStream{})
		if err != nil && errors.IsNotFound(err) {
			// Define a new ImageStream
			img := r.imageStreamForJBossWebServer(jbosswebserver)
			reqLogger.Info("Creating a new ImageStream.", "ImageStream.Namespace", img.Namespace, "ImageStream.Name", img.Name)
			err = r.client.Create(context.TODO(), img)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create new ImageStream.", "ImageStream.Namespace", img.Namespace, "ImageStream.Name", img.Name)
				return reconcile.Result{}, err
			}
			// ImageStream created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get ImageStream.")
			return reconcile.Result{}, err
		}

		// Check if the BuildConfig already exists, if not create a new one
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbosswebserver.Spec.ApplicationName, Namespace: jbosswebserver.Namespace}, &buildv1.BuildConfig{})
		if err != nil && errors.IsNotFound(err) {
			// Define a new BuildConfig
			bui := r.buildConfigForJBossWebServer(jbosswebserver)
			reqLogger.Info("Creating a new BuildConfig.", "BuildConfig.Namespace", bui.Namespace, "BuildConfig.Name", bui.Name)
			err = r.client.Create(context.TODO(), bui)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create new BuildConfig.", "BuildConfig.Namespace", bui.Namespace, "BuildConfig.Name", bui.Name)
				return reconcile.Result{}, err
			}
			// BuildConfig created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get BuildConfig.")
			return reconcile.Result{}, err
		}

		// Check if the DeploymentConfig already exists, if not create a new one
		foundDeployment := &appsv1.DeploymentConfig{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbosswebserver.Spec.ApplicationName, Namespace: jbosswebserver.Namespace}, foundDeployment)
		if err != nil && errors.IsNotFound(err) {
			// Define a new DeploymentConfig
			dep := r.deploymentConfigForJBossWebServer(jbosswebserver)
			reqLogger.Info("Creating a new DeploymentConfig.", "DeploymentConfig.Namespace", dep.Namespace, "DeploymentConfig.Name", dep.Name)
			err = r.client.Create(context.TODO(), dep)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create new DeploymentConfig.", "DeploymentConfig.Namespace", dep.Namespace, "DeploymentConfig.Name", dep.Name)
				return reconcile.Result{}, err
			}
			// DeploymentConfig created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get DeploymentConfig.")
			return reconcile.Result{}, err
		}

		if int(foundDeployment.Status.LatestVersion) == 0 {
			reqLogger.Info("The deployment has not finished deploying the pods yet")
		}

		// Handle Scaling
		foundReplicas = foundDeployment.Spec.Replicas
		replicas := jbosswebserver.Spec.Replicas
		if foundReplicas != replicas {
			reqLogger.Info("Deployment replicas number does not match the JBossWebServer specification")
			foundDeployment.Spec.Replicas = replicas
			err = r.client.Update(context.TODO(), foundDeployment)
			if err != nil {
				reqLogger.Error(err, "Failed to update Deployment.", "Deployment.Namespace", foundDeployment.Namespace, "Deployment.Name", foundDeployment.Name)
				return reconcile.Result{}, err
			}
			// Spec updated - return and requeue
			return reconcile.Result{Requeue: true}, nil
		}
	} else {
		// Check if the Deployment already exists, if not create a new one
		foundDeployment := &kbappsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbosswebserver.Spec.ApplicationName, Namespace: jbosswebserver.Namespace}, foundDeployment)
		if err != nil && errors.IsNotFound(err) {
			// Define a new Deployment
			dep := r.deploymentForJBossWebServer(jbosswebserver)
			reqLogger.Info("Creating a new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			err = r.client.Create(context.TODO(), dep)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
				return reconcile.Result{}, err
			}
			// Deployment created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get Deployment.")
			return reconcile.Result{}, err
		}

		// Handle Scaling
		foundReplicas = *foundDeployment.Spec.Replicas
		replicas := jbosswebserver.Spec.Replicas
		if foundReplicas != replicas {
			reqLogger.Info("Deployment replicas number does not match the JBossWebServer specification")
			foundDeployment.Spec.Replicas = &replicas
			err = r.client.Update(context.TODO(), foundDeployment)
			if err != nil {
				reqLogger.Error(err, "Failed to update Deployment.", "Deployment.Namespace", foundDeployment.Namespace, "Deployment.Name", foundDeployment.Name)
				return reconcile.Result{}, err
			}
			// Spec updated - return and requeue
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// List of pods which belongs under this jbosswebserver instance
	podList, err := GetPodsForJBossWebServer(r, jbosswebserver)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods.", "JBossWebServer.Namespace", jbosswebserver.Namespace, "JBossWebServer.Name", jbosswebserver.Name)
		return reconcile.Result{}, err
	}

	// Update the pod status...
	podsMissingIP, podsStatus := getPodStatus(podList.Items, jbosswebserver.Status.Pods)
	if podsMissingIP {
		reqLogger.Info("Some pods don't have an IP, will requeue")
		updateJBossWebServer = true
	}
	if !reflect.DeepEqual(podsStatus, jbosswebserver.Status.Pods) {
		reqLogger.Info("Will update the JBossWebServer pod status", "New pod status list", podsStatus)
		reqLogger.Info("Will update the JBossWebServer pod status", "Existing pod status list", jbosswebserver.Status.Pods)
		jbosswebserver.Status.Pods = podsStatus
		updateJBossWebServer = true
	}

	if r.isOpenShift {
		route := &routev1.Route{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbosswebserver.Spec.ApplicationName, Namespace: jbosswebserver.Namespace}, route)
		if err != nil {
			reqLogger.Error(err, "Failed to get Route.", "Route.Namespace", route.Namespace, "Route.Name", route.Name)
			return reconcile.Result{}, err
		}

		hosts := make([]string, len(route.Status.Ingress))
		for i, ingress := range route.Status.Ingress {
			hosts[i] = ingress.Host
		}
		sort.Strings(hosts)
		if !reflect.DeepEqual(hosts, jbosswebserver.Status.Hosts) {
			updateJBossWebServer = true
			jbosswebserver.Status.Hosts = hosts
			reqLogger.Info("Will update Status.Hosts")
		}
	}

	// Make sure the number of active pods is the desired replica size.
	numberOfDeployedPods := int32(len(podList.Items))
	if numberOfDeployedPods != jbosswebserver.Spec.Replicas {
		reqLogger.Info("Number of deployed pods does not match the JBossWebServer specification")
		updateJBossWebServer = true
	}

	// Update the replicas
	if jbosswebserver.Status.Replicas != foundReplicas {
		reqLogger.Info("Will update Status.Replicas")
		jbosswebserver.Status.Replicas = foundReplicas
		updateJBossWebServer = true
	}
	// Update the scaledown
	numberOfPodsToScaleDown := foundReplicas - jbosswebserver.Spec.Replicas
	if jbosswebserver.Status.ScalingdownPods != numberOfPodsToScaleDown {
		reqLogger.Info("Will update Status.ScalingdownPods")
		jbosswebserver.Status.ScalingdownPods = numberOfPodsToScaleDown
		updateJBossWebServer = true
	}
	// Update if needed.
	if updateJBossWebServer {
		err := UpdateJBossWebServerStatus(jbosswebserver, r.client)
		if err != nil {
			reqLogger.Error(err, "Failed to update JBossWebServer status.")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Reconciling JBossWebServer (requeueing) DONE!!!")
		return reconcile.Result{RequeueAfter: (500 * time.Millisecond)}, nil
	}
	reqLogger.Info("Reconciling JBossWebServer DONE!!!")
	return reconcile.Result{}, nil
}

func (r *ReconcileJBossWebServer) serviceForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer) *corev1.Service {

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Service",
		},
		ObjectMeta: objectMetaForJBossWebServer(t, t.Spec.ApplicationName),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "ui",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"deploymentConfig": t.Spec.ApplicationName,
				"JBossWebServer":   t.Name,
			},
		},
	}

	controllerutil.SetControllerReference(t, service, r.scheme)
	return service
}

func (r *ReconcileJBossWebServer) serviceForJBossWebServerDNS(t *jwsserversv1alpha1.JBossWebServer) *corev1.Service {

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Service",
		},
		ObjectMeta: objectMetaForJBossWebServer(t, "jbosswebserver"),
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"application": t.Spec.ApplicationName,
			},
		},
	}

	controllerutil.SetControllerReference(t, service, r.scheme)
	return service
}

func (r *ReconcileJBossWebServer) deploymentConfigForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer) *appsv1.DeploymentConfig {

	replicas := int32(1)
	podTemplateSpec := podTemplateSpecForJBossWebServer(t, t.Spec.ApplicationName)
	deploymentConfig := &appsv1.DeploymentConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.openshift.io/v1",
			Kind:       "DeploymentConfig",
		},
		ObjectMeta: objectMetaForJBossWebServer(t, t.Spec.ApplicationName),
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyTypeRecreate,
			},
			Triggers: []appsv1.DeploymentTriggerPolicy{{
				Type: appsv1.DeploymentTriggerOnImageChange,
				ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{t.Spec.ApplicationName},
					From: corev1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: t.Spec.ApplicationName + ":latest",
					},
				},
			},
				{
					Type: appsv1.DeploymentTriggerOnConfigChange,
				}},
			Replicas: replicas,
			Selector: map[string]string{
				"deploymentConfig": t.Spec.ApplicationName,
				"JBossWebServer":   t.Name,
			},
			Template: &podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(t, deploymentConfig, r.scheme)
	return deploymentConfig
}

func (r *ReconcileJBossWebServer) deploymentForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer) *kbappsv1.Deployment {

	replicas := int32(1)
	podTemplateSpec := podTemplateSpecForJBossWebServer(t, t.Spec.ApplicationImage)
	deployment := &kbappsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "k8s.io/api/apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: objectMetaForJBossWebServer(t, t.Spec.ApplicationName),
		Spec: kbappsv1.DeploymentSpec{
			Strategy: kbappsv1.DeploymentStrategy{
				Type: kbappsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deploymentConfig": t.Spec.ApplicationName,
					"JBossWebServer":   t.Name,
				},
			},
			Template: podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(t, deployment, r.scheme)
	return deployment
}

func objectMetaForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: t.Namespace,
		Labels: map[string]string{
			"application": t.Spec.ApplicationName,
		},
	}
}

func podTemplateSpecForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer, image string) corev1.PodTemplateSpec {
	objectMeta := objectMetaForJBossWebServer(t, t.Spec.ApplicationName)
	objectMeta.Labels["deploymentConfig"] = t.Spec.ApplicationName
	objectMeta.Labels["JBossWebServer"] = t.Name
	terminationGracePeriodSeconds := int64(60)
	return corev1.PodTemplateSpec{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Containers: []corev1.Container{{
				Name:            t.Spec.ApplicationName,
				Image:           image,
				ImagePullPolicy: "Always",
				ReadinessProbe:  createReadinessProbe(t),
				LivenessProbe:   createLivenessProbe(t),
				Ports: []corev1.ContainerPort{{
					Name:          "jolokia",
					ContainerPort: 8778,
					Protocol:      corev1.ProtocolTCP,
				}, {
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				}},
				Env: []corev1.EnvVar{{
					Name:  "KUBERNETES_NAMESPACE",
					Value: "jbosswebserver",
				}, {
					Name:  "JWS_ADMIN_USERNAME",
					Value: t.Spec.JwsAdminUsername,
				}, {
					Name:  "JWS_ADMIN_PASSWORD",
					Value: t.Spec.JwsAdminPassword,
				}},
			}},
		},
	}
}

func (r *ReconcileJBossWebServer) routeForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer) *routev1.Route {
	objectMeta := objectMetaForJBossWebServer(t, t.Spec.ApplicationName)
	objectMeta.Annotations = map[string]string{
		"description": "Route for application's http service.",
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "route.openshift.io/v1",
			Kind:       "Route",
		},
		ObjectMeta: objectMeta,
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Name: t.Spec.ApplicationName,
			},
		},
	}

	controllerutil.SetControllerReference(t, route, r.scheme)
	return route
}

func (r *ReconcileJBossWebServer) imageStreamForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer) *imagev1.ImageStream {

	imageStream := &imagev1.ImageStream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "image.openshift.io/v1",
			Kind:       "ImageStream",
		},
		ObjectMeta: objectMetaForJBossWebServer(t, t.Spec.ApplicationName),
	}

	controllerutil.SetControllerReference(t, imageStream, r.scheme)
	return imageStream
}

func (r *ReconcileJBossWebServer) buildConfigForJBossWebServer(t *jwsserversv1alpha1.JBossWebServer) *buildv1.BuildConfig {

	buildConfig := &buildv1.BuildConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "build.openshift.io/v1",
			Kind:       "BuildConfig",
		},
		ObjectMeta: objectMetaForJBossWebServer(t, t.Spec.ApplicationName),
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Type: "Git",
					Git: &buildv1.GitBuildSource{
						URI: t.Spec.SourceRepositoryUrl,
						Ref: t.Spec.SourceRepositoryRef,
					},
					ContextDir: t.Spec.ContextDir,
				},
				Strategy: buildv1.BuildStrategy{
					Type: "Source",
					SourceStrategy: &buildv1.SourceBuildStrategy{
						Env: []corev1.EnvVar{{
							Name:  "MAVEN_MIRROR_URL",
							Value: t.Spec.MavenMirrorUrl,
						}, {
							Name:  "ARTIFACT_DIR",
							Value: t.Spec.ArtifactDir,
						}},
						ForcePull: true,
						From: corev1.ObjectReference{
							Kind:      "ImageStreamTag",
							Namespace: t.Spec.ImageStreamNamespace,
							Name:      t.Spec.ImageStreamName,
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: t.Spec.ApplicationName + ":latest",
					},
				},
			},
			Triggers: []buildv1.BuildTriggerPolicy{{
				Type: "Github",
				GitHubWebHook: &buildv1.WebHookTrigger{
					Secret: t.Spec.GithubWebhookSecret,
				},
			}, {
				Type: "Generic",
				GenericWebHook: &buildv1.WebHookTrigger{
					Secret: t.Spec.GenericWebhookSecret,
				},
			}, {
				Type:        "ImageChange",
				ImageChange: &buildv1.ImageChangeTrigger{},
			}, {
				Type: "ConfigChange",
			}},
		},
	}

	controllerutil.SetControllerReference(t, buildConfig, r.scheme)
	return buildConfig
}

// createLivenessProbe returns a custom probe if the serverLivenessScript string is defined and not empty in the Custom Resource.
// Otherwise, it returns nil
//
// If defined, serverLivenessScript must be a shell script that
// complies to the Kubernetes probes requirements and use the following format
// shell -c "command"
func createLivenessProbe(t *jwsserversv1alpha1.JBossWebServer) *corev1.Probe {
	livenessProbeScript := t.Spec.ServerLivenessScript
	if livenessProbeScript != "" {
		return createCustomProbe(t, livenessProbeScript)
	}
	return nil
}

// createReadinessProbe returns a custom probe if the serverReadinessScript string is defined and not empty in the Custom Resource.
// Otherwise, it use the default /health Valve via curl.
//
// If defined, serverReadinessScript must be a shell script that
// complies to the Kubernetes probes requirements and use the following format
// shell -c "command"
func createReadinessProbe(t *jwsserversv1alpha1.JBossWebServer) *corev1.Probe {
	readinessProbeScript := t.Spec.ServerReadinessScript
	if readinessProbeScript != "" {
		return createCustomProbe(t, readinessProbeScript)
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

func createCustomProbe(t *jwsserversv1alpha1.JBossWebServer, probeScript string) *corev1.Probe {
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

// GetPodsForJBossWebServer lists pods which belongs to the JBossWeb server
// the pods are differentiated based on the selectors
func GetPodsForJBossWebServer(r *ReconcileJBossWebServer, j *jwsserversv1alpha1.JBossWebServer) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.InNamespace(j.Namespace),
		client.MatchingLabels(LabelsForJBossWeb(j)),
	}
	err := r.client.List(context.TODO(), podList, listOpts...)

	if err == nil {
		// sorting pods by number in the name
		SortPodListByName(podList)
	}
	return podList, err
}

// LabelsForJBossWeb return a map of labels that are used for identification
//  of objects belonging to the particular JBossWebServer instance
func LabelsForJBossWeb(j *jwsserversv1alpha1.JBossWebServer) map[string]string {
	labels := make(map[string]string)
	labels["deploymentConfig"] = j.Spec.ApplicationName
	labels["JBossWebServer"] = j.Name
	// labels["app.kubernetes.io/name"] = j.Name
	// labels["app.kubernetes.io/managed-by"] = os.Getenv("LABEL_APP_MANAGED_BY")
	// labels["app.openshift.io/runtime"] = os.Getenv("LABEL_APP_RUNTIME")
	if j.Labels != nil {
		for labelKey, labelValue := range j.Labels {
			log.Info("labels: ", labelKey, " : ", labelValue)
			labels[labelKey] = labelValue
		}
	}
	return labels
}

// SortPodListByName sorts the pod list by number in the name
//  expecting the format which the StatefulSet works with which is `<podname>-<number>`
func SortPodListByName(podList *corev1.PodList) *corev1.PodList {
	sort.SliceStable(podList.Items, func(i, j int) bool {
		return podList.Items[i].ObjectMeta.Name < podList.Items[j].ObjectMeta.Name
	})
	return podList
}

// UpdateJBossWebServerStatus updates status of the JBossWebServer resource.
func UpdateJBossWebServerStatus(j *jwsserversv1alpha1.JBossWebServer, client client.Client) error {
	logger := log.WithValues("JBossWebServer.Namespace", j.Namespace, "JBossWebServer.Name", j.Name)
	logger.Info("Updating status of JBossWebServer")

	if err := client.Update(context.Background(), j); err != nil {
		logger.Error(err, "Failed to update status of JBossWebServer")
		return err
	}

	logger.Info("Updated status of JBossWebServer")
	return nil
}

func UpdateStatus(j *jwsserversv1alpha1.JBossWebServer, client client.Client, objectDefinition runtime.Object) error {
	logger := log.WithValues("JBossWebServer.Namespace", j.Namespace, "JBossWebServer.Name", j.Name)
	logger.Info("Updating status of resource")

	if err := client.Update(context.Background(), objectDefinition); err != nil {
		logger.Error(err, "Failed to update status of resource")
		return err
	}

	logger.Info("Updated status of resource")
	return nil
}

// getPodStatus returns the pod names of the array of pods passed in
func getPodStatus(pods []corev1.Pod, originalPodStatuses []jwsserversv1alpha1.PodStatus) (bool, []jwsserversv1alpha1.PodStatus) {
	var requeue = false
	var podStatuses []jwsserversv1alpha1.PodStatus
	podStatusesOriginalMap := make(map[string]jwsserversv1alpha1.PodStatus)
	for _, v := range originalPodStatuses {
		podStatusesOriginalMap[v.Name] = v
	}
	for _, pod := range pods {
		podState := jwsserversv1alpha1.PodStateActive
		if value, exists := podStatusesOriginalMap[pod.Name]; exists {
			podState = value.State
		}
		podStatuses = append(podStatuses, jwsserversv1alpha1.PodStatus{
			Name:  pod.Name,
			PodIP: pod.Status.PodIP,
			State: podState,
		})
		if pod.Status.PodIP == "" {
			requeue = true
		}
	}
	return requeue, podStatuses
}
