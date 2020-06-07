package nodeinfo

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("NodeAttributes tests", func() {
	var testNode corev1.Node

	JustBeforeEach(func() {
		testNode = corev1.Node{}
		testNode.Kind = "Node"
		testNode.Name = "test-node"
		testNode.Labels = make(map[string]string)
	})

	Context("Create NodeAttributes from node with all relevant labels", func() {
		It("Should return NodeAttributes with all attributes", func() {
			testNode.Labels[nodeLabelCPUArch] = "amd64"
			testNode.Labels[nodeLabelHostname] = "test-host"
			testNode.Labels[nodeLabelKernelVerFull] = "5.4.0-generic"
			testNode.Labels[nodeLabelOSName] = "ubuntu"
			testNode.Labels[nodeLabelOSVer] = "20.04"
			attr := newNodeAttributes(&testNode)

			Expect(attr.Name).To(Equal("test-node"))
			Expect(attr.Attributes[AttrTypeHostname]).To(Equal(testNode.Labels[nodeLabelHostname]))
			Expect(attr.Attributes[AttrTypeOS]).To(Equal(
				testNode.Labels[nodeLabelOSName] + testNode.Labels[nodeLabelOSVer]))
			Expect(attr.Attributes[AttrTypeKernel]).To(Equal(testNode.Labels[nodeLabelKernelVerFull]))
			Expect(attr.Attributes[AttrTypeCPUArch]).To(Equal(testNode.Labels[nodeLabelCPUArch]))

		})
	})

	Context("Create NodeAttributes from node with some relevant labels", func() {
		It("Should return NodeAttributes with some attributes", func() {
			testNode.Labels[nodeLabelHostname] = "test-host"
			testNode.Labels[nodeLabelOSName] = "ubuntu"
			testNode.Labels[nodeLabelOSVer] = "20.04"
			attr := newNodeAttributes(&testNode)

			var exist bool
			_, exist = attr.Attributes[AttrTypeHostname]
			Expect(exist).To(BeTrue())
			_, exist = attr.Attributes[AttrTypeOS]
			Expect(exist).To(BeTrue())
			_, exist = attr.Attributes[AttrTypeCPUArch]
			Expect(exist).To(BeFalse())
			_, exist = attr.Attributes[AttrTypeKernel]
			Expect(exist).To(BeFalse())
		})
	})

	Context("Create NodeAttributes from node with no OS name", func() {
		It("Should return NodeAttributes with no AttrTypeOS", func() {
			testNode.Labels[nodeLabelOSName] = "ubuntu"
			attr := newNodeAttributes(&testNode)

			_, exist := attr.Attributes[AttrTypeOS]
			Expect(exist).To(BeFalse())
		})
	})

	Context("Create NodeAttributes from node with no OS version", func() {
		It("Should return NodeAttributes with no AttrTypeOS", func() {
			testNode.Labels[nodeLabelOSVer] = "20.04"
			attr := newNodeAttributes(&testNode)

			_, exist := attr.Attributes[AttrTypeOS]
			Expect(exist).To(BeFalse())
		})
	})

	Context("Create NodeAttributes with no labels", func() {
		It("Should return NodeAttributes with no attributes", func() {
			attr := newNodeAttributes(&testNode)
			Expect(attr.Attributes).To(BeEmpty())
		})
	})
})
