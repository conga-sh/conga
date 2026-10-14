// Command conga-lint runs CONGA's static schema analyzer over Go packages.
//
// Building and running conga-lint requires a Go 1.27+ toolchain so method-level
// generic type parameters are supported at parse time.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/conga-sh/conga/internal/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
