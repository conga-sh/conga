package system

import (
	"bytes"
	"os/user"
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/predict"
)

var (
	// System account file payloads in passwd(5) and group(5) format.
	//go:embed testdata/fixtures/group
	fixtureGroup []byte

	//go:embed testdata/fixtures/group_malformed
	fixtureGroupMalformed []byte

	//go:embed testdata/fixtures/passwd
	fixturePasswd []byte

	//go:embed testdata/fixtures/passwd_malformed
	fixturePasswdMalformed []byte
)

func TestFallbackLocales(t *testing.T) {
	t.Parallel()

	locales := fallbackLocales()

	assert.True(t, len(locales) > 0)
	assert.True(t, slices.Contains(locales, "en_US.UTF-8"))
	assert.True(t, slices.Contains(locales, "C"))
	assert.True(t, slices.Contains(locales, "POSIX"))
}

func TestParsePasswdReader(t *testing.T) {
	t.Parallel()

	items := parsePasswdReader(bytes.NewReader(fixturePasswd))

	assert.Equal(t, 7, len(items))

	aliceIndex := slices.IndexFunc(items, func(item predict.Item) bool { return item.Value == "alice" })
	assert.True(t, aliceIndex >= 0)
	assert.Equal(t, "Alice Smith", items[aliceIndex].Description)
	assert.Equal(t, "users", items[aliceIndex].Tag)

	charlieIndex := slices.IndexFunc(items, func(item predict.Item) bool { return item.Value == "charlie" })
	assert.True(t, charlieIndex >= 0)
	assert.Equal(t, "/home/charlie", items[charlieIndex].Description)
}

func TestParsePasswdReader_MalformedAndDuplicateLinesSkipped(t *testing.T) {
	t.Parallel()

	items := parsePasswdReader(bytes.NewReader(fixturePasswdMalformed))

	assert.Equal(t, 1, len(items))
	assert.Equal(t, "root", items[0].Value)
}

func TestParseGroupReader(t *testing.T) {
	t.Parallel()

	items := parseGroupReader(bytes.NewReader(fixtureGroup))

	assert.Equal(t, 8, len(items))

	sudoIndex := slices.IndexFunc(items, func(item predict.Item) bool { return item.Value == "sudo" })
	assert.True(t, sudoIndex >= 0)
	assert.Equal(t, "GID: 27 (alice,bob)", items[sudoIndex].Description)

	dockerIndex := slices.IndexFunc(items, func(item predict.Item) bool { return item.Value == "docker" })
	assert.True(t, dockerIndex >= 0)
	assert.Equal(t, "GID: 999 (alice)", items[dockerIndex].Description)
}

func TestParseGroupReader_MalformedLinesSkipped(t *testing.T) {
	t.Parallel()

	items := parseGroupReader(bytes.NewReader(fixtureGroupMalformed))

	assert.Equal(t, 2, len(items))
	assert.Equal(t, "root", items[0].Value)
	assert.Equal(t, "GID: 0", items[0].Description)
	assert.Equal(t, "wheel", items[1].Value)
}

func TestUserAndGroup_Lookup(t *testing.T) {
	t.Parallel()

	t.Run("empty user rejected", func(t *testing.T) {
		t.Parallel()

		empty := User("")

		_, err := empty.Lookup()

		assert.Error(t, err)
		assert.IsError(t, err, errEmptyUser)
	})

	t.Run("unknown user wrapped with context", func(t *testing.T) {
		t.Parallel()

		missing := User("no-such-user-conga-test")

		_, err := missing.Lookup()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user lookup")
	})

	t.Run("current user resolved", func(t *testing.T) {
		t.Parallel()

		current, err := user.Current()
		assert.NoError(t, err)

		target := User(current.Username)

		found, err := target.Lookup()

		assert.NoError(t, err)
		assert.Equal(t, current.Uid, found.Uid)
	})

	t.Run("empty group rejected", func(t *testing.T) {
		t.Parallel()

		empty := Group("")

		_, err := empty.Lookup()

		assert.Error(t, err)
		assert.IsError(t, err, errEmptyGroup)
	})

	t.Run("unknown group wrapped with context", func(t *testing.T) {
		t.Parallel()

		missing := Group("no-such-group-conga-test")

		_, err := missing.Lookup()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "group lookup")
	})
}
