package jbosswebserver

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	jbosswebserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/jbosswebservers/v1alpha1"

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

// Add creates a new JbossWebServer Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileJbossWebServer{client: mgr.GetClient(), scheme: mgr.GetScheme(), isOpenShift: isOpenShift(mgr.GetConfig())}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("jbosswebserver-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource JbossWebServer
	err = c.Watch(&source.Kind{Type: &jbosswebserversv1alpha1.JbossWebServer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner JbossWebServer
	enqueueRequestForOwner := handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jbosswebserversv1alpha1.JbossWebServer{},
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

var _ reconcile.Reconciler = &ReconcileJbossWebServer{}

// ReconcileJbossWebServer reconciles a JbossWebServer object
type ReconcileJbossWebServer struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client      client.Client
	scheme      *runtime.Scheme
	isOpenShift bool
}

// Reconcile reads that state of the cluster for a JbossWebServer object and makes changes based on the state read
// and what is in the JbossWebServer.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileJbossWebServer) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling JbossWebServer")
	updateJbossWebServer := false

	// Fetch the JbossWebServer tomcat
	jbossWebServer := &jbosswebserversv1alpha1.JbossWebServer{}
	err := r.client.Get(context.TODO(), request.NamespacedName, jbossWebServer)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("JbossWebServer resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get JbossWebServer")
		return reconcile.Result{}, err
	}

	ser := r.serviceForJbossWebServer(jbossWebServer)
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

	ser1 := r.serviceForJbossWebServerDNS(jbossWebServer)
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
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbossWebServer.Spec.ApplicationName, Namespace: jbossWebServer.Namespace}, &routev1.Route{})
		if err != nil && errors.IsNotFound(err) {
			// Define a new Route
			rou := r.routeForJbossWebServer(jbossWebServer)
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
	jbossWebImage := jbossWebServer.Spec.JbossWebImage
	applicationImage := ""
	if jbossWebImage != nil {
		applicationImage = jbossWebImage.ApplicationImage
	}
	if applicationImage == "" {
		// Are we building from sources and/or use the ImageStream

		if jbossWebServer.Spec.JbossWebImageStream == nil {
			reqLogger.Info("Missing JbossWebImageStream or ApplicationImage")
			return reconcile.Result{}, nil
		}

		myImageName := jbossWebServer.Spec.JbossWebImageStream.ImageStreamName
		myImageNameSpace := jbossWebServer.Spec.JbossWebImageStream.ImageStreamNamespace

		if jbossWebServer.Spec.JbossWebSources != nil {
			// Check if the ImageStream already exists, if not create a new one
			img := &imagev1.ImageStream{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbossWebServer.Spec.ApplicationName, Namespace: jbossWebServer.Namespace}, img)
			if err != nil && errors.IsNotFound(err) {
				// Define a new ImageStream
				img = r.imageStreamForJbossWebServer(jbossWebServer)
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
			myImageName = img.Name
			myImageNameSpace = img.Namespace

			buildConfig := &buildv1.BuildConfig{}
			// Check if the BuildConfig already exists, if not create a new one
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbossWebServer.Spec.ApplicationName, Namespace: jbossWebServer.Namespace}, buildConfig)
			if err != nil && errors.IsNotFound(err) {
				// Define a new BuildConfig
				buildConfig = r.buildConfigForJbossWebServer(jbossWebServer)
				reqLogger.Info("Creating a new BuildConfig.", "BuildConfig.Namespace", buildConfig.Namespace, "BuildConfig.Name", buildConfig.Name)
				err = r.client.Create(context.TODO(), buildConfig)
				if err != nil && !errors.IsAlreadyExists(err) {
					reqLogger.Error(err, "Failed to create new BuildConfig.", "BuildConfig.Namespace", buildConfig.Namespace, "BuildConfig.Name", buildConfig.Name)
					return reconcile.Result{}, err
				}
				// BuildConfig created successfully - return and requeue
				return reconcile.Result{Requeue: true}, nil
			} else if err != nil {
				reqLogger.Error(err, "Failed to get BuildConfig.")
				return reconcile.Result{}, err
			}

			build := &buildv1.Build{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbossWebServer.Spec.ApplicationName + "-" + strconv.FormatInt(buildConfig.Status.LastVersion, 10), Namespace: jbossWebServer.Namespace}, build)
			if err != nil && !errors.IsNotFound(err) {
				reqLogger.Error(err, "Failed to get Build")
			}

			switch build.Status.Phase {
			case buildv1.BuildPhaseFailed:
				reqLogger.Info("BUILD Failed: " + build.Status.Message)
				return reconcile.Result{}, nil
			case buildv1.BuildPhaseError:
				reqLogger.Info("BUILD Failed: " + build.Status.Message)
				return reconcile.Result{}, nil
			case buildv1.BuildPhaseCancelled:
				reqLogger.Info("BUILD Canceled")
				return reconcile.Result{}, nil
			}
		}

		// Check if the DeploymentConfig already exists, if not create a new one
		foundDeployment := &appsv1.DeploymentConfig{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbossWebServer.Spec.ApplicationName, Namespace: jbossWebServer.Namespace}, foundDeployment)
		if err != nil && errors.IsNotFound(err) {
			// Define a new DeploymentConfig
			dep := r.deploymentConfigForJbossWebServer(jbossWebServer, myImageName, myImageNameSpace)
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
		replicas := jbossWebServer.Spec.Replicas
		if foundReplicas != replicas {
			reqLogger.Info("Deployment replicas number does not match the JbossWebServer specification")
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
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbossWebServer.Spec.ApplicationName, Namespace: jbossWebServer.Namespace}, foundDeployment)
		if err != nil && errors.IsNotFound(err) {
			// Define a new Deployment
			dep := r.deploymentForJbossWebServer(jbossWebServer)
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
		replicas := jbossWebServer.Spec.Replicas
		if foundReplicas != replicas {
			reqLogger.Info("Deployment replicas number does not match the JbossWebServer specification")
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

	// List of pods which belongs under this jbossWebServer instance
	podList, err := GetPodsForJbossWebServer(r, jbossWebServer)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods.", "JbossWebServer.Namespace", jbossWebServer.Namespace, "JbossWebServer.Name", jbossWebServer.Name)
		return reconcile.Result{}, err
	}

	// Update the pod status...
	podsMissingIP, podsStatus := getPodStatus(podList.Items)
	if podsMissingIP {
		reqLogger.Info("Some pods don't have an IP, will requeue")
		updateJbossWebServer = true
	}
	if !reflect.DeepEqual(podsStatus, jbossWebServer.Status.Pods) {
		reqLogger.Info("Will update the JbossWebServer pod status", "New pod status list", podsStatus)
		reqLogger.Info("Will update the JbossWebServer pod status", "Existing pod status list", jbossWebServer.Status.Pods)
		jbossWebServer.Status.Pods = podsStatus
		updateJbossWebServer = true
	}

	if r.isOpenShift {
		route := &routev1.Route{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: jbossWebServer.Spec.ApplicationName, Namespace: jbossWebServer.Namespace}, route)
		if err != nil {
			reqLogger.Error(err, "Failed to get Route.", "Route.Namespace", route.Namespace, "Route.Name", route.Name)
			return reconcile.Result{}, err
		}

		hosts := make([]string, len(route.Status.Ingress))
		for i, ingress := range route.Status.Ingress {
			hosts[i] = ingress.Host
		}
		sort.Strings(hosts)
		if !reflect.DeepEqual(hosts, jbossWebServer.Status.Hosts) {
			updateJbossWebServer = true
			jbossWebServer.Status.Hosts = hosts
			reqLogger.Info("Will update Status.Hosts")
		}
	}

	// Make sure the number of active pods is the desired replica size.
	numberOfDeployedPods := int32(len(podList.Items))
	if numberOfDeployedPods != jbossWebServer.Spec.Replicas {
		reqLogger.Info("Number of deployed pods does not match the JbossWebServer specification")
		updateJbossWebServer = true
	}

	// Update the replicas
	if jbossWebServer.Status.Replicas != foundReplicas {
		reqLogger.Info("Will update Status.Replicas")
		jbossWebServer.Status.Replicas = foundReplicas
		updateJbossWebServer = true
	}
	// Update the scaledown
	numberOfPodsToScaleDown := foundReplicas - jbossWebServer.Spec.Replicas
	if jbossWebServer.Status.ScalingdownPods != numberOfPodsToScaleDown {
		reqLogger.Info("Will update Status.ScalingdownPods")
		jbossWebServer.Status.ScalingdownPods = numberOfPodsToScaleDown
		updateJbossWebServer = true
	}
	// Update if needed.
	if updateJbossWebServer {
		err := UpdateJbossWebServerStatus(jbossWebServer, r.client)
		if err != nil {
			reqLogger.Error(err, "Failed to update JbossWebServer status.")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Reconciling JbossWebServer (requeueing) DONE!!!")
		return reconcile.Result{RequeueAfter: (500 * time.Millisecond)}, nil
	}
	reqLogger.Info("Reconciling JbossWebServer DONE!!!")
	return reconcile.Result{}, nil
}

func (r *ReconcileJbossWebServer) serviceForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer) *corev1.Service {

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Service",
		},
		ObjectMeta: objectMetaForJbossWebServer(t, t.Spec.ApplicationName),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "ui",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"deploymentConfig": t.Spec.ApplicationName,
				"JbossWebServer":   t.Name,
			},
		},
	}

	controllerutil.SetControllerReference(t, service, r.scheme)
	return service
}

func (r *ReconcileJbossWebServer) serviceForJbossWebServerDNS(t *jbosswebserversv1alpha1.JbossWebServer) *corev1.Service {

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Service",
		},
		ObjectMeta: objectMetaForJbossWebServer(t, "jbosswebserver"),
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

func (r *ReconcileJbossWebServer) deploymentConfigForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer, image string, namespace string) *appsv1.DeploymentConfig {

	replicas := int32(1)
	podTemplateSpec := podTemplateSpecForJbossWebServer(t, t.Spec.ApplicationName)
	deploymentConfig := &appsv1.DeploymentConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.openshift.io/v1",
			Kind:       "DeploymentConfig",
		},
		ObjectMeta: objectMetaForJbossWebServer(t, t.Spec.ApplicationName),
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
						Kind:      "ImageStreamTag",
						Name:      image + ":latest",
						Namespace: namespace,
					},
				},
			},
				{
					Type: appsv1.DeploymentTriggerOnConfigChange,
				}},
			Replicas: replicas,
			Selector: map[string]string{
				"deploymentConfig": t.Spec.ApplicationName,
				"JbossWebServer":   t.Name,
			},
			Template: &podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(t, deploymentConfig, r.scheme)
	return deploymentConfig
}

func (r *ReconcileJbossWebServer) deploymentForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer) *kbappsv1.Deployment {

	replicas := int32(1)
	podTemplateSpec := podTemplateSpecForJbossWebServer(t, t.Spec.JbossWebImage.ApplicationImage)
	deployment := &kbappsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "k8s.io/api/apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: objectMetaForJbossWebServer(t, t.Spec.ApplicationName),
		Spec: kbappsv1.DeploymentSpec{
			Strategy: kbappsv1.DeploymentStrategy{
				Type: kbappsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deploymentConfig": t.Spec.ApplicationName,
					"JbossWebServer":   t.Name,
				},
			},
			Template: podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(t, deployment, r.scheme)
	return deployment
}

func objectMetaForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: t.Namespace,
		Labels: map[string]string{
			"application": t.Spec.ApplicationName,
		},
	}
}

func podTemplateSpecForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer, image string) corev1.PodTemplateSpec {
	objectMeta := objectMetaForJbossWebServer(t, t.Spec.ApplicationName)
	objectMeta.Labels["deploymentConfig"] = t.Spec.ApplicationName
	objectMeta.Labels["JbossWebServer"] = t.Name
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
				Env: createEnvVars(t),
			}},
		},
	}
}

func (r *ReconcileJbossWebServer) routeForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer) *routev1.Route {
	objectMeta := objectMetaForJbossWebServer(t, t.Spec.ApplicationName)
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

func (r *ReconcileJbossWebServer) imageStreamForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer) *imagev1.ImageStream {

	imageStream := &imagev1.ImageStream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "image.openshift.io/v1",
			Kind:       "ImageStream",
		},
		ObjectMeta: objectMetaForJbossWebServer(t, t.Spec.ApplicationName),
	}

	controllerutil.SetControllerReference(t, imageStream, r.scheme)
	return imageStream
}

func (r *ReconcileJbossWebServer) buildConfigForJbossWebServer(t *jbosswebserversv1alpha1.JbossWebServer) *buildv1.BuildConfig {

	buildConfig := &buildv1.BuildConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "build.openshift.io/v1",
			Kind:       "BuildConfig",
		},
		ObjectMeta: objectMetaForJbossWebServer(t, t.Spec.ApplicationName),
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Type: "Git",
					Git: &buildv1.GitBuildSource{
						URI: t.Spec.JbossWebSources.SourceRepositoryUrl,
						Ref: t.Spec.JbossWebSources.SourceRepositoryRef,
					},
					ContextDir: t.Spec.JbossWebSources.ContextDir,
				},
				Strategy: buildv1.BuildStrategy{
					Type: "Source",
					SourceStrategy: &buildv1.SourceBuildStrategy{
						Env:       createEnvBuild(t),
						ForcePull: true,
						From: corev1.ObjectReference{
							Kind:      "ImageStreamTag",
							Namespace: t.Spec.JbossWebImageStream.ImageStreamNamespace,
							Name:      t.Spec.JbossWebImageStream.ImageStreamName + ":latest",
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
			Triggers: createBuildTriggerPolicy(t),
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
func createLivenessProbe(t *jbosswebserversv1alpha1.JbossWebServer) *corev1.Probe {
	health := t.Spec.JbossWebServerHealthCheck
	livenessProbeScript := ""
	if health != nil {
		livenessProbeScript = health.ServerLivenessScript
	}
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
func createReadinessProbe(t *jbosswebserversv1alpha1.JbossWebServer) *corev1.Probe {
	health := t.Spec.JbossWebServerHealthCheck
	readinessProbeScript := ""
	if health != nil {
		readinessProbeScript = health.ServerReadinessScript
	}
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

func createCustomProbe(t *jbosswebserversv1alpha1.JbossWebServer, probeScript string) *corev1.Probe {
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

// GetPodsForJbossWebServer lists pods which belongs to the JbossWeb server
// the pods are differentiated based on the selectors
func GetPodsForJbossWebServer(r *ReconcileJbossWebServer, j *jbosswebserversv1alpha1.JbossWebServer) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.InNamespace(j.Namespace),
		client.MatchingLabels(LabelsForJbossWeb(j)),
	}
	err := r.client.List(context.TODO(), podList, listOpts...)

	if err == nil {
		// sorting pods by number in the name
		SortPodListByName(podList)
	}
	return podList, err
}

// LabelsForJbossWeb return a map of labels that are used for identification
//  of objects belonging to the particular JbossWebServer instance
func LabelsForJbossWeb(j *jbosswebserversv1alpha1.JbossWebServer) map[string]string {
	labels := map[string]string{
		"deploymentConfig": j.Spec.ApplicationName,
		"JbossWebServer":   j.Name,
	}
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

// UpdateJbossWebServerStatus updates status of the JbossWebServer resource.
func UpdateJbossWebServerStatus(j *jbosswebserversv1alpha1.JbossWebServer, client client.Client) error {
	logger := log.WithValues("JbossWebServer.Namespace", j.Namespace, "JbossWebServer.Name", j.Name)
	logger.Info("Updating status of JbossWebServer")

	if err := client.Update(context.Background(), j); err != nil {
		logger.Error(err, "Failed to update status of JbossWebServer")
		return err
	}

	logger.Info("Updated status of JbossWebServer")
	return nil
}

func UpdateStatus(j *jbosswebserversv1alpha1.JbossWebServer, client client.Client, objectDefinition runtime.Object) error {
	logger := log.WithValues("JbossWebServer.Namespace", j.Namespace, "JbossWebServer.Name", j.Name)
	logger.Info("Updating status of resource")

	if err := client.Update(context.Background(), objectDefinition); err != nil {
		logger.Error(err, "Failed to update status of resource")
		return err
	}

	logger.Info("Updated status of resource")
	return nil
}

// getPodStatus returns the pod names of the array of pods passed in
func getPodStatus(pods []corev1.Pod) (bool, []jbosswebserversv1alpha1.PodStatus) {
	var requeue = false
	var podStatuses []jbosswebserversv1alpha1.PodStatus
	for _, pod := range pods {
		podState := jbosswebserversv1alpha1.PodStateFailed

		switch pod.Status.Phase {
		case corev1.PodPending:
			podState = jbosswebserversv1alpha1.PodStatePending
		case corev1.PodRunning:
			podState = jbosswebserversv1alpha1.PodStateActive
		}

		podStatuses = append(podStatuses, jbosswebserversv1alpha1.PodStatus{
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

// Create the env for the pods we are starting.
func createEnvVars(t *jbosswebserversv1alpha1.JbossWebServer) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "KUBERNETES_NAMESPACE",
			Value: "jbosswebserver",
		},
	}
	health := t.Spec.JbossWebServerHealthCheck
	if health != nil {
		health53 := health.JbossWebServer53HealthCheck
		if health53 != nil {
			env = append(env, corev1.EnvVar{
				Name:  "JWS_ADMIN_USERNAME",
				Value: health53.JwsAdminUsername,
			})
			env = append(env, corev1.EnvVar{
				Name:  "JWS_ADMIN_PASSWORD",
				Value: health53.JwsAdminPassword,
			})
		}
	}
	return env
}

// Create the env for the maven build
func createEnvBuild(t *jbosswebserversv1alpha1.JbossWebServer) []corev1.EnvVar {
	var env []corev1.EnvVar
	sources := t.Spec.JbossWebSources
	if sources != nil {
		params := sources.JbossWebSourcesParams
		if params != nil {
			if params.MavenMirrorUrl != "" {
				env = append(env, corev1.EnvVar{
					Name:  "MAVEN_MIRROR_URL",
					Value: params.MavenMirrorUrl,
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
func createBuildTriggerPolicy(t *jbosswebserversv1alpha1.JbossWebServer) []buildv1.BuildTriggerPolicy {
	env := []buildv1.BuildTriggerPolicy{
		{
			Type:        "ImageChange",
			ImageChange: &buildv1.ImageChangeTrigger{},
		},
		{
			Type: "ConfigChange",
		},
	}
	sources := t.Spec.JbossWebSources
	if sources != nil {
		params := sources.JbossWebSourcesParams
		if params != nil {
			if params.GithubWebhookSecret != "" {
				env = append(env, buildv1.BuildTriggerPolicy{
					Type: "Github",
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: params.GithubWebhookSecret,
					},
				})
			}
			if params.GenericWebhookSecret != "" {
				env = append(env, buildv1.BuildTriggerPolicy{
					Type: "Generic",
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: params.GenericWebhookSecret,
					},
				})
			}
		}
	}
	return env
}
