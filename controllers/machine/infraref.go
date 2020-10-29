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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/utils/pointer"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
	mapierrors "github.com/criticalstack/machine-api/errors"
	"github.com/criticalstack/machine-api/util"
	"github.com/criticalstack/machine-api/util/external"
)

// reconcileInfrastructure reconciles the Spec.InfrastructureRef object on a Machine.
func (r *MachineReconciler) reconcileInfrastructure(ctx context.Context, m *machinev1.Machine) error {
	if m.Spec.InfrastructureRef == nil {
		r.Log.Info("infrastructure reference is empty", "machine", m.Name)
		return nil
	}

	// Call generic external reconciler.
	infraReconcileResult, err := r.reconcileExternal(ctx, m, m.Spec.InfrastructureRef)
	if err != nil {
		if m.Status.InfrastructureReady && strings.Contains(err.Error(), "could not find") {
			// Infra object went missing after the machine was up and running
			r.Log.Error(err, "Machine infrastructure reference has been deleted after being ready, setting failure state")
			m.Status.FailureReason = mapierrors.MachineStatusErrorPtr(mapierrors.InvalidConfigurationMachineError)
			m.Status.FailureMessage = pointer.StringPtr(fmt.Sprintf("Machine infrastructure resource %v with name %q has been deleted after being ready",
				m.Spec.InfrastructureRef.GroupVersionKind(), m.Spec.InfrastructureRef.Name))
		}
		return err
	}
	infraConfig := infraReconcileResult.Result

	if !infraConfig.GetDeletionTimestamp().IsZero() {
		return nil
	}

	// Determine if the infrastructure provider is ready.
	ready, err := external.IsReady(infraConfig)
	if err != nil {
		return err
	}
	m.Status.InfrastructureReady = ready
	if !ready {
		return errors.Wrapf(&mapierrors.RequeueAfterError{RequeueAfter: r.externalReadyWait},
			"Infrastructure provider for Machine %q in namespace %q is not ready, requeuing", m.Name, m.Namespace,
		)
	}

	// Get Spec.ProviderID from the infrastructure provider.
	var providerID string
	if err := util.UnstructuredUnmarshalField(infraConfig, &providerID, "spec", "providerID"); err != nil {
		return errors.Wrapf(err, "failed to retrieve Spec.ProviderID from infrastructure provider for Machine %q in namespace %q", m.Name, m.Namespace)
	} else if providerID == "" {
		return errors.Errorf("retrieved empty Spec.ProviderID from infrastructure provider for Machine %q in namespace %q", m.Name, m.Namespace)
	}

	// Get and set Status.Addresses from the infrastructure provider.
	err = util.UnstructuredUnmarshalField(infraConfig, &m.Status.Addresses, "status", "addresses")
	if err != nil && err != util.ErrUnstructuredFieldNotFound {
		return errors.Wrapf(err, "failed to retrieve addresses from infrastructure provider for Machine %q in namespace %q", m.Name, m.Namespace)
	}

	// Get and set the failure domain from the infrastructure provider.
	var failureDomain string
	err = util.UnstructuredUnmarshalField(infraConfig, &failureDomain, "spec", "failureDomain")
	switch {
	case err == util.ErrUnstructuredFieldNotFound: // no-op
	case err != nil:
		return errors.Wrapf(err, "failed to failure domain from infrastructure provider for Machine %q in namespace %q", m.Name, m.Namespace)
	default:
		m.Spec.FailureDomain = pointer.StringPtr(failureDomain)
	}

	m.Spec.ProviderID = pointer.StringPtr(providerID)
	return nil
}
