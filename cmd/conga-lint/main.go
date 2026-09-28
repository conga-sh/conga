// Command conga-lint runs CONGA's static schema analyzer over Go packages.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/conga-sh/conga/internal/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
