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

package config

import (
	"context"
	"encoding/base64"

	configutil "github.com/criticalstack/crit/pkg/config/util"
	critv1 "github.com/criticalstack/crit/pkg/config/v1alpha2"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
	"github.com/criticalstack/machine-api/util/cloudinit"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&machinev1.Config{}).
		WithOptions(options).
		Complete(r)
}

// +kubebuilder:rbac:groups=machine.crit.sh,resources=configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=machine.crit.sh,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;patch

func (r *ConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("config", req.NamespacedName)

	cfg := &machinev1.Config{}
	if err := r.Get(ctx, req.NamespacedName, cfg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if cfg.Status.Ready {
		return ctrl.Result{}, nil
	}

	// validate the Config
	obj, err := configutil.Unmarshal([]byte(cfg.Spec.Config))
	if err != nil {
		return ctrl.Result{}, err
	}
	var data []byte
	switch c := obj.(type) {
	case *critv1.ControlPlaneConfiguration:
		log.Info("found valid ControlPlaneConfiguration", "cluster", c.ClusterName)
		data, err = configutil.Marshal(obj)
		if err != nil {
			return ctrl.Result{}, err
		}
	case *critv1.WorkerConfiguration:
		log.Info("found valid WorkerConfiguration", "cluster", c.ClusterName)
		data, err = configutil.Marshal(obj)
		if err != nil {
			return ctrl.Result{}, err
		}
	default:
		return ctrl.Result{}, errors.Errorf("Config %q contained invalid configuration type: %T", cfg.Name, c)
	}

	cloudConfig := &cloudinit.Config{
		Files:            cfg.Spec.Files,
		PreCritCommands:  cfg.Spec.PreCritCommands,
		PostCritCommands: cfg.Spec.PostCritCommands,
		Users:            cfg.Spec.Users,
		NTP:              cfg.Spec.NTP,
		Format:           cfg.Spec.Format,
		Verbosity:        cfg.Spec.Verbosity,
	}
	cloudConfig.Files = append(cloudConfig.Files, machinev1.File{
		Path:        "/var/lib/crit/config.yaml",
		Owner:       "root:root",
		Permissions: "0640",
		Encoding:    machinev1.Base64,
		Content:     base64.StdEncoding.EncodeToString(data),
	})

	secrets := make(map[string][]machinev1.SecretFile)
	for _, s := range cfg.Spec.Secrets {
		secrets[s.DataSecretName] = append(secrets[s.DataSecretName], s)
	}
	for k, files := range secrets {
		secret := &corev1.Secret{}
		if err := r.Get(ctx, client.ObjectKey{Name: k, Namespace: cfg.Namespace}, secret); err != nil {
			return ctrl.Result{}, err
		}
		for _, f := range files {
			content, ok := secret.Data[f.SecretKeyName]
			if !ok {
				log.Info("secret missing contents", "secretName", f.DataSecretName, "secretKeyName", f.SecretKeyName)
				continue
			}
			cloudConfig.Files = append(cloudConfig.Files, machinev1.File{
				Path:        f.Path,
				Owner:       f.Owner,
				Permissions: f.Permissions,
				Encoding:    f.Encoding,
				Content:     string(content),
			})
		}
	}
	data, err = cloudinit.Write(cloudConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	data, err = cloudinit.CreateMessage(data)
	if err != nil {
		return ctrl.Result{}, err
	}
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName(cfg.Name + "-"),
			Namespace: cfg.Namespace,
		},
		StringData: map[string]string{
			"cloud-config": string(data),
		},
	}
	if err := controllerutil.SetOwnerReference(cfg, s, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, s); err != nil {
		return ctrl.Result{}, err
	}
	cfg.Status.Ready = true
	cfg.Status.DataSecretName = pointer.StringPtr(s.ObjectMeta.Name)
	if err := r.Status().Update(ctx, cfg); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
