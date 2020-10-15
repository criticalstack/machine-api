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
	"testing"

	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetResourceFound(t *testing.T) {
	g := NewWithT(t)

	namespace := "test"
	testResourceName := "greenTemplate"
	testResourceKind := "GreenTemplate"
	testResourceAPIVersion := "green.io/v1"

	testResource := &unstructured.Unstructured{}
	testResource.SetKind(testResourceKind)
	testResource.SetAPIVersion(testResourceAPIVersion)
	testResource.SetName(testResourceName)
	testResource.SetNamespace(namespace)

	testResourceReference := &corev1.ObjectReference{
		Kind:       testResourceKind,
		APIVersion: testResourceAPIVersion,
		Name:       testResourceName,
		Namespace:  namespace,
	}

	fakeClient := fake.NewFakeClientWithScheme(runtime.NewScheme(), testResource.DeepCopy())
	got, err := Get(context.Background(), fakeClient, testResourceReference, namespace)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(testResource))
}

func TestGetResourceNotFound(t *testing.T) {
	g := NewWithT(t)

	namespace := "test"

	testResourceReference := &corev1.ObjectReference{
		Kind:       "BlueTemplate",
		APIVersion: "blue.io/v1",
		Name:       "blueTemplate",
		Namespace:  namespace,
	}

	fakeClient := fake.NewFakeClientWithScheme(runtime.NewScheme())
	_, err := Get(context.Background(), fakeClient, testResourceReference, namespace)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(errors.Cause(err))).To(BeTrue())
}
