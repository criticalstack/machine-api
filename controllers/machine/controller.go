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

package machine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
	mapierrors "github.com/criticalstack/machine-api/errors"
	"github.com/criticalstack/machine-api/util/external"
	"github.com/criticalstack/machine-api/util/patch"
)

// MachineReconciler reconciles a Machine object
type MachineReconciler struct {
	client.Client
	Log logr.Logger

	config          *rest.Config
	externalTracker external.ObjectTracker
	recorder        record.EventRecorder
	scheme          *runtime.Scheme

	externalReadyWait time.Duration
}

func (r *MachineReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options, externalReadyWait time.Duration) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&machinev1.Machine{}).
		WithOptions(options).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	r.config = mgr.GetConfig()
	r.scheme = mgr.GetScheme()
	r.recorder = mgr.GetEventRecorderFor("machine-controller")
	r.externalTracker = external.ObjectTracker{
		Controller: controller,
	}
	r.externalReadyWait = externalReadyWait
	return nil
}

// +kubebuilder:rbac:groups=machine.crit.sh,resources=machines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=machine.crit.sh,resources=machines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.crit.sh,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="core",resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *MachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	log := r.Log.WithValues("machine", req.NamespacedName)

	m := &machinev1.Machine{}
	if err := r.Get(ctx, req.NamespacedName, m); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion reconciliation loop.
	if !m.ObjectMeta.DeletionTimestamp.IsZero() {
		if m.Status.Phase != machinev1.MachineTerminating {
			m.Status.Phase = machinev1.MachineTerminating
			if err := r.Status().Update(ctx, m); err != nil {
				return ctrl.Result{}, err
			}
		}
		if res, err := r.reconcileDelete(ctx, m); err != nil {
			return res, err
		}
		controllerutil.RemoveFinalizer(m, machinev1.MachineFinalizer)
		if err := r.Update(ctx, m); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Patch any changes to Machine object on each reconciliation.
	patchHelper, err := patch.NewHelper(m, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		r.reconcilePhase(ctx, m)
		if err := patchHelper.Patch(ctx, m); err != nil {
			if reterr == nil {
				reterr = err
			}
		}
	}()

	// If the Machine doesn't have a finalizer, add one.
	controllerutil.AddFinalizer(m, machinev1.MachineFinalizer)

	// Call the inner reconciliation methods.
	reconciliationErrors := []error{
		r.reconcileInfrastructure(ctx, m),
		r.reconcileNodeRef(ctx, m),
	}

	// Parse the errors, making sure we record if there is a RequeueAfterError.
	res := ctrl.Result{}
	errs := []error{}
	for _, err := range reconciliationErrors {
		if requeueErr, ok := errors.Cause(err).(mapierrors.HasRequeueAfterError); ok {
			// Only record and log the first RequeueAfterError.
			if !res.Requeue {
				res.Requeue = true
				res.RequeueAfter = requeueErr.GetRequeueAfter()
				log.V(1).Info("Reconciliation for Machine asked to requeue", "err", err.Error())
			}
			continue
		}
		errs = append(errs, err)
	}
	return res, kerrors.NewAggregate(errs)
}

func (r *MachineReconciler) reconcileDelete(ctx context.Context, m *machinev1.Machine) (ctrl.Result, error) {
	logger := r.Log.WithValues("machine", m.Name, "namespace", m.Namespace)
	if m.Status.NodeRef == nil {
		logger.Info("machine does not have NodeRef")
		return ctrl.Result{}, nil
	}

	err := r.isDeleteNodeAllowed(ctx, m)
	isDeleteNodeAllowed := err == nil
	if err != nil {
		switch err {
		case errNoControlPlaneNodes, errLastControlPlaneNode, errNilNodeRef:
			logger.Info("Deleting Kubernetes Node associated with Machine is not allowed", "node", m.Status.NodeRef, "cause", err)
		default:
			return ctrl.Result{}, errors.Wrapf(err, "failed to check if Kubernetes Node deletion is allowed")
		}
	}

	if isDeleteNodeAllowed {
		// Drain node before deletion.
		logger.Info("Draining node", "node", m.Status.NodeRef.Name)
		if err := r.drainNode(ctx, m.Status.NodeRef.Name); err != nil {
			r.recorder.Eventf(m, corev1.EventTypeWarning, "FailedDrainNode", "error draining Machine's node %q: %v", m.Status.NodeRef.Name, err)
			return ctrl.Result{}, err
		}
		r.recorder.Eventf(m, corev1.EventTypeNormal, "SuccessfulDrainNode", "success draining Machine's node %q", m.Status.NodeRef.Name)
	}

	if err := r.reconcileDeleteExternal(ctx, m); err != nil {
		// Return early and don't remove the finalizer if we got an error or
		// the external reconciliation deletion isn't ready.
		return ctrl.Result{}, err
	}

	// We only delete the node after the underlying infrastructure is gone.
	// https://github.com/kubernetes-sigs/cluster-api/issues/2565
	if isDeleteNodeAllowed {
		logger.Info("Deleting node", "node", m.Status.NodeRef.Name)

		var deleteNodeErr error
		waitErr := wait.PollImmediate(2*time.Second, 10*time.Second, func() (bool, error) {
			if deleteNodeErr = r.deleteNode(ctx, m.Status.NodeRef.Name); deleteNodeErr != nil && !apierrors.IsNotFound(deleteNodeErr) {
				return false, nil
			}
			return true, nil
		})
		if waitErr != nil {
			logger.Error(deleteNodeErr, "Timed out deleting node, moving on", "node", m.Status.NodeRef.Name)
			r.recorder.Eventf(m, corev1.EventTypeWarning, "FailedDeleteNode", "error deleting Machine's node: %v", deleteNodeErr)
		}
	}

	return ctrl.Result{}, nil
}
