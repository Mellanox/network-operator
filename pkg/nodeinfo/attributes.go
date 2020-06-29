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

package nodeinfo

import (
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/network-operator/pkg/consts"
)

var log = logf.Log.WithName("nodeinfo")

// Node labels used by nodeinfo package
const (
	NodeLabelOSName        = "feature.node.kubernetes.io/system-os_release.ID"
	NodeLabelOSVer         = "feature.node.kubernetes.io/system-os_release.VERSION_ID"
	NodeLabelKernelVerFull = "feature.node.kubernetes.io/kernel-version.full"
	NodeLabelHostname      = "kubernetes.io/hostname"
	NodeLabelCPUArch       = "kubernetes.io/arch"
	NodeLabelMlnxNIC       = "feature.node.kubernetes.io/pci-15b3.present"
	NodeLabelNvGPU         = "feature.node.kubernetes.io/pci-10de.present"
)

type AttributeType int

// Attribute type Enum, add new types before Last and update the mapping below
const (
	AttrTypeOS = iota
	AttrTypeKernel
	AttrTypeHostname
	AttrTypeCPUArch
	AttrTypeLast
)

var attrToLabel = [...][]string{
	// AttrTypeOS
	{NodeLabelOSName, NodeLabelOSVer},
	// AttrTypeKernel
	{NodeLabelKernelVerFull},
	// AttrTypeHostname
	{NodeLabelHostname},
	// AttrTypeCPUArch
	{NodeLabelCPUArch},
}

// NodeAttributes provides attributes of a specific node
type NodeAttributes struct {
	// Node Name
	Name string
	// Node Attributes
	Attributes map[AttributeType]string
}

// fromLabels adds a new attribute of type attrT to NodeAttributes by joining value of selectedLabels
// using delim delimiter
//nolint:unparam
func (a *NodeAttributes) fromLabels(
	attrT AttributeType, nodeLabels map[string]string, selectedLabels []string, delim string) error {
	var attrVal string

	//nolint:prealloc
	var labelVals []string
	for _, selectedLabel := range selectedLabels {
		selectedLabelVal, ok := nodeLabels[selectedLabel]
		if !ok {
			return errors.Errorf("cannot create node attribute, missing label: %s", selectedLabel)
		}
		labelVals = append(labelVals, selectedLabelVal)
	}
	attrVal = strings.Join(labelVals, delim)

	// Note: attrVal may be empty, this could indicate a binary attribute which relies on key existence
	a.Attributes[attrT] = attrVal
	return nil
}

// newNodeAttributes creates a new NodeAttributes
func newNodeAttributes(node *corev1.Node) NodeAttributes {
	attr := NodeAttributes{
		Name:       node.GetName(),
		Attributes: make(map[AttributeType]string),
	}
	var err error

	nLabels := node.GetLabels()
	for attrType, labels := range attrToLabel {
		err = attr.fromLabels(AttributeType(attrType), nLabels, labels, "")
		if err != nil {
			log.V(consts.LogLevelWarning).Info("Warning:", "cannot create NodeAttribute(%x), %v", attrType, err)
		}
	}
	return attr
}
