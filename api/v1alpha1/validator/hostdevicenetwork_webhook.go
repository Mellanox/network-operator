/*
2023 NVIDIA CORPORATION & AFFILIATES

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

// Package validator contains webhook validation code for API types.
package validator

import (
	"context"
	"errors"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Mellanox/network-operator/api/v1alpha1"
)

// log is for logging in this package.
var hostDeviceNetworkLog = logf.Log.WithName("hostdevicenetwork-resource")

type hostDeviceNetworkValidator struct{}

var _ webhook.CustomValidator = &hostDeviceNetworkValidator{}

// SetupHostDeviceNetworkWebhookWithManager sets up webhook for HostDeviceNetwork.
func SetupHostDeviceNetworkWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.HostDeviceNetwork{}).
		WithValidator(&hostDeviceNetworkValidator{}).
		Complete()
}

//nolint:lll
//+kubebuilder:webhook:path=/validate-mellanox-com-v1alpha1-hostdevicenetwork,mutating=false,failurePolicy=fail,sideEffects=None,groups=mellanox.com,resources=hostdevicenetworks,verbs=create;update,versions=v1alpha1,name=vhostdevicenetwork.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *hostDeviceNetworkValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	if skipValidations {
		nicClusterPolicyLog.Info("skipping CR validation")
		return nil, nil
	}
	hostDeviceNetwork, ok := obj.(*v1alpha1.HostDeviceNetwork)
	if !ok {
		return nil, errors.New("failed to unmarshal HostDeviceNetwork object to validate")
	}
	hostDeviceNetworkLog.Info("validate create", "name", hostDeviceNetwork.Name)
	return nil, w.validateHostDeviceNetwork(hostDeviceNetwork)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *hostDeviceNetworkValidator) ValidateUpdate(
	_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	if skipValidations {
		nicClusterPolicyLog.Info("skipping CR validation")
		return nil, nil
	}

	hostDeviceNetwork, ok := newObj.(*v1alpha1.HostDeviceNetwork)
	if !ok {
		return nil, errors.New("failed to unmarshal HostDeviceNetwork object to validate")
	}
	hostDeviceNetworkLog.Info("validate update", "name", hostDeviceNetwork.Name)

	return nil, w.validateHostDeviceNetwork(hostDeviceNetwork)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *hostDeviceNetworkValidator) ValidateDelete(
	_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	if skipValidations {
		nicClusterPolicyLog.Info("skipping CR validation")
		return nil, nil
	}

	hostDeviceNetwork, ok := obj.(*v1alpha1.HostDeviceNetwork)
	if !ok {
		return nil, errors.New("failed to unmarshal HostDeviceNetwork object to validate")
	}

	hostDeviceNetworkLog.Info("validate delete", "name", hostDeviceNetwork.Name)

	// Validation for delete call is not required
	return nil, nil
}

/*
We are validating here HostDeviceNetwork:
  - ResourceName must be valid for k8s
*/

func (w *hostDeviceNetworkValidator) validateHostDeviceNetwork(in *v1alpha1.HostDeviceNetwork) error {
	resourceName := in.Spec.ResourceName
	if !isValidHostDeviceNetworkResourceName(resourceName) {
		var allErrs field.ErrorList
		allErrs = append(allErrs, field.Invalid(field.NewPath("Spec"), resourceName,
			"Invalid Resource name, it must consist of alphanumeric characters, '-', '_' or '.', "+
				"and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', "+
				"regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')"))
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "mellanox.com", Kind: "HostDeviceNetwork"},
			in.Name, allErrs)
	}
	return nil
}

func isValidHostDeviceNetworkResourceName(resourceName string) bool {
	resourceNamePattern := `^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`
	resourceNameRegex := regexp.MustCompile(resourceNamePattern)
	return resourceNameRegex.MatchString(resourceName)
}
