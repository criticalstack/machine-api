package util

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
)

// MachineToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// Machine events and returns reconciliation requests for an infrastructure provider object.
func MachineToInfrastructureMapFunc(gvk schema.GroupVersionKind) handler.ToRequestsFunc {
	return func(o handler.MapObject) []reconcile.Request {
		m, ok := o.Object.(*machinev1.Machine)
		if !ok {
			return nil
		}

		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect.
		infraGK := m.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

// GetOwnerMachine returns the Machine object owning the current resource.
func GetOwnerMachine(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*machinev1.Machine, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind == "Machine" && ref.APIVersion == machinev1.GroupVersion.String() {
			return GetMachineByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetMachineByName finds and return a Machine object using the specified params.
func GetMachineByName(ctx context.Context, c client.Client, namespace, name string) (*machinev1.Machine, error) {
	m := &machinev1.Machine{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetOwnerInfrastructureProvider returns the InfrastructureProvider object
// owning the current resource.
func GetOwnerInfrastructureProvider(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*machinev1.InfrastructureProvider, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind == "InfrastructureProvider" && ref.APIVersion == machinev1.GroupVersion.String() {
			return GetInfrastructureProviderByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetInfrastructureProviderByName finds and return a InfrastructureProvider
// object using the specified params.
func GetInfrastructureProviderByName(ctx context.Context, c client.Client, namespace, name string) (*machinev1.InfrastructureProvider, error) {
	ip := &machinev1.InfrastructureProvider{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, ip); err != nil {
		return nil, err
	}
	return ip, nil
}

var (
	ErrUnstructuredFieldNotFound = fmt.Errorf("field not found")
)

// UnstructuredUnmarshalField is a wrapper around json and unstructured objects to decode and copy a specific field
// value into an object.
func UnstructuredUnmarshalField(obj *unstructured.Unstructured, v interface{}, fields ...string) error {
	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, fields...)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve field %q from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	if !found || value == nil {
		return ErrUnstructuredFieldNotFound
	}
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return errors.Wrapf(err, "failed to json-encode field %q value from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	if err := json.Unmarshal(valueBytes, v); err != nil {
		return errors.Wrapf(err, "failed to json-decode field %q value from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	return nil
}
