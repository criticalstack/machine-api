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
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	mapierrors "github.com/criticalstack/machine-api/errors"
)

type CSRApproverReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	config *rest.Config
}

func (r *CSRApproverReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	r.config = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&certificatesv1beta1.CertificateSigningRequest{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=machine.crit.sh,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;watch;update;delete
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=create;update
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func (r *CSRApproverReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("csr", req.NamespacedName)

	csr := &certificatesv1beta1.CertificateSigningRequest{}
	if err := r.Get(ctx, req.NamespacedName, csr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	log = log.WithValues("name", csr.Name, "username", csr.Spec.Username, "groups", csr.Spec.Groups)

	// Before continuing, we determine if this is a CSR we handle. This
	// controller is only designed to auto-approve certificates for nodes.
	if !strings.HasPrefix(csr.Spec.Username, "system:node:") {
		log.Info("CSR is not for a node serving certificate")
		return ctrl.Result{}, nil
	}

	// check if already approved/denied
	for _, condition := range csr.Status.Conditions {
		switch condition.Type {
		case certificatesv1beta1.CertificateApproved, certificatesv1beta1.CertificateDenied:
			log.Info("CSR already handled", "result", condition.Type)
			return ctrl.Result{}, nil
		}
	}

	// validate CSR
	if err := r.validateCSR(ctx, csr); err != nil {
		if requeueErr, ok := errors.Cause(err).(mapierrors.HasRequeueAfterError); ok {
			return ctrl.Result{RequeueAfter: requeueErr.GetRequeueAfter()}, nil
		}
		log.Info("cannot validate CSR", "reason", err)
		return ctrl.Result{}, nil
	}

	// approve CSR
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1beta1.CertificateSigningRequestCondition{
		Type:   certificatesv1beta1.CertificateApproved,
		Reason: "approved by machine-api controller",
	})
	k, err := kubernetes.NewForConfig(r.config)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("approving CSR")
	if _, err := k.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(ctx, csr, metav1.UpdateOptions{}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
