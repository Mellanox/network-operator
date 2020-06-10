package nodeinfo

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NodeAttributes tests", func() {

	nodes := []*corev1.Node{
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					nodeLabelOSName:        "ubuntu",
					nodeLabelCPUArch:       "amd64",
					nodeLabelKernelVerFull: "5.4.0-generic",
					nodeLabelOSVer:         "20.04"},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
				Labels: map[string]string{
					nodeLabelOSName:        "rhel",
					nodeLabelCPUArch:       "x86_64",
					nodeLabelKernelVerFull: "5.4.0-generic"},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-3",
				Labels: map[string]string{
					nodeLabelOSName:        "ubuntu",
					nodeLabelCPUArch:       "amd64",
					nodeLabelKernelVerFull: "4.5.0-generic"},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-4",
				Labels: map[string]string{
					nodeLabelOSName:        "coreos",
					nodeLabelCPUArch:       "x86_64",
					nodeLabelKernelVerFull: "5.4.0-generic"},
			},
		}}

	Context("Filter on empty list of nodes", func() {
		It("Should return an empty list of nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(nodeLabelHostname, "test-host").
				Build()
			filteredNodes := filter.Apply([]*corev1.Node{})

			Expect(filteredNodes).To(BeEmpty())
		})
	})

	Context("Filter with criteria that doesnt match any node", func() {
		It("Should return an empty list of nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(nodeLabelCPUArch, "arm64").
				Build()
			filteredNodes := filter.Apply(nodes)

			Expect(filteredNodes).To(BeEmpty())
		})
	})

	Context("Filter with criteria that is missing from nodes", func() {
		It("Should return an empty list of nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(nodeLabelHostname, "test-host").
				Build()
			filteredNodes := filter.Apply(nodes)

			Expect(filteredNodes).To(BeEmpty())
		})
	})

	Context("Filter with criteria that match on some nodes", func() {
		It("Should only return the relevant nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(nodeLabelKernelVerFull, "5.4.0-generic").
				WithLabel(nodeLabelCPUArch, "x86_64").
				Build()
			filteredNodes := filter.Apply(nodes)
			Expect(len(filteredNodes)).To(Equal(2))
			Expect(filteredNodes[0].Name).To(Equal("node-2"))
			Expect(filteredNodes[1].Name).To(Equal("node-4"))
		})
	})
})
