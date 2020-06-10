package nodeinfo

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A Filter applies a filter on a list of Nodes
type dummyFilter struct {
	called   bool
	filtered []*corev1.Node
}

func (df *dummyFilter) Apply(nodes []*corev1.Node) []*corev1.Node {
	df.called = true
	return df.filtered
}

var _ = Describe("nodeinfo Provider tests", func() {

	Context("GetNodesAttributes with provided filters", func() {
		It("Should Apply filters repeatedly on the nodes and return node attributes for the filtered nodes", func() {
			filter1 := &dummyFilter{filtered: []*corev1.Node{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Node"},
					ObjectMeta: metav1.ObjectMeta{Name: "Node-1"},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Node"},
					ObjectMeta: metav1.ObjectMeta{Name: "Node-2"},
				},
			}}
			filter2 := &dummyFilter{
				filtered: []*corev1.Node{
					{
						TypeMeta:   metav1.TypeMeta{Kind: "Node"},
						ObjectMeta: metav1.ObjectMeta{Name: "Node-2"},
					},
				},
			}
			provider := NewProvider([]*corev1.Node{})
			attrs := provider.GetNodesAttributes(filter1, filter2)

			Expect(filter1.called).To(BeTrue())
			Expect(filter2.called).To(BeTrue())
			Expect(len(attrs)).To(Equal(1))
			Expect(attrs[0].Name).To(Equal("Node-2"))
		})
	})

	Context("GetNodesAttributes with empty list of filters", func() {
		It("Should return all nodes attributes", func() {
			provider := NewProvider([]*corev1.Node{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Node"},
					ObjectMeta: metav1.ObjectMeta{Name: "Node-1"},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Node"},
					ObjectMeta: metav1.ObjectMeta{Name: "Node-2"},
				},
			})

			attrs := provider.GetNodesAttributes()
			Expect(len(attrs)).To(Equal(2))
			Expect(attrs[0].Name).To(Equal("Node-1"))
			Expect(attrs[1].Name).To(Equal("Node-2"))
		})
	})

	Context("GetNodesAttributes with filter returning no match", func() {
		It("Should return an empty list of nodes", func() {
			filter := &dummyFilter{filtered: []*corev1.Node{}}
			provider := NewProvider([]*corev1.Node{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Node"},
					ObjectMeta: metav1.ObjectMeta{Name: "Node-1"},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Node"},
					ObjectMeta: metav1.ObjectMeta{Name: "Node-2"},
				},
			})
			attrs := provider.GetNodesAttributes(filter)
			Expect(len(attrs)).To(Equal(0))
		})
	})
})
