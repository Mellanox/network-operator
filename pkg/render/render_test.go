package render_test

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/Mellanox/mellanox-network-operator/pkg/render"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type templateData struct {
	Foo string
	Bar string
	Baz string
}

func checkRenderedUnstructured(objs []*unstructured.Unstructured, t *templateData) {
	for idx, obj := range objs {
		Expect(obj.GetKind()).To(Equal(fmt.Sprint("TestObj", idx+1)))
		Expect(obj.Object["metadata"].(map[string]interface{})["name"].(string)).To(Equal(t.Foo))
		Expect(obj.Object["spec"].(map[string]interface{})["attribute"].(string)).To(Equal(t.Bar))
		Expect(obj.Object["spec"].(map[string]interface{})["anotherAttribute"].(string)).To(Equal(t.Baz))
	}
}

var _ = Describe("Test Renderer via API", func() {
	t := &render.TemplatingData{
		Funcs: nil,
		Data:  &templateData{"foo", "bar", "baz"},
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic("Failed to get CWD")
	}
	manifestsTestDir := filepath.Join(cwd, "testdata")

	Context("Render objects from non existing dir", func() {
		It("Should fail", func() {
			r := render.NewRenderer(filepath.Join(manifestsTestDir, "doesNotExist"))
			objs, err := r.RenderObjects(t)
			Expect(err).To(HaveOccurred())
			Expect(objs).To(BeNil())
		})
	})

	Context("Render objects from existing dir with no files", func() {
		It("Should return no objects", func() {
			r := render.NewRenderer(filepath.Join(manifestsTestDir, "emptyDir"))
			objs, err := r.RenderObjects(t)
			Expect(err).ToNot(HaveOccurred())
			Expect(objs).To(BeEmpty())
		})
	})

	Context("Render objects from existing dir with empty file", func() {
		It("Should return no objects", func() {
			r := render.NewRenderer(filepath.Join(manifestsTestDir, "emptyManifests"))
			objs, err := r.RenderObjects(t)
			Expect(err).ToNot(HaveOccurred())
			Expect(objs).To(BeEmpty())
		})
	})

	Context("Render objects from valid manifests dir", func() {
		It("Should return objects in order as appear in the directory lexicographically", func() {
			r := render.NewRenderer(filepath.Join(manifestsTestDir, "manifests"))
			objs, err := r.RenderObjects(t)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(objs)).To(Equal(3))
			checkRenderedUnstructured(objs, t.Data.(*templateData))
		})
	})

	Context("Render objects from valid manifests dir with mixed file suffixes", func() {
		It("Should return objects in order as appear in the directory lexicographically", func() {
			r := render.NewRenderer(filepath.Join(manifestsTestDir, "mixedManifests"))
			objs, err := r.RenderObjects(t)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(objs)).To(Equal(3))
			checkRenderedUnstructured(objs, t.Data.(*templateData))
		})
	})
})
