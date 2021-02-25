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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webserverv1 "github.com/jstage/apusic-aas-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	res "github.com/jstage/apusic-aas-operator/pkg/resources"
)

// ApusicControlPlaneReconciler reconciles a ApusicControlPlane object
type ApusicControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webserver.apusic.com,resources=apusiccontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webserver.apusic.com,resources=apusiccontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods;services,verbs=get;list;watch;create;update;patch;delete

func (r *ApusicControlPlaneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("apusiccontrolplane", req.NamespacedName)

	// your logic here

	apusicControlPlane := &webserverv1.ApusicControlPlane{}
	err := r.Get(ctx, req.NamespacedName, apusicControlPlane)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("ApusicControlPlane resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ApusicControlPlane Instance")
		return ctrl.Result{}, err
	}
	acp := &res.Acp{
		ApusicControlPlane: apusicControlPlane,
	}
	foundStateful := &appsv1.StatefulSet{}

	err = r.Get(ctx, types.NamespacedName{Name: apusicControlPlane.Name, Namespace: apusicControlPlane.Namespace}, foundStateful)

	if err != nil && errors.IsNotFound(err) {
		consulHeadless := acp.HeadlessService()
		consulStateful := acp.StatusfulSet(consulHeadless.Name)
		uideploy, pvcName, selector := acp.Deployment(consulHeadless.Name)
		uisvc := acp.UIService(selector)
		pvc := acp.UIPvc(pvcName)
		err = r.Create(ctx, consulHeadless)
		if err != nil {
			log.Error(err, "Failed to create new consul HeadlessService", "HeadlessService.Namespace", consulHeadless.Namespace, "HeadlessService.Name", consulHeadless.Name)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		ctrl.SetControllerReference(apusicControlPlane, consulHeadless, r.Scheme)
		err = r.Create(ctx, consulStateful)
		if err != nil {
			log.Error(err, "Failed to create new consul StatefulSet", "StatefulSet.Namespace", consulStateful.Namespace, "StatefulSet.Name", consulStateful.Name)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		ctrl.SetControllerReference(apusicControlPlane, consulStateful, r.Scheme)
		err = r.Create(ctx, uideploy)
		if err != nil {
			log.Error(err, "Failed to create new consul ui deployment", "deployment.Namespace", uideploy.Namespace, "deployment.Name", uideploy.Name)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		ctrl.SetControllerReference(apusicControlPlane, uideploy, r.Scheme)
		err = r.Create(ctx, uisvc)
		if err != nil {
			log.Error(err, "Failed to create new consul ui service", "service.Namespace", uisvc.Namespace, "service.Name", uisvc.Name)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		ctrl.SetControllerReference(apusicControlPlane, uisvc, r.Scheme)
		err = r.Create(ctx, pvc)
		if err != nil {
			log.Error(err, "Failed to create new consul ui pvc", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		ctrl.SetControllerReference(apusicControlPlane, pvc, r.Scheme)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get consul instance")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ApusicControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webserverv1.ApusicControlPlane{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
