package nodeinfo

import (
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/mellanox-network-operator/pkg/consts"
)

var log = logf.Log.WithName("nodeinfo")

// Node labels used by nodeinfo package
const (
	nodeLabelOSName        = "feature.node.kubernetes.io/system-os_release.ID"
	nodeLabelOSVer         = "feature.node.kubernetes.io/system-os_release.VERSION_ID"
	nodeLabelKernelVerFull = "feature.node.kubernetes.io/kernel-version.full"
	nodeLabelHostname      = "kubernetes.io/hostname"
	nodeLabelCPUArch       = "kubernetes.io/arch"
	nodeLabelMlnxNIC       = "feature.node.kubernetes.io/pci-15b3.present"
)

type AttributeType int

const (
	AttrTypeOS = iota
	AttrTypeKernel
	AttrTypeHostname
	AttrTypeCPUArch
)

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
	// OS name
	err = attr.fromLabels(AttrTypeOS, nLabels, []string{nodeLabelOSName, nodeLabelOSVer}, "")
	if err != nil {
		log.V(consts.LogLevelWarning).Info("Warning:", "cannot create NodeAttributes, %v", err)
	}

	// Kernel
	err = attr.fromLabels(AttrTypeKernel, nLabels, []string{nodeLabelKernelVerFull}, "")
	if err != nil {
		log.V(consts.LogLevelWarning).Info("Warning:", "cannot create NodeAttributes, %v", err)
	}

	// CPU Arch
	err = attr.fromLabels(AttrTypeCPUArch, nLabels, []string{nodeLabelCPUArch}, "")
	if err != nil {
		log.V(consts.LogLevelWarning).Info("Warning:", "cannot create NodeAttributes, %v", err)
	}

	// Hostname
	err = attr.fromLabels(AttrTypeHostname, nLabels, []string{nodeLabelHostname}, "")
	if err != nil {
		log.V(consts.LogLevelWarning).Info("Warning:", "cannot create NodeAttributes, %v", err)
	}

	return attr
}
