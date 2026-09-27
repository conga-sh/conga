package appname_test

import (
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/appname"
)

//nolint:paralleltest // Mutates global os.Args for invocation name testing.
func TestBinary(t *testing.T) {
	tests := []struct {
		name     string
		argv     []string
		wantName string
		wantOK   bool
	}{
		{
			name:     "plain binary",
			argv:     []string{"/usr/local/bin/foobar"},
			wantName: "foobar",
			wantOK:   true,
		},
		{
			name:     "windows exe stripped",
			argv:     []string{"/usr/local/bin/foobar.exe"},
			wantName: "foobar",
			wantOK:   true,
		},
		{
			name:     "test runner ignored",
			argv:     []string{"/tmp/go-build/foo/conga.test"},
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "empty argv0",
			argv:     []string{""},
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "empty argv slice",
			argv:     []string{},
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "exe only name",
			argv:     []string{".exe"},
			wantName: "",
			wantOK:   false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()

			os.Args = testCase.argv

			gotName, gotOK := appname.Binary()

			assert.Equal(t, testCase.wantOK, gotOK)
			assert.Equal(t, testCase.wantName, gotName)
		})
	}
}
