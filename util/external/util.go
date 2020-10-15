/*
Copyright 2019 The Kubernetes Authors.

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

package external

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Get uses the client and reference to get an external, unstructured object.
func Get(ctx context.Context, c client.Client, ref *corev1.ObjectReference, namespace string) (*unstructured.Unstructured, error) {
	obj := new(unstructured.Unstructured)
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetName(ref.Name)
	key := client.ObjectKey{Name: obj.GetName(), Namespace: namespace}
	if err := c.Get(ctx, key, obj); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve %s external object %q/%q", obj.GetKind(), key.Namespace, key.Name)
	}
	return obj, nil
}

// GetObjectReference converts an unstructured into object reference.
func GetObjectReference(obj *unstructured.Unstructured) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		UID:        obj.GetUID(),
	}
}

// FailuresFrom returns the FailureReason and FailureMessage fields from the external object status.
func FailuresFrom(obj *unstructured.Unstructured) (string, string, error) {
	failureReason, _, err := unstructured.NestedString(obj.Object, "status", "failureReason")
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to determine failureReason on %v %q",
			obj.GroupVersionKind(), obj.GetName())
	}
	failureMessage, _, err := unstructured.NestedString(obj.Object, "status", "failureMessage")
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to determine failureMessage on %v %q",
			obj.GroupVersionKind(), obj.GetName())
	}
	return failureReason, failureMessage, nil
}

// IsReady returns true if the Status.Ready field on an external object is true.
func IsReady(obj *unstructured.Unstructured) (bool, error) {
	ready, found, err := unstructured.NestedBool(obj.Object, "status", "ready")
	if err != nil {
		return false, errors.Wrapf(err, "failed to determine %v %q readiness",
			obj.GroupVersionKind(), obj.GetName())
	}
	return ready && found, nil
}

// IsInitialized returns true if the Status.Initialized field on an external object is true.
func IsInitialized(obj *unstructured.Unstructured) (bool, error) {
	initialized, found, err := unstructured.NestedBool(obj.Object, "status", "initialized")
	if err != nil {
		return false, errors.Wrapf(err, "failed to determine %v %q initialized",
			obj.GroupVersionKind(), obj.GetName())
	}
	return initialized && found, nil
}
