/*
Copyright 2020 Critical Stack, LLC

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

package infraprovider

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
	mapierrors "github.com/criticalstack/machine-api/errors"
	"github.com/criticalstack/machine-api/util/external"
)

// InfrastructureProviderReconciler reconciles an InfrastructureProvider object
type InfrastructureProviderReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	ctrl            controller.Controller
	externalTracker external.ObjectTracker
	scheme          *runtime.Scheme

	externalReadyWait time.Duration
}

func (r *InfrastructureProviderReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options, externalReadyWait time.Duration) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&machinev1.InfrastructureProvider{}).
		Build(r)
	if err != nil {
		return err
	}
	r.ctrl = c
	r.scheme = mgr.GetScheme()
	r.externalTracker = external.ObjectTracker{
		Controller: c,
	}
	r.externalReadyWait = externalReadyWait
	return nil
}

// +kubebuilder:rbac:groups=machine.crit.sh,resources=infrastructureproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=machine.crit.sh,resources=infrastructureproviders/status,verbs=get;update;patch

func (r *InfrastructureProviderReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	ip := &machinev1.InfrastructureProvider{}
	if err := r.Get(ctx, req.NamespacedName, ip); err != nil {
		return ctrl.Result{}, err
	}

	obj, err := r.reconcileExternal(ctx, ip, &ip.Spec.InfrastructureRef)
	if err != nil {
		if requeueErr, ok := errors.Cause(err).(mapierrors.HasRequeueAfterError); ok {
			return ctrl.Result{RequeueAfter: requeueErr.GetRequeueAfter()}, nil
		}
		return ctrl.Result{}, err
	}
	if !obj.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}
	ready, err := external.IsReady(obj)
	if err != nil {
		return ctrl.Result{}, err
	}
	ip.Status.Ready = ready
	if err := r.Status().Update(ctx, ip); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
