package state

import (
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewStateManager creates a state.Manager for the given CRD Kind
func NewManager(crdKind string, k8sAPIClient client.Client, scheme *runtime.Scheme) (Manager, error) {
	return &fakeMananger{watchResources: []*source.Kind{}}, nil
}
