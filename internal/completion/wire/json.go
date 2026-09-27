package wire

import (
	"github.com/conga-sh/conga/internal/jsonwire"
)

// AppendJSONString appends the RFC 8259 JSON string literal for input
// (including the surrounding quotes) to dst and returns the extended buffer.
//
// It delegates to jsonwire.AppendString so the completion wire protocol and
// the version report share one escaper without importing encoding/json
// (ADR 005 dependency boundaries).
func AppendJSONString(dst []byte, input string) []byte {
	return jsonwire.AppendString(dst, input)
}
