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
// +kubebuilder:rbac:groups=apps,resources=statefulsets;deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods;services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

func (r *ApusicControlPlaneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("apusiccontrolplane", req.NamespacedName)

	// your logic here

	apusicControlPlane := &webserverv1.ApusicControlPlane{}
	acpCtrl := &res.Acp{
		ApusicControlPlane: apusicControlPlane,
	}
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
	//add finalizer
	if !apusicControlPlane.DeletionTimestamp.IsZero() {
		if err := r.handleFinalizer(ctx, apusicControlPlane, acpCtrl); err != nil {
			return ctrl.Result{}, fmt.Errorf("error when handling finalizer: %v", err)
		}
		return ctrl.Result{}, nil
	}
	if !apusicControlPlane.HasFinalizer(webserverv1.ApusicControlPlaneFinalizerName) {
		if err := r.addFinalizer(ctx, apusicControlPlane); err != nil {
			return ctrl.Result{}, fmt.Errorf("error when adding finalizer: %v", err)
		}
		return ctrl.Result{}, nil
	}

	foundStateful := &appsv1.StatefulSet{}

	err = r.Get(ctx, types.NamespacedName{Name: apusicControlPlane.Name, Namespace: apusicControlPlane.Namespace}, foundStateful)

	if err != nil && errors.IsNotFound(err) {
		consulHeadless := acpCtrl.HeadlessService()
		consulStateful := acpCtrl.StatusfulSet(consulHeadless.Name)
		uideploy, pvcName, selector := acpCtrl.Deployment(consulHeadless.Name)
		uisvc := acpCtrl.UIService(selector)
		pvc := acpCtrl.UIPvc(pvcName)
		err = r.Create(ctx, consulHeadless)
		if err != nil {
			log.Error(err, "Failed to create new consul HeadlessService", "HeadlessService.Namespace", consulHeadless.Namespace, "HeadlessService.Name", consulHeadless.Name)
			return ctrl.Result{}, err
		}
		ctrl.SetControllerReference(apusicControlPlane, consulHeadless, r.Scheme)
		err = r.Create(ctx, consulStateful)
		if err != nil {
			log.Error(err, "Failed to create new consul StatefulSet", "StatefulSet.Namespace", consulStateful.Namespace, "StatefulSet.Name", consulStateful.Name)
			return ctrl.Result{}, err
		}
		ctrl.SetControllerReference(apusicControlPlane, consulStateful, r.Scheme)
		err = r.Create(ctx, uideploy)
		if err != nil {
			log.Error(err, "Failed to create new consul ui deployment", "deployment.Namespace", uideploy.Namespace, "deployment.Name", uideploy.Name)
			return ctrl.Result{}, err
		}
		ctrl.SetControllerReference(apusicControlPlane, uideploy, r.Scheme)
		err = r.Create(ctx, uisvc)
		if err != nil {
			log.Error(err, "Failed to create new consul ui service", "service.Namespace", uisvc.Namespace, "service.Name", uisvc.Name)
			return ctrl.Result{}, err
		}
		ctrl.SetControllerReference(apusicControlPlane, uisvc, r.Scheme)
		err = r.Create(ctx, pvc)
		if err != nil {
			log.Error(err, "Failed to create new consul ui pvc", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)
			return ctrl.Result{}, err
		}
		ctrl.SetControllerReference(apusicControlPlane, pvc, r.Scheme)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get consul instance")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ApusicControlPlaneReconciler) delete(ctx context.Context, acp *webserverv1.ApusicControlPlane, acpCtrl *res.Acp) error {
	var headlessSvc *corev1.Service
	headlessName := acpCtrl.ResTypeFuncs[res.HEADLESS]
	err := r.Get(ctx, client.ObjectKey{Namespace: acp.Namespace, Name: headlessName(acp.Name)}, headlessSvc)
	if err == nil {
		if err := r.Delete(ctx, headlessSvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}
	var uiSvc *corev1.Service
	uiName := acpCtrl.ResTypeFuncs[res.SVCNAME]
	err = r.Get(ctx, client.ObjectKey{Namespace: acp.Namespace, Name: uiName(acp.Name)}, uiSvc)
	if err == nil {
		if err := r.Delete(ctx, uiSvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}
	var stateful *appsv1.StatefulSet
	statefulName := acpCtrl.ResTypeFuncs[res.STATEFULNAME]
	err = r.Get(ctx, client.ObjectKey{Namespace: acp.Namespace, Name: statefulName(acp.Name)}, stateful)
	if err == nil {
		if err := r.Delete(ctx, stateful); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}
	var deploy *appsv1.Deployment
	deployName := acpCtrl.ResTypeFuncs[res.DEPLOYNAME]
	err = r.Get(ctx, client.ObjectKey{Namespace: acp.Namespace, Name: deployName(acp.Name)}, deploy)
	if err == nil {
		if err := r.Delete(ctx, deploy); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}
	var pvc *corev1.PersistentVolumeClaim
	pvcName := acpCtrl.ResTypeFuncs[res.PVCNAME]
	err = r.Get(ctx, client.ObjectKey{Namespace: acp.Namespace, Name: pvcName(acp.Name)}, pvc)
	if err == nil {
		if err := r.Delete(ctx, pvc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else if !errors.IsNotFound(err) {
		return err
	}
	return err
}

func (r *ApusicControlPlaneReconciler) addFinalizer(ctx context.Context, instance *webserverv1.ApusicControlPlane) error {
	instance.AddFinalizer(webserverv1.ApusicControlPlaneFinalizerName)
	return r.Update(ctx, instance)
}

func (r *ApusicControlPlaneReconciler) handleFinalizer(ctx context.Context, instance *webserverv1.ApusicControlPlane, acpCtrl *res.Acp) error {
	if !instance.HasFinalizer(webserverv1.ApusicControlPlaneFinalizerName) {
		return nil
	}
	if err := r.delete(ctx, instance, acpCtrl); err != nil {
		return err
	}
	instance.RemoveFinalizer(webserverv1.ApusicControlPlaneFinalizerName)
	return r.Update(ctx, instance)
}

func (r *ApusicControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webserverv1.ApusicControlPlane{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
