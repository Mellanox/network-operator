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

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
)

// GetFilesWithSuffix returns all files under a given base directory that have a specific suffix
// The operation is performed recurively on sub directories as well
func GetFilesWithSuffix(baseDir string, suffixes ...string) ([]string, error) {
	var files []string
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		// Error during traversal
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip non suffix files
		base := info.Name()
		for _, s := range suffixes {
			if strings.HasSuffix(base, s) {
				files = append(files, path)
			}
		}
		return nil
	})

	if err != nil {
		return nil, errors.Wrapf(err, "error traversing directory tree")
	}
	return files, nil
}

func GetNetworkAttachmentDefLink(netAttDef *netattdefv1.NetworkAttachmentDefinition) (link string) {
	link = fmt.Sprintf("%s/namespaces/%s/%s/%s",
		netAttDef.APIVersion, netAttDef.Namespace, netAttDef.Kind, netAttDef.Name)
	return
}
