package system_test

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unicode/utf8"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
	"github.com/conga-sh/conga/system"
)

var (
	// System account file payloads in passwd(5) and group(5) format.
	//go:embed testdata/fixtures/group
	fixtureGroup []byte

	//go:embed testdata/fixtures/passwd
	fixturePasswd []byte
)

func TestEnvVars_ContextEnv(t *testing.T) {
	t.Parallel()

	pred := system.EnvVars()
	result := pred.Invoke(predict.Context{
		Env: map[string]string{
			"CONGA_ENV":  "production",
			"CONGA_PORT": "8080",
		},
	})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "CONGA_ENV"))
	assert.True(t, slices.Contains(values, "CONGA_PORT"))

	for _, item := range result.Items {
		switch item.Value {
		case "CONGA_ENV":
			assert.Equal(t, "production", item.Description)
			assert.Equal(t, "environment variables", item.Tag)
			assert.Equal(t, "yellow", item.Style)
		case "CONGA_PORT":
			assert.Equal(t, "8080", item.Description)
		}
	}
}

func TestEnvVars_DollarPrefix(t *testing.T) {
	t.Parallel()

	pred := system.EnvVars()
	result := pred.Invoke(predict.Context{
		ToComplete: "$CONGA_",
		Env: map[string]string{
			"CONGA_MODE": "testing",
		},
	})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "$CONGA_MODE"))
	assert.False(t, slices.Contains(values, "CONGA_MODE"))
}

func TestEnvVars_TruncateDescription(t *testing.T) {
	t.Parallel()

	longValue := strings.Repeat("A", 100)
	pred := system.EnvVars()
	result := pred.Invoke(predict.Context{
		Env: map[string]string{
			"CONGA_LONG": longValue,
		},
	})

	for _, item := range result.Items {
		if item.Value == "CONGA_LONG" {
			assert.True(t, strings.HasSuffix(item.Description, "..."))
			assert.Equal(t, 40, len(item.Description))
		}
	}
}

func TestEnvVars_MultiByteUTF8Truncation(t *testing.T) {
	t.Parallel()

	valueWithUTF8 := strings.Repeat("a", 36) + "€€€€€"
	predictor := system.EnvVars()
	result := predictor.Invoke(predict.Context{
		Env: map[string]string{
			"CONGA_UTF8": valueWithUTF8,
		},
	})

	for _, item := range result.Items {
		if item.Value == "CONGA_UTF8" {
			assert.True(t, utf8.ValidString(item.Description), "description must be valid UTF-8 and not split multi-byte runes")
			assert.Equal(t, strings.Repeat("a", 36)+"€...", item.Description)
		}
	}
}

func TestUsers_Execution(t *testing.T) {
	t.Parallel()

	pred := system.Users()
	result := pred.Invoke(predict.Context{})

	assert.True(t, len(result.Items) > 0, "expected at least one user candidate")

	for _, item := range result.Items {
		assert.Equal(t, "users", item.Tag)
		assert.Equal(t, "blue", item.Style)
		assert.NotZero(t, item.Value)
	}
}

func TestGroups_Execution(t *testing.T) {
	t.Parallel()

	pred := system.Groups()
	result := pred.Invoke(predict.Context{})

	assert.True(t, len(result.Items) > 0, "expected at least one group candidate")

	for _, item := range result.Items {
		assert.Equal(t, "user groups", item.Tag)
		assert.Equal(t, "cyan", item.Style)
		assert.NotZero(t, item.Value)
	}
}

func TestSignals_FullAndShortNames(t *testing.T) {
	t.Parallel()

	pred := system.Signals()
	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "SIGTERM"))
	assert.True(t, slices.Contains(values, "TERM"))
	assert.True(t, slices.Contains(values, "15"))
	assert.True(t, slices.Contains(values, "SIGKILL"))
	assert.True(t, slices.Contains(values, "KILL"))
	assert.True(t, slices.Contains(values, "9"))
	assert.True(t, slices.Contains(values, "SIGINT"))
	assert.True(t, slices.Contains(values, "INT"))
	assert.True(t, slices.Contains(values, "2"))
}

func TestSignals_SeverityStyles(t *testing.T) {
	t.Parallel()

	pred := system.Signals()
	result := pred.Invoke(predict.Context{})

	for _, item := range result.Items {
		assert.Equal(t, "signals", item.Tag)

		switch item.Value {
		case "SIGKILL", "KILL", "9", "SIGQUIT", "SIGABRT", "SIGSEGV":
			assert.Equal(t, "red", item.Style)
		case "SIGTERM", "TERM", "15", "SIGINT", "INT", "2", "SIGHUP":
			assert.Equal(t, "yellow", item.Style)
		case "SIGSTOP", "STOP", "SIGTSTP":
			assert.Equal(t, "magenta", item.Style)
		case "SIGCONT", "CONT":
			assert.Equal(t, "green", item.Style)
		case "SIGUSR1", "USR1", "SIGUSR2", "USR2":
			assert.Equal(t, "blue", item.Style)
		}
	}
}

func TestProcessNames_Scanning(t *testing.T) {
	t.Parallel()

	pred := system.ProcessNames()
	result := pred.Invoke(predict.Context{})

	assert.True(t, len(result.Items) > 0, "expected active processes to be returned")

	for _, item := range result.Items {
		assert.Equal(t, "processes", item.Tag)
		assert.Equal(t, "green", item.Style)
		assert.NotZero(t, item.Value)
		assert.True(t, strings.Contains(item.Description, "instance"))
	}
}

func TestProcessIDs_Scanning(t *testing.T) {
	t.Parallel()

	pred := system.ProcessIDs()
	result := pred.Invoke(predict.Context{})

	assert.True(t, len(result.Items) > 0, "expected active process IDs to be returned")

	currentPid := strconv.Itoa(os.Getpid())
	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, currentPid), "expected current process PID")

	for _, item := range result.Items {
		assert.Equal(t, "pids", item.Tag)
		assert.Equal(t, "magenta", item.Style)
		assert.NotZero(t, item.Value)
	}
}

func TestLocales_Fallback(t *testing.T) {
	t.Parallel()

	pred := system.Locales()
	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "C") || slices.Contains(values, "en_US.UTF-8"))

	for _, item := range result.Items {
		assert.Equal(t, "locales", item.Tag)
		assert.Equal(t, "cyan", item.Style)
	}
}

func TestPasswdAndGroup_Readers(t *testing.T) {
	t.Parallel()

	assert.True(t, len(fixturePasswd) > 0)
	assert.True(t, len(fixtureGroup) > 0)
	assert.True(t, bytes.Contains(fixturePasswd, []byte("alice")))
	assert.True(t, bytes.Contains(fixtureGroup, []byte("sudo")))
}

func TestDomainTypes(t *testing.T) {
	t.Parallel()

	t.Run("EnvVar domain type", func(t *testing.T) {
		t.Parallel()

		var envVar system.EnvVar

		err := envVar.UnmarshalText([]byte("PATH"))

		assert.NoError(t, err)
		assert.Equal(t, "PATH", envVar.String())
		assert.NotZero(t, envVar.Value())

		value, ok := envVar.Lookup()

		assert.True(t, ok)
		assert.NotZero(t, value)
		assert.NotZero(t, envVar.Predict())
	})

	t.Run("User domain type", func(t *testing.T) {
		t.Parallel()

		var testUser system.User

		err := testUser.UnmarshalText([]byte("root"))

		assert.NoError(t, err)
		assert.Equal(t, "root", testUser.String())
		assert.NotZero(t, testUser.Predict())
	})

	t.Run("Group domain type", func(t *testing.T) {
		t.Parallel()

		var testGroup system.Group

		err := testGroup.UnmarshalText([]byte("root"))

		assert.NoError(t, err)
		assert.Equal(t, "root", testGroup.String())
		assert.NotZero(t, testGroup.Predict())
	})

	t.Run("Process domain type", func(t *testing.T) {
		t.Parallel()

		var testProcess system.Process

		err := testProcess.UnmarshalText([]byte("nginx"))

		assert.NoError(t, err)
		assert.Equal(t, "nginx", testProcess.String())
		assert.NotZero(t, testProcess.Predict())
	})

	t.Run("PID domain type", func(t *testing.T) {
		t.Parallel()

		var testPID system.PID

		err := testPID.UnmarshalText([]byte("1234"))

		assert.NoError(t, err)
		assert.Equal(t, 1234, testPID.Int())
		assert.Equal(t, "1234", testPID.String())
		assert.NotZero(t, testPID.Predict())

		var invalidPID system.PID

		err = invalidPID.UnmarshalText([]byte("abc"))

		assert.Error(t, err)

		err = invalidPID.UnmarshalText([]byte("-5"))

		assert.Error(t, err)
	})

	t.Run("Signal domain type", func(t *testing.T) {
		t.Parallel()

		var testSignal system.Signal

		err := testSignal.UnmarshalText([]byte("SIGTERM"))

		assert.NoError(t, err)
		assert.Equal(t, "SIGTERM", testSignal.String())
		assert.Equal(t, syscall.SIGTERM, testSignal.Signal())
		assert.NotZero(t, testSignal.Predict())

		var killSig system.Signal = "9"
		assert.Equal(t, syscall.SIGKILL, killSig.Signal())

		var emptySig system.Signal
		assert.Equal(t, syscall.Signal(0), emptySig.Signal())

		var unknownSig system.Signal = "BOGUS"
		assert.Equal(t, syscall.Signal(0), unknownSig.Signal())

		var invalidSig system.Signal

		err = invalidSig.UnmarshalText([]byte("INVALID_SIGNAL_XYZ"))

		assert.Error(t, err)
	})

	t.Run("Locale domain type", func(t *testing.T) {
		t.Parallel()

		var testLocale system.Locale

		err := testLocale.UnmarshalText([]byte("en_US.UTF-8"))

		assert.NoError(t, err)
		assert.Equal(t, "en_US.UTF-8", testLocale.String())
		assert.NotZero(t, testLocale.Predict())
	})
}

func TestDomainTypes_MarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"EnvVar", system.EnvVar("PATH"), "PATH"},
		{"Group", system.Group("wheel"), "wheel"},
		{"Locale", system.Locale("en_US.UTF-8"), "en_US.UTF-8"},
		{"PID", system.PID(1234), "1234"},
		{"Process", system.Process("nginx"), "nginx"},
		{"Signal", system.Signal("SIGTERM"), "SIGTERM"},
		{"User", system.User("root"), "root"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			marshaler, ok := testCase.input.(interface{ MarshalText() ([]byte, error) })
			assert.True(t, ok)

			encoded, err := marshaler.MarshalText()
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, string(encoded))
		})
	}
}

type systemChainingCLI struct {
	PID     conga.OptionalFlag[system.PID]     `short:"p"`
	Env     conga.OptionalFlag[system.EnvVar]  `short:"e"`
	Signal  conga.OptionalFlag[system.Signal]  `short:"s"`
	User    conga.OptionalFlag[system.User]    `short:"u"`
	Group   conga.OptionalFlag[system.Group]   `short:"g"`
	Locale  conga.OptionalFlag[system.Locale]  `short:"l"`
	Process conga.OptionalFlag[system.Process] `short:"r"`
}

func (c *systemChainingCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestDomainTypes_ValueReceiverChaining(t *testing.T) {
	t.Parallel()

	cli := &systemChainingCLI{}
	app := conga.New(cli).Args([]string{
		"-p", "1234",
		"-e", "PATH",
		"-s", "SIGTERM",
		"-u", "root",
		"-g", "wheel",
		"-l", "en_US.UTF-8",
		"-r", "nginx",
	})

	err := app.Run()

	assert.NoError(t, err)

	assert.Equal(t, 1234, cli.PID.Get().Int())
	assert.Equal(t, "1234", cli.PID.Get().String())
	assert.Equal(t, "PATH", cli.Env.Get().String())
	assert.NotZero(t, cli.Env.Get().Value())
	assert.Equal(t, "SIGTERM", cli.Signal.Get().String())
	assert.Equal(t, syscall.SIGTERM, cli.Signal.Get().Signal())
	assert.Equal(t, "root", cli.User.Get().String())
	assert.Equal(t, "wheel", cli.Group.Get().String())
	assert.Equal(t, "en_US.UTF-8", cli.Locale.Get().String())
	assert.Equal(t, "nginx", cli.Process.Get().String())
}

func TestPIDs_AliasesProcessIDs(t *testing.T) {
	t.Parallel()

	aliasItems := system.PIDs().Invoke(predict.Context{}).Items
	canonicalItems := system.ProcessIDs().Invoke(predict.Context{}).Items

	assert.Equal(t, itemValues(canonicalItems), itemValues(aliasItems))
}

func TestProcesses_AliasesProcessNames(t *testing.T) {
	t.Parallel()

	aliasItems := system.Processes().Invoke(predict.Context{}).Items
	canonicalItems := system.ProcessNames().Invoke(predict.Context{}).Items

	assert.Equal(t, itemValues(canonicalItems), itemValues(aliasItems))
}

func itemValues(items []predict.Item) []string {
	return slicex.Map(items, func(item predict.Item) string { return item.Value })
}
