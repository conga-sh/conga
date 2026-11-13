package style

import (
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestLsColorsStyle(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		sgr  string
		want string
	}{
		{name: "attribute and color", sgr: "01;36", want: "bold cyan"},
		{name: "every attribute", sgr: "1;2;3;4;5;7", want: "bold dim italic underlined blink inverse"},
		{name: "standard colors", sgr: "30;37", want: "black white"},
		{name: "bright colors", sgr: "90;97", want: "bright-black bright-white"},
		{name: "backgrounds", sgr: "40;47;100;107", want: "bg-black bg-white bg-bright-black bg-bright-white"},
		{name: "defaults", sgr: "39;49", want: "fg-default bg-default"},
		{name: "zero-padded code", sgr: "044", want: "bg-blue"},
		{name: "indexed foreground", sgr: "38;5;208", want: "color208"},
		{name: "zero-padded indexed foreground", sgr: "38;05;208", want: "color208"},
		{name: "indexed background", sgr: "48;5;17", want: "bg-color17"},
		{name: "truecolor foreground", sgr: "38;2;1;2;3", want: "#010203"},
		{name: "zero-padded truecolor foreground", sgr: "38;02;1;2;3", want: "#010203"},
		{name: "truecolor background", sgr: "48;2;255;128;0", want: "bg-#ff8000"},
		{name: "extended color then attribute", sgr: "38;5;208;1", want: "color208 bold"},
		{name: "leading reset", sgr: "00;34", want: "blue"},
		{name: "mid-sequence reset clears earlier codes", sgr: "01;31;0;32", want: "green"},
		{name: "reset only renders plain", sgr: "00", want: "fg-default"},
		{name: "unsupported codes only render plain", sgr: "08", want: "fg-default"},
		{name: "concealed and crossed out dropped", sgr: "8;9;31", want: "red"},
		{name: "out-of-range index dropped", sgr: "38;5;256;32", want: "green"},
		{name: "truncated extended color dropped", sgr: "1;38;5", want: "bold"},
		{name: "unknown extended form drops the rest", sgr: "1;38;7;33", want: "bold"},
		{name: "malformed extended form drops the rest", sgr: "1;38;x;31", want: "bold"},
		{name: "malformed code dropped", sgr: "x;35", want: "magenta"},
		{name: "empty renders plain", sgr: "", want: "fg-default"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, lsColorsStyle(test.sgr))
		})
	}
}

// TestLsColorsStyleRoundTrip keeps lsColorsStyle (SGR code to token) and
// Style.SGR (token to SGR code) in agreement: every code lsColorsStyle
// translates must render back to the same code.
func TestLsColorsStyleRoundTrip(t *testing.T) {
	t.Parallel()

	codes := []string{"1", "2", "3", "4", "5", "7", "39", "49", "38;5;208", "48;5;17", "38;2;1;2;3", "48;2;255;128;0"}
	for code := 30; code <= 37; code++ {
		codes = append(codes, strconv.Itoa(code), strconv.Itoa(code+10), strconv.Itoa(code+60), strconv.Itoa(code+70))
	}

	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, code, Parse(lsColorsStyle(code)).SGR())
		})
	}
}

func TestParseLsColorsLastSuffixEntryWins(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*.tar.gz=32:*.gz=31")

	matched, ok := colors.suffixStyle("x.tar.gz")

	assert.True(t, ok)
	assert.Equal(t, "red", matched)
}

func TestParseLsColorsLaterDuplicateOverrides(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*.gz=31:*.tar.gz=32")

	matched, ok := colors.suffixStyle("x.tar.gz")

	assert.True(t, ok)
	assert.Equal(t, "green", matched)
}

func TestParseLsColorsSuffixWithoutDot(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*README=35:*~=2")

	readme, readmeOK := colors.suffixStyle("docs/readme")
	backup, backupOK := colors.suffixStyle("notes.txt~")

	assert.True(t, readmeOK)
	assert.Equal(t, "magenta", readme)
	assert.True(t, backupOK)
	assert.Equal(t, "dim", backup)
}

func TestParseLsColorsLaterDuplicateExtensionOverrides(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*.go=31:*.GO=32")

	matched, ok := colors.suffixStyle("main.go")

	assert.True(t, ok)
	assert.Equal(t, "green", matched)
}

func TestParseLsColorsSuffixAfterExtensionWins(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*.go=31:*o=32")

	matched, ok := colors.suffixStyle("main.go")

	assert.True(t, ok)
	assert.Equal(t, "green", matched)
}

func TestParseLsColorsExtensionAfterSuffixWins(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*o=32:*.go=31")

	matched, ok := colors.suffixStyle("main.go")

	assert.True(t, ok)
	assert.Equal(t, "red", matched)
}

func TestParseLsColorsExtensionMatchesDotfile(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*.bashrc=33")

	matched, ok := colors.suffixStyle(".bashrc")

	assert.True(t, ok)
	assert.Equal(t, "yellow", matched)
}

func TestParseLsColorsBareStarIgnored(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("*=31")

	_, ok := colors.suffixStyle("anything")

	assert.False(t, ok)
}

func TestLsColorsKindDefaults(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("")

	assert.Equal(t, "bold blue", colors.directoryStyle())
	assert.Equal(t, Green, colors.executableStyle())
	assert.Equal(t, Cyan, colors.symlinkStyle())
	assert.Equal(t, Cyan, colors.orphanSymlinkStyle(), "an orphan without an or entry takes the link style")
	assert.False(t, colors.symlinkAsTarget)
}

func TestLsColorsKindEntries(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("di=00:ex=01;32:ln=01;36:fi=31:pi=33")

	assert.Equal(t, "fg-default", colors.directoryStyle(), "di=00 renders plain over the bold blue default")
	assert.Equal(t, "bold green", colors.executableStyle())
	assert.Equal(t, "bold cyan", colors.symlinkStyle())
	assert.Equal(t, "bold cyan", colors.orphanSymlinkStyle(), "an orphan without an or entry takes the ln entry")
}

func TestLsColorsSymlinkAsTarget(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("ln=target")

	assert.True(t, colors.symlinkAsTarget)
	assert.Equal(t, "fg-default", colors.orphanSymlinkStyle(), "a missing target leaves no style to borrow, so the link renders plain")
}

func TestLsColorsSymlinkAsTargetWithOrphanEntry(t *testing.T) {
	t.Parallel()

	colors := parseLsColors("ln=target:or=01;31")

	assert.Equal(t, "bold red", colors.orphanSymlinkStyle())
}
