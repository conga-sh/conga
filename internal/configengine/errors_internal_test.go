package configengine

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/errlines"
)

// pieceCause is a decoder error that supplies origins and a hint.
type pieceCause struct {
	origins []string
	hint    string
}

func (e *pieceCause) Error() string     { return "decode failed" }
func (e *pieceCause) Origins() []string { return e.origins }
func (e *pieceCause) Hint() string      { return e.hint }

// originsHinter mirrors conga.Originer and conga.Hinter, which this package
// cannot import.
type originsHinter interface {
	error
	Origins() []string
	Hint() string
}

func TestConfigError_PiecesReachTheCause(t *testing.T) {
	t.Parallel()

	cause := &pieceCause{origins: []string{"config.hcl"}, hint: "use an unlabeled block"}

	tests := []struct {
		name        string
		err         *configError
		wantOrigins []string
		wantHint    string
	}{
		{
			name:        "own values win",
			err:         &configError{msg: "load failed", origin: "config file \"own.hcl\"", hint: "own hint", cause: cause},
			wantOrigins: []string{`config file "own.hcl"`},
			wantHint:    "own hint",
		},
		{
			name:        "own empty, the cause supplies them",
			err:         &configError{msg: "load failed", cause: cause},
			wantOrigins: []string{"config.hcl"},
			wantHint:    "use an unlabeled block",
		},
		{
			name:        "own origin only, the cause supplies the hint",
			err:         &configError{msg: "load failed", origin: "file", cause: cause},
			wantOrigins: []string{"file"},
			wantHint:    "use an unlabeled block",
		},
		{
			name: "both empty",
			err:  &configError{msg: "load failed", cause: errors.New("plain")},
		},
		{
			name: "no cause",
			err:  &configError{msg: "load failed"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.wantOrigins, test.err.Origins())
			assert.Equal(t, test.wantHint, test.err.Hint())

			found, ok := errors.AsType[originsHinter](test.err)
			assert.True(t, ok)
			assert.Equal(t, test.wantOrigins, found.Origins())
			assert.Equal(t, test.wantHint, found.Hint())

			assert.Equal(t, test.wantOrigins, errlines.Origins(test.err))
			assert.Equal(t, test.wantHint, errlines.Hint(test.err))
		})
	}
}

func TestConfigError_Error(t *testing.T) {
	t.Parallel()

	t.Run("explicit msg takes precedence", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			path:  "config.yaml",
			msg:   "custom error message",
			cause: errors.New("underlying cause"),
		}

		msg := err.Error()

		assert.Equal(t, "custom error message", msg)
	})

	t.Run("formats path and cause when msg is empty", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			path:  "app.json",
			cause: errors.New("permission denied"),
		}

		msg := err.Error()

		assert.Equal(t, `config file "app.json": permission denied`, msg)
	})

	t.Run("formats path when only path provided", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			path: "app.json",
		}

		msg := err.Error()

		assert.Equal(t, `config file "app.json" error`, msg)
	})

	t.Run("returns cause error when only cause provided", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			cause: errors.New("read failed"),
		}

		msg := err.Error()

		assert.Equal(t, "read failed", msg)
	})

	t.Run("returns generic error when all fields empty", func(t *testing.T) {
		t.Parallel()

		err := &configError{}

		msg := err.Error()

		assert.Equal(t, "config error", msg)
	})
}

func TestConfigError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("read failed")

	assert.Equal(t, cause, (&configError{path: "app.json", cause: cause}).Unwrap())
	assert.Equal(t, nil, (&configError{path: "app.json"}).Unwrap())
}
