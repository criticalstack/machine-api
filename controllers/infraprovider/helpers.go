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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
	mapierrors "github.com/criticalstack/machine-api/errors"
	"github.com/criticalstack/machine-api/util/external"
	"github.com/criticalstack/machine-api/util/patch"
)

func (r *InfrastructureProviderReconciler) reconcileExternal(ctx context.Context, ip *machinev1.InfrastructureProvider, ref *corev1.ObjectReference) (*unstructured.Unstructured, error) {
	logger := r.Log.WithValues("infraprovider", ip.Name, "namespace", ip.Namespace)

	obj, err := external.Get(ctx, r.Client, ref, ref.Namespace)
	if err != nil {
		if apierrors.IsNotFound(errors.Cause(err)) {
			return nil, errors.Wrapf(&mapierrors.RequeueAfterError{RequeueAfter: r.externalReadyWait},
				"could not find %v %q for InfrastructureProvider %q in namespace %q, requeuing",
				ref.GroupVersionKind(), ref.Name, ip.Name, ip.Namespace)
		}
		return nil, err
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return nil, err
	}

	// Set external object ControllerReference to the Machine.
	if err := controllerutil.SetControllerReference(ip, obj, r.scheme); err != nil {
		return nil, err
	}

	// Always attempt to Patch the external object.
	if err := patchHelper.Patch(ctx, obj); err != nil {
		return nil, err
	}

	if err := r.externalTracker.Watch(logger, obj, &handler.EnqueueRequestForOwner{OwnerType: &machinev1.InfrastructureProvider{}}); err != nil {
		return nil, err
	}
	return obj, nil
}
