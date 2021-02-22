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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webserverv1 "github.com/jstage/apusic-aas-operator/api/v1"
)

// ApusicControlPlaneReconciler reconciles a ApusicControlPlane object
type ApusicControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webserver.apusic.com,resources=apusiccontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webserver.apusic.com,resources=apusiccontrolplanes/status,verbs=get;update;patch

func (r *ApusicControlPlaneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("apusiccontrolplane", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *ApusicControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webserverv1.ApusicControlPlane{}).
		Complete(r)
}
