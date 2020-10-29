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

package controllers

import (
	"context"
	"encoding/json"

	nodeutil "github.com/criticalstack/crit/pkg/kubernetes/util/node"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
)

// NodeReconciler reconciles a corev1.Node object and creates Machine objects
// for nodes where one does not exist. This ensures that even nodes that were
// created outside of the machine-api are described by Kubernetes resources.
type NodeReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	config *rest.Config
}

func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	r.config = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&corev1.Node{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=machine.crit.sh,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=machine.crit.sh,resources=configs;configs/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete

func (r *NodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("node", req.NamespacedName)

	n := &corev1.Node{}
	if err := r.Get(ctx, req.NamespacedName, n); err != nil {
		if apierrors.IsNotFound(err) {
			// TODO(chrism): handle node deletion?
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	annotations := n.GetAnnotations()
	if _, ok := annotations[machinev1.NodeOwnerLabelName]; !ok {
		log.Info("machine label not found")
		if err := r.ensureMachineForNode(ctx, n); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *NodeReconciler) ensureMachineForNode(ctx context.Context, n *corev1.Node) error {
	log := r.Log.WithValues("node", n.Name)

	machines := &machinev1.MachineList{}
	if err := r.List(ctx, machines); err != nil {
		return err
	}
	for _, m := range machines.Items {
		if m.Spec.ProviderID != nil && *m.Spec.ProviderID != "" && *m.Spec.ProviderID == n.Spec.ProviderID {
			log.V(1).Info("node already has a machine associated with it, only needs an annotation")
			return r.setMachineAnnotation(ctx, &m, n.Name)
		}
	}
	m := &machinev1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: metav1.NamespaceSystem,
		},
		Spec: machinev1.MachineSpec{
			ProviderID: pointer.StringPtr(n.Spec.ProviderID),
		},
	}
	if err := r.Create(ctx, m); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return r.setMachineAnnotation(ctx, m, n.Name)
}

func (r *NodeReconciler) setMachineAnnotation(ctx context.Context, m *machinev1.Machine, name string) error {
	ref := corev1.ObjectReference{
		APIVersion: m.APIVersion,
		Kind:       "Machine",
		Name:       m.ObjectMeta.Name,
		Namespace:  m.Namespace,
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	k, err := kubernetes.NewForConfig(r.config)
	if err != nil {
		return err
	}
	return nodeutil.PatchNode(ctx, k, name, func(n *corev1.Node) {
		annotations := n.GetAnnotations()
		annotations[machinev1.NodeOwnerLabelName] = string(data)
	})
}
