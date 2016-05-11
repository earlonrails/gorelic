package gorelic

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGorelic(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gorelic Suite")
}
