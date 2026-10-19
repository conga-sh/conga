package conga

// WhyOption configures the built-in --why provenance reporting engine on [App.EnableWhy].
//
// WhyOption is a sealed interface: values are produced exclusively by CONGA
// option constructors such as [WhyJSON], and cannot be implemented by other packages.
type WhyOption interface {
	applyWhy(settings *whySettings)
}

// WhyJSON enables machine-readable JSON output (--why=json) on [App.EnableWhy].
//
// By default, [App.EnableWhy] compiles only the human-readable text table
// renderer, keeping the binary footprint lean and dead-code eliminating the
// RFC 8259 JSON serialization pipeline. Pass WhyJSON to opt into machine-readable
// provenance reports.
func WhyJSON() WhyOption {
	return whyOptionFunc(func(s *whySettings) {
		s.jsonFormatter = formatWhyJSON
		s.rowType = whyFlagType
	})
}

type whyOptionFunc func(*whySettings)

func (f whyOptionFunc) applyWhy(s *whySettings) { f(s) }

type whySettings struct {
	jsonFormatter whyJSONFormatterFunc
	rowType       whyRowTypeFunc
}
