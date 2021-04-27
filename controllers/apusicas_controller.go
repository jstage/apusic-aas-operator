/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	webserverv1 "github.com/jstage/apusic-aas-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultAasImage = "apusicas:9.0"

	defaultAasAppImage = "mc-apusic:v1.0"

	defaultAasImagePolicy = corev1.PullIfNotPresent

	defaultAasPort = 6888
)

// ApusicAsReconciler reconciles a ApusicAs object
type ApusicAsReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webserver.apusic.com,resources=apusicas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webserver.apusic.com,resources=apusicas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods;secrets;configmaps,verbs=get;list;watch;

func (r *ApusicAsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("apusicas", req.NamespacedName)

	// your logic here

	apusicas := &webserverv1.ApusicAs{}
	//获取所有的aas部署实例
	err := r.Get(ctx, req.NamespacedName, apusicas)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("ApusicAs resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ApusicAs Instance")
		return ctrl.Result{}, err
	}
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: apusicas.Name, Namespace: apusicas.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		dep := r.deploymentForApusicAs(ctx, apusicas)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err := r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}
	size := apusicas.Spec.Replicas
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	podList := &corev1.PodList{}
	listopts := []client.ListOption{
		client.InNamespace(apusicas.Namespace),
		client.MatchingLabels(labelsForApusicAs(apusicas.Name)),
	}
	if err = r.List(ctx, podList, listopts...); err != nil {
		log.Error(err, "Failed to list pods", "ApusicAs.Namespace", apusicas.Namespace, "ApusicAs.Name", apusicas.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, apusicas.Status.Nodes) {
		apusicas.Status.Nodes = podNames
		err := r.Status().Update(ctx, apusicas)
		if err != nil {
			log.Error(err, "Failed to update ApusicAs status")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ApusicAsReconciler) deploymentForApusicAs(ctx context.Context, aas *webserverv1.ApusicAs) *appsv1.Deployment {
	lb := labelsForApusicAs(aas.Name)
	replicas := aas.Spec.Replicas
	envars := r.envars(ctx, aas)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aas.Name,
			Namespace: aas.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: lb,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lb,
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:            "apusicas-app",
							Image:           defaultAasAppImage,
							ImagePullPolicy: defaultAasImagePolicy,
							Env:             envars,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "webapps",
									MountPath: "/webapps",
								},
							},
						},
					},
					Containers: []corev1.Container{{
						Name:            "apusicas",
						Image:           defaultAasImage,
						ImagePullPolicy: defaultAasImagePolicy,
						Ports: []corev1.ContainerPort{{
							ContainerPort: defaultAasPort,
							Name:          "apusicas",
						}},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "webapps",
								MountPath: "/webapps/app.war",
								SubPath:   "app.war",
							},
							{
								Name:      "license",
								MountPath: "/opt/AAS/license.xml",
								SubPath:   "license.xml",
							},
							{
								Name:      "appConfig",
								MountPath: "/webapps/default/META-INF/application.xml",
								SubPath:   "application.xml",
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "license",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: aas.Spec.ConfigRef,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "license",
											Path: "license.xml",
										},
									},
								},
							},
						},
						{
							Name: "appConfig",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: aas.Spec.ConfigRef,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "appConfig",
											Path: "application.xml",
										},
									},
								},
							},
						},
						{
							Name: "webapps",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(aas, dep, r.Scheme)
	return dep
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// labelsForMemcached returns the labels for selecting the resources
// belonging to the given memcached CR name.
func labelsForApusicAs(name string) map[string]string {
	return map[string]string{"app": "apusicas", "apusicas_cr": name}
}

func (r *ApusicAsReconciler) envars(ctx context.Context, aas *webserverv1.ApusicAs) []corev1.EnvVar {
	ref := aas.Spec.OssSecertRef
	secert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref,
			Namespace: aas.Namespace,
		},
	}
	r.Get(ctx, types.NamespacedName{Name: aas.Name, Namespace: aas.Namespace}, secert)
	url := aas.Spec.OssUrl
	endpoint, bucket, object := geturl(url)
	envars := make([]corev1.EnvVar, 0)
	envars = append(envars, corev1.EnvVar{
		Name:  "MINIO_ENDPOINT",
		Value: endpoint,
	}, corev1.EnvVar{
		Name:  "MINIO_KEY",
		Value: string(secert.Data["accessKeyID"]),
	}, corev1.EnvVar{
		Name:  "MINIO_SECERT",
		Value: string(secert.Data["secretAccessKey"]),
	}, corev1.EnvVar{
		Name:  "MINIO_BUCKET",
		Value: bucket,
	}, corev1.EnvVar{
		Name:  "MINIO_OBJECT",
		Value: object,
	}, corev1.EnvVar{
		Name:  "TARGET_DIR",
		Value: "/webapps",
	}, corev1.EnvVar{
		Name:  "TARGET_FILE",
		Value: "app.war",
	})
	return envars
}

//str = "http://127.0.0.1:9000/yukiso/test/aas.war"
func geturl(str string) (endpoint, bucket, object string) {
	last := strings.LastIndex(str, "/")
	first := strings.Index(str, "://")
	object = str[last+1:]
	realUrl := str[first+3:]
	endpoint = fmt.Sprintf("%s%s", str[0:first+3], realUrl[0:strings.Index(realUrl, "/")])
	bucket = realUrl[strings.Index(realUrl, "/")+1 : strings.LastIndex(realUrl, "/")]
	return
}
func (r *ApusicAsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webserverv1.ApusicAs{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
