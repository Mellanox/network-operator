package render

/*
 Render package renders k8s API objects from a given set of template .yaml files
 provided in a source directory and a RenderData struct to be used in the rendering process

 The objects are rendered in `Unstructured` format provided by
 k8s.io/apimachinery/pkg/apis/meta/v1/unstructured package.
*/

import (
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Renderer renders k8s objects from a manifest source dir and TemplatingData used by the templating engine
type Renderer interface {
	// RenderObjects renders kubernetes objects
	RenderObjects() ([]*unstructured.Unstructured, error)
}

// TemplatingData is used by the templating engine to render templates
type TemplatingData struct {
	// Funcs are additional Functions used during the templating process
	Funcs template.FuncMap
	// Data used for the rendering process
	Data interface{}
}

// NewRenderer creates a Renderer object, that will render all template files in manifestDirPath utilizing renderData
// in the templating engine.
func NewRenderer(manifestDirPath string, data TemplatingData) Renderer {
	return &textTemplateRenderer{
		path:           manifestDirPath,
		templatingData: data,
	}
}

// textTemplateRenderer is an implementation of the Renderer interface using golang builtin text/template package
// as its templating engine
type textTemplateRenderer struct {
	path           string
	templatingData TemplatingData
}

// RenderObjects renders kubernetes objects
func (r *textTemplateRenderer) RenderObjects() ([]*unstructured.Unstructured, error) {
	return []*unstructured.Unstructured{{}}, nil
}
