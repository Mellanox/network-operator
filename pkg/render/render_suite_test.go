package render_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRender(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Render Test Suite")
}
