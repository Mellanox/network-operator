package state

import (
	"reflect"

	"github.com/Mellanox/mellanox-network-operator/pkg/nodeinfo"
)

type ServiceType uint

const (
	ServiceTypeNodeInfo = iota
)

func NewServiceCatalog() ServiceCatalog {
	return &serviceCatalog{services: make(map[ServiceType]interface{})}
}

// ServiceCatalog is a service catalog to be used to retrieve services. used for State implementation that require
// additional helping functionality to perfrom the Sync operation. As more States are added,
// more services may be added to aid them. for any service if not present in the catalog, nil will be returned.
type ServiceCatalog interface {
	// Add a service of ServiceType to the catalog, the service added MUST be of pointer type.
	Add(ServiceType, interface{})
	// GetNodeInfoProvider returns a reference nodeinfo.Provider from catalog or nil if provider does not exist
	GetNodeInfoProvider() nodeinfo.Provider
}

type serviceCatalog struct {
	services map[ServiceType]interface{}
}

func (sc *serviceCatalog) Add(st ServiceType, service interface{}) {
	if reflect.ValueOf(service).Kind() != reflect.Ptr {
		// We should not get here
		panic("Invalid interface provided to ServiceCatalog.Add()")
	}
	sc.services[st] = service
}

func (sc *serviceCatalog) GetNodeInfoProvider() nodeinfo.Provider {
	service, ok := sc.services[ServiceTypeNodeInfo]
	if !ok {
		return nil
	}
	return service.(nodeinfo.Provider)
}
