package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestCommandIsBuiltin(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		command *spec.Command
		want    bool
	}{
		{name: "help", command: &spec.Command{Name: "help", IsHelpCmd: true}, want: true},
		{name: "version", command: &spec.Command{Name: "version", IsVersionCmd: true}, want: true},
		{name: "completion", command: &spec.Command{Name: "completion", IsCompletionCmd: true}, want: true},
		{name: "user", command: &spec.Command{Name: "help"}, want: false},
		{name: "nil", command: nil, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.command.IsBuiltin())
		})
	}
}
