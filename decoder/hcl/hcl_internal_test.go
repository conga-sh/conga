package hcl

import (
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/hashicorp/hcl/v2"
)

func TestDiagnosticsError_Rendering(t *testing.T) {
	t.Parallel()

	subject := hcl.Range{Start: hcl.Pos{Line: 2, Column: 4}, End: hcl.Pos{Line: 2, Column: 9}}

	t.Run("the first error names its position, summary and detail", func(t *testing.T) {
		t.Parallel()

		err := &diagnosticsError{diagnostics: hcl.Diagnostics{
			{Severity: hcl.DiagWarning, Summary: "Deprecated", Detail: "a warning", Subject: &subject},
			{Severity: hcl.DiagError, Summary: "Invalid expression", Detail: "Expected a value.", Subject: &subject},
		}}

		assert.Equal(t, "line 2, column 4: Invalid expression", err.Error())
		assert.Equal(t, "Expected a value.", err.Note())
	})

	t.Run("further errors are counted on line 1", func(t *testing.T) {
		t.Parallel()

		err := &diagnosticsError{diagnostics: hcl.Diagnostics{
			{Severity: hcl.DiagError, Summary: "Invalid expression", Detail: "Expected a value.", Subject: &subject},
			{Severity: hcl.DiagError, Summary: "Missing expression", Detail: "Expected the end."},
			{Severity: hcl.DiagError, Summary: "Unclosed block", Detail: "No closing brace."},
		}}

		assert.Equal(t, "line 2, column 4: Invalid expression, and 2 other diagnostic(s)", err.Error())
		assert.Equal(t, "Expected a value.", err.Note())
	})

	t.Run("an error without a subject has no position", func(t *testing.T) {
		t.Parallel()

		err := &diagnosticsError{diagnostics: hcl.Diagnostics{
			{Severity: hcl.DiagError, Summary: "Invalid body", Detail: "The body is empty."},
		}}

		assert.Equal(t, "Invalid body", err.Error())
		assert.Equal(t, "The body is empty.", err.Note())
	})

	t.Run("without an error the first diagnostic is reported", func(t *testing.T) {
		t.Parallel()

		err := &diagnosticsError{diagnostics: hcl.Diagnostics{
			{Severity: hcl.DiagWarning, Summary: "Deprecated", Detail: "a warning", Subject: &subject},
		}}

		assert.Equal(t, "line 2, column 4: Deprecated", err.Error())
		assert.Equal(t, "a warning", err.Note())
	})

	t.Run("an empty list keeps the library's text", func(t *testing.T) {
		t.Parallel()

		err := &diagnosticsError{}

		assert.Equal(t, (hcl.Diagnostics{}).Error(), err.Error())
		assert.Equal(t, "", err.Note())
	})
}
