/*
Copyright 2020 The Kubernetes Authors.

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
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ObjectTracker is a helper struct to deal when watching external unstructured objects.
type ObjectTracker struct {
	m sync.Map

	Controller controller.Controller
}

// Watch uses the controller to issue a Watch only if the object hasn't been seen before.
func (o *ObjectTracker) Watch(log logr.Logger, obj runtime.Object, handler handler.EventHandler) error {
	if o.Controller == nil {
		return errors.Errorf("Watch called on ObjectTracker with no Controller")
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	_, loaded := o.m.LoadOrStore(gvk.GroupKind().String(), struct{}{})
	if loaded {
		return nil
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	log.Info("Adding watcher on external object", "GroupVersionKind", gvk.String())
	err := o.Controller.Watch(
		&source.Kind{Type: u},
		handler,
	)
	if err != nil {
		o.m.Delete(obj)
		return errors.Wrapf(err, "failed to add watcher on external object %q", gvk.String())
	}
	return nil
}
