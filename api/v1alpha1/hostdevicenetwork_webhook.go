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

package v1alpha1

import (
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hostDeviceNetworkLog = logf.Log.WithName("hostdevicenetwork-resource")

func (w *HostDeviceNetwork) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(w).
		Complete()
}

//nolint:lll
//+kubebuilder:webhook:path=/validate-mellanox-com-v1alpha1-hostdevicenetwork,mutating=false,failurePolicy=fail,sideEffects=None,groups=mellanox.com,resources=hostdevicenetworks,verbs=create;update,versions=v1alpha1,name=vhostdevicenetwork.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HostDeviceNetwork{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *HostDeviceNetwork) ValidateCreate() error {
	hostDeviceNetworkLog.Info("validate create", "name", w.Name)

	return w.validateHostDeviceNetwork()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *HostDeviceNetwork) ValidateUpdate(_ runtime.Object) error {
	hostDeviceNetworkLog.Info("validate update", "name", w.Name)

	return w.validateHostDeviceNetwork()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *HostDeviceNetwork) ValidateDelete() error {
	hostDeviceNetworkLog.Info("validate delete", "name", w.Name)

	// Validation for delete call is not required
	return nil
}

/*
We are validating here HostDeviceNetwork:
  - ResourceName must be valid for k8s
*/

func (w *HostDeviceNetwork) validateHostDeviceNetwork() error {
	resourceName := w.Spec.ResourceName
	if !isValidHostDeviceNetworkResourceName(resourceName) {
		var allErrs field.ErrorList
		allErrs = append(allErrs, field.Invalid(field.NewPath("Spec"), resourceName,
			"Invalid Resource name, it must consist of alphanumeric characters, '-', '_' or '.', "+
				"and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', "+
				"regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')"))
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "mellanox.com", Kind: "HostDeviceNetwork"},
			w.Name, allErrs)
	}
	return nil
}

func isValidHostDeviceNetworkResourceName(resourceName string) bool {
	resourceNamePattern := `^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`
	resourceNameRegex := regexp.MustCompile(resourceNamePattern)
	return resourceNameRegex.MatchString(resourceName)
}
