package builderorderinvalid

import (
	"testing"

	"github.com/conga-sh/conga"
)

// TestSkippedTestFiles locks the contract that _test.go files are exempt from
// the builder call order check: the deliberately inverted chain below carries
// no expectation.
func TestSkippedTestFiles(t *testing.T) {
	t.Parallel()

	app := conga.New[CLI]().
		Version("1.0.0").
		Name("skipped-test-file")

	_ = app
}
