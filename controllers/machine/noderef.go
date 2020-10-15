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
	"strings"
	"time"

	"github.com/blang/semver"
	corev1 "k8s.io/api/core/v1"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
	mapierrors "github.com/criticalstack/machine-api/errors"
)

func (r *MachineReconciler) reconcileNodeRef(ctx context.Context, m *machinev1.Machine) error {
	log := r.Log.WithValues("machine", m.Name, "namespace", m.Namespace)

	if m.Status.NodeRef != nil {
		return nil
	}

	if m.Spec.ProviderID == nil {
		log.Info("Machine doesn't have a valid ProviderID yet")
		return nil
	}

	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		return err
	}
	for _, node := range nodes.Items {
		if node.Spec.ProviderID == *m.Spec.ProviderID {
			m.Status.NodeRef = &corev1.ObjectReference{
				Kind:       node.Kind,
				APIVersion: node.APIVersion,
				Name:       node.Name,
				Namespace:  node.Namespace,
			}
			v, _ := semver.Parse(strings.TrimPrefix(node.Status.NodeInfo.KubeletVersion, "v"))
			m.Status.SetVersion(v.String())
			log.Info("Set Machine's NodeRef", "noderef", m.Status.NodeRef.Name)
			r.recorder.Event(m, corev1.EventTypeNormal, "SuccessfulSetNodeRef", m.Status.NodeRef.Name)
			return nil
		}
	}
	return mapierrors.NewRequeueErrorf(1*time.Second,
		"cannot assign NodeRef to Machine %q in namespace %q, no matching Node", m.Name, m.Namespace)
}
