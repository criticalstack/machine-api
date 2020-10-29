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
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/pkg/errors"
	authorizationv1beta1 "k8s.io/api/authorization/v1beta1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
	mapierrors "github.com/criticalstack/machine-api/errors"
)

func (r *CSRApproverReconciler) validateCSR(ctx context.Context, csr *certificatesv1beta1.CertificateSigningRequest) error {
	nodeName := strings.TrimPrefix(csr.Spec.Username, "system:node:")
	log := r.Log.WithValues("name", csr.Name, "username", csr.Spec.Username, "groups", csr.Spec.Groups, "node", nodeName)

	// check username/groups
	if len(nodeName) == 0 {
		log.Info("CSR has invalid username")
		return nil
	}
	if !sets.NewString(csr.Spec.Groups...).HasAll("system:nodes", "system:authenticated") {
		log.Info("CSR has invalid group(s)")
		return nil
	}

	// verify node exists
	node := &corev1.Node{}
	if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		return err
	}

	// check if usage is allowed
	for _, usage := range csr.Spec.Usages {
		if !isUsageAllowed(usage) {
			return errors.Errorf("usage %q not allowed", usage)
		}
	}

	// validate DNS/IP addresses requested
	m, err := r.getMachine(ctx, nodeName)
	if err != nil {
		return errors.Wrap(&mapierrors.RequeueAfterError{RequeueAfter: 5 * time.Second}, err.Error())
	}
	addresses := sets.NewString(m.Status.NodeRef.Name)
	for _, address := range m.Status.Addresses {
		addresses.Insert(address.Address)
	}
	block, _ := pem.Decode(csr.Spec.Request)
	if block == nil {
		log.Info("CSR missing request data")
		return nil
	}
	req, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return err
	}
	for _, dns := range req.DNSNames {
		if !addresses.Has(dns) {
			return errors.Errorf("node %q not allowed to specify DNS address %q", m.Status.NodeRef.Name, dns)
		}
	}
	for _, ip := range req.IPAddresses {
		if !addresses.Has(ip.String()) {
			return errors.Errorf("node %q not allowed to specify IP address %q", m.Status.NodeRef.Name, ip)
		}
	}

	// perform SAR to verify requesting user has permission to create a CSR
	sar := &authorizationv1beta1.SubjectAccessReview{
		Spec: authorizationv1beta1.SubjectAccessReviewSpec{
			User:   csr.Spec.Username,
			UID:    csr.Spec.UID,
			Groups: csr.Spec.Groups,
			Extra:  make(map[string]authorizationv1beta1.ExtraValue),
			ResourceAttributes: &authorizationv1beta1.ResourceAttributes{
				Group:    "certificates.k8s.io",
				Resource: "certificatesigningrequests",
				Verb:     "create",
			},
		},
	}
	for k, v := range csr.Spec.Extra {
		sar.Spec.Extra[k] = authorizationv1beta1.ExtraValue(v)
	}
	k, err := kubernetes.NewForConfig(r.config)
	if err != nil {
		return err
	}
	result, err := k.AuthorizationV1beta1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	if !result.Status.Allowed {
		return errors.New("CSR requestor failed SAR")
	}
	return nil
}

func (r *CSRApproverReconciler) getMachine(ctx context.Context, nodeName string) (*machinev1.Machine, error) {
	machines := &machinev1.MachineList{}
	if err := r.List(ctx, machines); err != nil {
		return nil, err
	}
	for _, m := range machines.Items {
		if m.Status.NodeRef == nil {
			continue
		}
		if nodeName == m.Status.NodeRef.Name {
			if !m.Status.InfrastructureReady {
				return nil, errors.Errorf("infrastructure for machine %q is not yet ready", m.Name)
			}
			return &m, nil
		}
	}
	return nil, errors.Errorf("cannot find machine for node %q", nodeName)
}

var allowedUsages = []certificatesv1beta1.KeyUsage{
	certificatesv1beta1.UsageDigitalSignature,
	certificatesv1beta1.UsageKeyEncipherment,
	certificatesv1beta1.UsageServerAuth,
}

func isUsageAllowed(usage certificatesv1beta1.KeyUsage) bool {
	for _, usageListItem := range allowedUsages {
		if usage == usageListItem {
			return true
		}
	}
	return false
}
