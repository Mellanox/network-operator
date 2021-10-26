module github.com/Mellanox/network-operator

go 1.15

require (
	github.com/caarlos0/env/v6 v6.4.0
	github.com/go-logr/logr v0.3.0
	github.com/googleapis/gnostic v0.5.3 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.6.1
	github.com/vektra/mockery/v2 v2.7.4 // indirect
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.1
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
