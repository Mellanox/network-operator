package state

import (
	"github.com/Mellanox/mellanox-network-operator/pkg/nodeinfo"
)

type InfoType uint

const (
	InfoTypeNodeInfo = iota
)

func NewInfoCatalog() InfoCatalog {
	return &infoCatalog{infoSources: make(map[InfoType]InfoSource)}
}

// InfoSource represents an object that is a souce of information
type InfoSource interface{}

// InfoCatalog is an information catalog to be used to retrieve infoSources. used for State implementation that require
// additional helping functionality to perfrom the Sync operation. As more States are added,
// more infoSources may be added to aid them. for any infoSource if not present in the catalog, nil will be returned.
type InfoCatalog interface {
	// Add an infoSource of InfoType to the catalog
	Add(InfoType, InfoSource)
	// GetNodeInfoProvider returns a reference nodeinfo.Provider from catalog or nil if provider does not exist
	GetNodeInfoProvider() nodeinfo.Provider
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
