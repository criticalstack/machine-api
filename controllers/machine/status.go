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

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
)

func (r *MachineReconciler) reconcilePhase(ctx context.Context, m *machinev1.Machine) {
	if m.Status.Phase == "" {
		m.Status.Phase = machinev1.MachinePending
	}

	if m.Status.NodeRef != nil {
		m.Status.Phase = machinev1.MachineRunning
	}

	// Set the phase to "failed" if any of Status.FailureReason or Status.FailureMessage is not-nil.
	if m.Status.FailureReason != nil || m.Status.FailureMessage != nil {
		m.Status.Phase = machinev1.MachineFailed
	}

	// Set the phase to "deleting" if the deletion timestamp is set.
	if !m.DeletionTimestamp.IsZero() {
		m.Status.Phase = machinev1.MachineTerminating
	}
}
