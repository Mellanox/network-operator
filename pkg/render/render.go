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

/*
Package render renders k8s API objects from a given set of template .yaml files
provided in a source directory and a RenderData struct to be used in the rendering process

The objects are rendered in `Unstructured` format provided by
k8s.io/apimachinery/pkg/apis/meta/v1/unstructured package.
*/
package render

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	yamlConverter "sigs.k8s.io/yaml"
)

const (
	maxBufSizeForYamlDecode = 4096
)

// ManifestFileSuffix is the collection of suffixes considered when searching for Kubernetes manifest files.
var ManifestFileSuffix = []string{"yaml", "yml", "json"}

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

// NewRenderer creates a Renderer object, that will render all template files provided.
// file format needs to be either json or yaml.
func NewRenderer(files []string) Renderer {
	return &textTemplateRenderer{
		files: files,
	}
}

// textTemplateRenderer is an implementation of the Renderer interface using golang builtin text/template package
// as its templating engine
type textTemplateRenderer struct {
	files []string
}

// RenderObjects renders kubernetes objects utilizing the provided TemplatingData.
func (r *textTemplateRenderer) RenderObjects(data *TemplatingData) ([]*unstructured.Unstructured, error) {
	objs := []*unstructured.Unstructured{}

	for _, file := range r.files {
		out, err := r.renderFile(file, data)
		if err != nil {
			return nil, err
		}
		objs = append(objs, out...)
	}
	return objs, nil
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.Replace(v, "\n", "\n"+pad, -1)
}

func nindent(spaces int, v string) string {
	return "\n" + indent(spaces, v)
}

// nindentPrefix adds a prefix in front of the indented string, left from the initial indentation
func nindentPrefix(spaces int, prefix, v string) string {
	// Remove len(prefix) spaces from the beginning of the indented string
	return strings.Replace(nindent(spaces, prefix+v), " ", "", len(prefix))
}

// imagePath returns container image path, supporting sha256 format
func imagePath(repository, image, version string) string {
	if strings.HasPrefix(version, "sha256:") {
		return repository + "/" + image + "@" + version
	}
	return repository + "/" + image + ":" + version
}

// renderFile renders a single file to a list of k8s unstructured objects
func (r *textTemplateRenderer) renderFile(filePath string, data *TemplatingData) ([]*unstructured.Unstructured, error) {
	// Read file
	txt, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read manifest file %s", filePath)
	}

	// Create a new template
	tmpl := template.New(path.Base(filePath)).Option("missingkey=error")
	tmpl.Funcs(template.FuncMap{
		"yaml": func(obj interface{}) (string, error) {
			yamlBytes, err := yamlConverter.Marshal(obj)
			return string(yamlBytes), err
		},
		"quote": func(obj interface{}) string {
			return fmt.Sprintf("%q", obj)
		},
		"indent":        indent,
		"nindent":       nindent,
		"nindentPrefix": nindentPrefix,
		"hasPrefix":     strings.HasPrefix,
		"imagePath":     imagePath,
	})

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

	decoder := yamlDecoder.NewYAMLOrJSONDecoder(&rendered, maxBufSizeForYamlDecode)
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
