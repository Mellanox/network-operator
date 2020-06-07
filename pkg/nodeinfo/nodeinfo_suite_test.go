package nodeinfo_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNodeInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "nodeinfo test Suite")
}
