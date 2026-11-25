package configengine

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestConfigError_OriginsAndHint(t *testing.T) {
	t.Parallel()

	t.Run("a strict-validation error carries its origin and hint", func(t *testing.T) {
		t.Parallel()

		err := newFieldConfigError(`config file "tool.yaml"`, `unknown field "prot"`).withHint(`did you mean "port"?`)

		assert.Equal(t, `unknown field "prot"`, err.Error())
		assert.Equal(t, []string{`config file "tool.yaml"`}, err.Origins())
		assert.Equal(t, `did you mean "port"?`, err.Hint())
	})

	t.Run("a strict-validation error with no origin has no origin line", func(t *testing.T) {
		t.Parallel()

		err := newFieldConfigError("", `unknown field "prot"`)

		assert.Zero(t, err.Origins())
		assert.Equal(t, "", err.Hint())
	})

	t.Run("an error from finding or loading files names its file in the message", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("permission denied")
		err := &configError{path: "tool.yaml", msg: `cannot read config file "tool.yaml": permission denied`, cause: cause}

		assert.Zero(t, err.Origins())
		assert.Equal(t, "", err.Hint())
		assert.Equal(t, cause, err.Unwrap())
	})
}
