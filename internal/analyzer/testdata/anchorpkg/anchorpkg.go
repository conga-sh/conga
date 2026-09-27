// Package anchorpkg anchors a CLI whose command structs live in libpkg,
// exercising cross-package hierarchy traversal and cross-package diagnostic
// positioning.
package anchorpkg

import (
	"github.com/conga-sh/conga"

	"congalint.test/fixtures/libpkg"
)

var _ = conga.New[libpkg.Root]() // want `leaf command "bad" \(struct BadLeaf\) does not implement conga\.Runner`
