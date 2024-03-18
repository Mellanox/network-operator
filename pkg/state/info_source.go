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

package state

import (
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

// InfoType is used to categorize an InfoSource.
type InfoType uint

const (
	// InfoTypeNodeInfo describes an InfoSource related to a node.
	InfoTypeNodeInfo = iota
	// InfoTypeClusterType describes an InfoSource related to a cluster.
	InfoTypeClusterType
	// InfoTypeStaticConfig describes an InfoSource related to a static configuration.
	InfoTypeStaticConfig
	// InfoTypeDocaDriverImage describes an InfoSource related to DOCA Drivers images
	InfoTypeDocaDriverImage
)

// NewInfoCatalog returns an initialized InfoCatalog.
func NewInfoCatalog() InfoCatalog {
	return &infoCatalog{infoSources: make(map[InfoType]InfoSource)}
}

// InfoSource represents an object that is a source of information.
type InfoSource interface{}

// InfoCatalog is an information catalog to be used to retrieve infoSources. used for State implementation that require
// additional helping functionality to perfrom the Sync operation. As more States are added,
// more infoSources may be added to aid them. for any infoSource if not present in the catalog, nil will be returned.
type InfoCatalog interface {
	// Add an infoSource of InfoType to the catalog
	Add(InfoType, InfoSource)
	// GetNodeInfoProvider returns a reference nodeinfo.Provider from catalog or nil if provider does not exist
	GetNodeInfoProvider() nodeinfo.Provider
	// GetClusterTypeProvider returns a reference clustertype.Provider from catalog or nil if provider does not exist
	GetClusterTypeProvider() clustertype.Provider
	// GetStaticConfigProvider returns a reference staticinfo.Provider from catalog or nil if provider does not exist
	GetStaticConfigProvider() staticconfig.Provider
	// GetDocaDriverImageProvider returns a reference docadriverimages.Provider from catalog
	// or nil if provider does not exist
	GetDocaDriverImageProvider() docadriverimages.Provider
}

type infoCatalog struct {
	infoSources map[InfoType]InfoSource
}

func (sc *infoCatalog) Add(infoType InfoType, infoSource InfoSource) {
	sc.infoSources[infoType] = infoSource
}

func (sc *infoCatalog) GetNodeInfoProvider() nodeinfo.Provider {
	infoSource, ok := sc.infoSources[InfoTypeNodeInfo]
	if !ok {
		return nil
	}
	return infoSource.(nodeinfo.Provider)
}

func (sc *infoCatalog) GetClusterTypeProvider() clustertype.Provider {
	infoSource, ok := sc.infoSources[InfoTypeClusterType]
	if !ok {
		return nil
	}
	return infoSource.(clustertype.Provider)
}

func (sc *infoCatalog) GetStaticConfigProvider() staticconfig.Provider {
	infoSource, ok := sc.infoSources[InfoTypeStaticConfig]
	if !ok {
		return nil
	}
	return infoSource.(staticconfig.Provider)
}

func (sc *infoCatalog) GetDocaDriverImageProvider() docadriverimages.Provider {
	infoSource, ok := sc.infoSources[InfoTypeDocaDriverImage]
	if !ok {
		return nil
	}
	return infoSource.(docadriverimages.Provider)
}
