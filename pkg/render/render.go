package render

/*
 Render package renders k8s API objects from a given set of template .yaml files
 provided in a source directory and a RenderData struct to be used in the rendering process

 The objects are rendered in `Unstructured` format provided by
 k8s.io/apimachinery/pkg/apis/meta/v1/unstructured package.
*/

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Renderer renders k8s objects from a manifest source dir and TemplatingData used by the templating engine
type Renderer interface {
	// RenderObjects renders kubernetes objects using provided TemplatingData
	RenderObjects(data *TemplatingData) ([]*unstructured.Unstructured, error)
}

// TemplatingData is used by the templating engine to render templates
type TemplatingData struct {
	// Funcs are additional Functions used during the templating process
	Funcs template.FuncMap
	// Data used for the rendering process
	Data interface{}
}

// NewRenderer creates a Renderer object, that will render all template files in manifestDirPath.
func NewRenderer(manifestDirPath string) Renderer {
	return &textTemplateRenderer{
		dirPath: manifestDirPath,
	}
}

// textTemplateRenderer is an implementation of the Renderer interface using golang builtin text/template package
// as its templating engine
type textTemplateRenderer struct {
	dirPath string
}

// RenderObjects renders kubernetes objects utilizing the provided TemplatingData.
func (r *textTemplateRenderer) RenderObjects(data *TemplatingData) ([]*unstructured.Unstructured, error) {
	objs := []*unstructured.Unstructured{}
	err := filepath.Walk(r.dirPath, func(path string, info os.FileInfo, err error) error {
		// Error during traversal
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip non [yml, yaml, json] files
		if base := info.Name(); !(strings.HasSuffix(base, ".yml") ||
			strings.HasSuffix(base, ".yaml") ||
			strings.HasSuffix(base, ".json")) {
			return nil
		}

		out, err := r.renderFile(path, data)
		if err != nil {
			return err
		}

		objs = append(objs, out...)
		return nil
	})

	if err != nil {
		return nil, errors.Wrapf(err, "error renedring manifests")
	}
	return objs, nil
}

func (r *textTemplateRenderer) renderFile(filePath string, data *TemplatingData) ([]*unstructured.Unstructured, error) {
	// Read file
	txt, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read manifest file %s", filePath)
	}

	// Create a new template
	tmpl := template.New(path.Base(filePath)).Option("missingkey=error")
	if data.Funcs != nil {
		tmpl.Funcs(data.Funcs)
	}

	if _, err := tmpl.Parse(string(txt)); err != nil {
		return nil, errors.Wrapf(err, "failed to parse manifest file %s", filePath)
	}
	rendered := bytes.Buffer{}

	if err := tmpl.Execute(&rendered, data.Data); err != nil {
		return nil, errors.Wrapf(err, "failed to render manifest %s", filePath)
	}

	out := []*unstructured.Unstructured{}

	// special case - if the entire file is whitespace, skip
	if strings.TrimSpace(rendered.String()) == "" {
		return out, nil
	}

	decoder := yaml.NewYAMLOrJSONDecoder(&rendered, 4096)
	for {
		u := unstructured.Unstructured{}
		if err := decoder.Decode(&u); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "failed to unmarshal manifest %s", filePath)
		}
		// Ensure object is not empty by checking the object kind
		if u.GetKind() == "" {
			continue
		}
		out = append(out, &u)
	}

	return out, nil
}
