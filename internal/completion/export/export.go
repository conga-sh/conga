// Package export renders completion results as single-line JSON: the
// machine-readable test surface and unknown-shell fallback framing of the
// wire protocol (docs/completion-wire-protocol.md).
//
// The powershell framing shares this emitter with an additional
// per-candidate "sgr" field; Marshal exposes the corresponding extension
// point.
package export

import (
	"strconv"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// StyleResolver renders the optional per-candidate "sgr" field: the ECMA-48
// SGR parameter numbers translated from the candidate's style tokens. It
// returns "" when the candidate carries no style (powershell's translation,
// Phase 5).
type StyleResolver func(candidate wire.Candidate) string

// Format renders candidates and directives in the export framing: a
// single-line JSON object (no trailing newline) with candidates in engine
// order, the always-complete directives object, and the usage string. The
// framing is position-independent: toComplete is accepted for seam symmetry
// and ignored.
func Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	return Marshal(directives, candidates, nil)
}

// Marshal renders the export schema with an optional per-candidate "sgr"
// field produced by resolve (powershell's style translation). A nil resolver
// omits the field entirely, yielding the plain export framing.
func Marshal(directives wire.Directives, candidates []wire.Candidate, resolve StyleResolver) string {
	buffer := make([]byte, 0, 64*len(candidates)+128)

	buffer = append(buffer, `{"candidates":[`...)

	for i, candidate := range candidates {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendCandidate(buffer, candidate, resolve)
	}

	buffer = append(buffer, `],"directives":{"keeporder":`...)
	buffer = strconv.AppendBool(buffer, directives.KeepOrder)
	buffer = append(buffer, `,"messages":[`...)

	for i, message := range directives.Messages {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = wire.AppendJSONString(buffer, message)
	}

	buffer = append(buffer, `],"noprefix":`...)
	buffer = strconv.AppendBool(buffer, !directives.NoPrefix.IsEmpty())
	buffer = append(buffer, `,"nospace":`...)
	buffer = strconv.AppendBool(buffer, !directives.NoSpace.IsEmpty())
	buffer = append(buffer, `},"usage":`...)
	buffer = wire.AppendJSONString(buffer, directives.Usage)
	buffer = append(buffer, '}')

	return string(buffer)
}

// appendCandidate appends one candidate object: value and display always
// present, description/style/tag omitted when empty, and the optional "sgr"
// field appended last when the resolver yields a non-empty rendering.
func appendCandidate(buffer []byte, candidate wire.Candidate, resolve StyleResolver) []byte {
	buffer = append(buffer, `{"value":`...)
	buffer = wire.AppendJSONString(buffer, candidate.Value)
	buffer = append(buffer, `,"display":`...)
	buffer = wire.AppendJSONString(buffer, candidate.Display)

	if candidate.Description != "" {
		buffer = append(buffer, `,"description":`...)
		buffer = wire.AppendJSONString(buffer, candidate.Description)
	}

	if candidate.Style != "" {
		buffer = append(buffer, `,"style":`...)
		buffer = wire.AppendJSONString(buffer, candidate.Style)
	}

	if candidate.Tag != "" {
		buffer = append(buffer, `,"tag":`...)
		buffer = wire.AppendJSONString(buffer, candidate.Tag)
	}

	if resolve != nil {
		if sgr := resolve(candidate); sgr != "" {
			buffer = append(buffer, `,"sgr":`...)
			buffer = wire.AppendJSONString(buffer, sgr)
		}
	}

	return append(buffer, '}')
}
