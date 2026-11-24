package sentinel_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/sentinel"
)

func TestErrRedirectTarget_Message(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "bash completion: current position is a redirect target", sentinel.ErrRedirectTarget.Error())
}
