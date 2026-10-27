package conga

import "github.com/conga-sh/conga/internal/why"

// WhyOption configures the built-in --why provenance reporting engine on [App.EnableWhy].
//
// WhyOption is a sealed interface: values are produced exclusively by CONGA
// option constructors such as [WhyOptionJSON], and cannot be implemented by other packages.
type WhyOption interface {
	applyWhy(settings *whySettings)
}

// WhyOptionJSON enables machine-readable JSON output (--why=json) on [App.EnableWhy].
//
// By default, [App.EnableWhy] compiles only the human-readable text table
// renderer, keeping the binary footprint lean and dead-code eliminating the
// RFC 8259 JSON serialization pipeline. Pass [WhyOptionJSON] to opt into machine-readable
// provenance reports.
func WhyOptionJSON() WhyOption {
	return whyOptionFunc(func(s *whySettings) {
		s.jsonFormatter = why.FormatJSONReport
		s.rowType = why.FlagType
	})
}

type whyOptionFunc func(*whySettings)

func (f whyOptionFunc) applyWhy(s *whySettings) { f(s) }

type whySettings struct {
	jsonFormatter whyJSONFormatterFunc
	rowType       whyRowTypeFunc
}
