/*
Copyright 2020 NVIDIA

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

package state //nolint:dupl

import (
	"context"
	"strings"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

const (
	stateMacvlanNetworkName        = "state-Macvlan-Network"
	stateMacvlanNetworkDescription = "Macvlan net-attach-def CR deployed in cluster"
	lastNetworkNamespaceAnnot      = "operator.macvlannetwork.mellanox.com/last-network-namespace"
)

// NewStateMacvlanNetwork creates a new state for MacvlanNetwork CR
func NewStateMacvlanNetwork(k8sAPIClient client.Client, scheme *runtime.Scheme, manifestDir string) (State, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	return &stateMacvlanNetwork{
		stateSkel: stateSkel{
			name:        stateMacvlanNetworkName,
			description: stateMacvlanNetworkDescription,
			client:      k8sAPIClient,
			scheme:      scheme,
			renderer:    renderer,
		}}, nil
}

type stateMacvlanNetwork struct {
	stateSkel
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
func (s *stateMacvlanNetwork) Sync(customResource interface{}, _ InfoCatalog) (SyncState, error) {
	cr := customResource.(*mellanoxv1alpha1.MacvlanNetwork)
	log.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.Name, "Namespace:", cr.Namespace)

	objs, err := s.getManifestObjects(cr)
	if err != nil {
		return SyncStateError, errors.Wrap(err, "failed to render MacvlanNetwork")
	}

	if len(objs) == 0 {
		return SyncStateError, errors.Wrap(err, "no rendered objects found")
	}

	netAttDef := objs[0]
	if netAttDef.GetKind() != "NetworkAttachmentDefinition" {
		return SyncStateError, errors.Wrap(err, "no NetworkAttachmentDefinition object found")
	}

	// Delete NetworkAttachmentDefinition if not in desired namespace
	if err = s.handleNamespaceChange(cr, netAttDef); err != nil {
		return SyncStateError, errors.Wrap(err, "Couldn't delete NetworkAttachmentDefinition CR")
	}

	err = s.createOrUpdateObjs(func(obj *unstructured.Unstructured) error {
		if err := controllerutil.SetControllerReference(cr, obj, s.scheme); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}
		return nil
	}, objs)

	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create/update objects")
	}

	// Check objects status
	syncState, err := s.getSyncState(objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to get sync state")
	}

	if err := s.updateNetAttDefNamespace(cr, netAttDef); err != nil {
		return SyncStateError, err
	}

	// Get NetworkAttachmentDefinition SelfLink
	if err := s.getObj(netAttDef); err != nil {
		return SyncStateError, errors.Wrap(err, "failed to get NetworkAttachmentDefinition")
	}
	return syncState, nil
}

// Get a map of source kinds that should be watched for the state keyed by the source kind name
func (s *stateMacvlanNetwork) GetWatchSources() map[string]*source.Kind {
	wr := make(map[string]*source.Kind)
	wr["MacvlanNetwork"] = &source.Kind{Type: &mellanoxv1alpha1.MacvlanNetwork{}}
	wr["NetworkAttachmentDefinition"] = &source.Kind{Type: &netattdefv1.NetworkAttachmentDefinition{}}
	return wr
}

func (s *stateMacvlanNetwork) getManifestObjects(
	cr *mellanoxv1alpha1.MacvlanNetwork) ([]*unstructured.Unstructured, error) {
	data := map[string]interface{}{}
	data["NetworkName"] = cr.Name
	if cr.Spec.NetworkNamespace == "" {
		data["NetworkNamespace"] = "default"
	} else {
		data["NetworkNamespace"] = cr.Spec.NetworkNamespace
	}

	data["Master"] = cr.Spec.Master
	data["Mode"] = cr.Spec.Mode
	data["Mtu"] = cr.Spec.Mtu

	if cr.Spec.IPAM != "" {
		data["Ipam"] = "\"ipam\":" + strings.Join(strings.Fields(cr.Spec.IPAM), "")
	} else {
		data["Ipam"] = "\"ipam\":{}"
	}

	// render objects
	log.V(consts.LogLevelDebug).Info("Rendering objects", "data:", data)
	objs, err := s.renderer.RenderObjects(&render.TemplatingData{Data: data})
	if err != nil {
		return nil, errors.Wrap(err, "failed to render objects")
	}
	log.V(consts.LogLevelDebug).Info("Rendered", "objects:", objs)
	return objs, nil
}

func (s *stateMacvlanNetwork) handleNamespaceChange(cr *mellanoxv1alpha1.MacvlanNetwork,
	netAttDef *unstructured.Unstructured) error {
	// Delete NetworkAttachmentDefinition if not in desired namespace
	lnns, lnnsExists := cr.GetAnnotations()[lastNetworkNamespaceAnnot]
	netAttDefChangedNamespace := lnnsExists && netAttDef.GetNamespace() != lnns
	if netAttDefChangedNamespace {
		err := s.client.Delete(context.TODO(), &netattdefv1.NetworkAttachmentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cr.GetName(),
				Namespace: lnns,
			},
		})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (s *stateMacvlanNetwork) updateNetAttDefNamespace(cr *mellanoxv1alpha1.MacvlanNetwork,
	netAttDef *unstructured.Unstructured) error {
	lnns, lnnsExists := cr.GetAnnotations()[lastNetworkNamespaceAnnot]
	netAttDefChangedNamespace := lnnsExists && netAttDef.GetNamespace() != lnns
	if !lnnsExists || netAttDefChangedNamespace {
		anno := map[string]string{lastNetworkNamespaceAnnot: netAttDef.GetNamespace()}
		cr.SetAnnotations(anno)
		if err := s.client.Update(context.Background(), cr); err != nil {
			return errors.Wrap(err, "failed to update MacvlanNetwork annotations")
		}
	}
	return nil
}
