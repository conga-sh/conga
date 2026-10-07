package conga_test

import (
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
)

func TestGenericContainers(t *testing.T) {
	t.Parallel()

	t.Run("OptionalFlag[int] uninitialized default state", func(t *testing.T) {
		t.Parallel()

		var intFlag conga.OptionalFlag[int]

		assert.False(t, intFlag.IsSet())
		assert.Equal(t, 8080, intFlag.GetOr(8080))
	})

	t.Run("OptionalFlag[int] set value and source detail", func(t *testing.T) {
		t.Parallel()

		var intFlag conga.OptionalFlag[int]

		conga.OptionalFlagContainer(&intFlag).SetValue(9000, conga.SourceCLI, "--port")

		assert.True(t, intFlag.IsSet())
		assert.Equal(t, 9000, intFlag.Get())
		assert.Equal(t, conga.SourceCLI, intFlag.Source())
		assert.Equal(t, "--port", intFlag.SourceDetail())
	})

	t.Run("RequiredFlag[string] required state", func(t *testing.T) {
		t.Parallel()

		var requiredFlag conga.RequiredFlag[string]

		assert.True(t, conga.RequiredFlagContainer(&requiredFlag).IsRequired())
	})

	t.Run("RequiredFlag[string] set value and source", func(t *testing.T) {
		t.Parallel()

		var requiredFlag conga.RequiredFlag[string]

		conga.RequiredFlagContainer(&requiredFlag).SetValue("admin", conga.SourceEnv, "ROLE")

		assert.Equal(t, "admin", requiredFlag.Get())
		assert.Equal(t, conga.SourceEnv, requiredFlag.Source())
	})

	t.Run("RequiredArg[string] required state", func(t *testing.T) {
		t.Parallel()

		var stringArg conga.RequiredArg[string]

		assert.True(t, conga.RequiredArgContainer(&stringArg).IsRequired())
	})

	t.Run("RequiredArg[string] set value", func(t *testing.T) {
		t.Parallel()

		var stringArg conga.RequiredArg[string]

		conga.RequiredArgContainer(&stringArg).SetValue("source.txt", conga.SourceCLI, "")

		assert.Equal(t, "source.txt", stringArg.Get())
	})

	t.Run("OptionalArg[string]", func(t *testing.T) {
		t.Parallel()

		var optionalArg conga.OptionalArg[string]

		assert.False(t, conga.OptionalArgContainer(&optionalArg).IsRequired())
		assert.Equal(t, "default.txt", optionalArg.GetOr("default.txt"))
	})

	t.Run("DashArgs uninitialized", func(t *testing.T) {
		t.Parallel()

		var dashArgs conga.DashArgs

		assert.False(t, dashArgs.IsSet())
		assert.Equal(t, 0, dashArgs.Len())
	})

	t.Run("DashArgs set", func(t *testing.T) {
		t.Parallel()

		var dashArgs conga.DashArgs

		conga.DashArgsContainer(&dashArgs).SetArgs([]string{"-v", "-run", "TestFoo"})

		assert.True(t, dashArgs.IsSet())
		assert.Equal(t, 3, dashArgs.Len())
		assert.Equal(t, "-v -run TestFoo", dashArgs.String())
	})

	t.Run("Counter uninitialized", func(t *testing.T) {
		t.Parallel()

		var counter conga.Counter

		assert.False(t, counter.IsSet())
		assert.Equal(t, 0, counter.Get())
		assert.Equal(t, 5, counter.GetOr(5))
	})

	t.Run("Counter set", func(t *testing.T) {
		t.Parallel()

		var counter conga.Counter

		conga.CounterContainer(&counter).SetValue(3, conga.SourceCLI, "-vvv")

		assert.True(t, counter.IsSet())
		assert.Equal(t, 3, counter.Get())
		assert.Equal(t, conga.SourceCLI, counter.Source())
		assert.Equal(t, "-vvv", counter.SourceDetail())
		assert.Equal(t, "3", counter.String())
	})

	t.Run("OptionalFlag[map[string]string] uninitialized", func(t *testing.T) {
		t.Parallel()

		var mapContainer conga.OptionalFlag[map[string]string]

		assert.False(t, mapContainer.IsSet())
		assert.Equal(t, map[string]string(nil), mapContainer.Get())
		assert.Equal(t, map[string]string{"default": "1"}, mapContainer.GetOr(map[string]string{"default": "1"}))
	})

	t.Run("OptionalFlag[map[string]string] set", func(t *testing.T) {
		t.Parallel()

		var mapContainer conga.OptionalFlag[map[string]string]

		conga.OptionalFlagContainer(&mapContainer).SetValue(map[string]string{"env": "prod", "tier": "backend"}, conga.SourceCLI, "--tag")

		assert.True(t, mapContainer.IsSet())
		assert.Equal(t, 2, len(mapContainer.Get()))
		assert.Equal(t, "prod", mapContainer.Get()["env"])
		assert.Equal(t, "backend", mapContainer.Get()["tier"])
		assert.Equal(t, conga.SourceCLI, mapContainer.Source())
		assert.Equal(t, "--tag", mapContainer.SourceDetail())
	})

	t.Run("RawArgs uninitialized", func(t *testing.T) {
		t.Parallel()

		var rawArgs conga.RawArgs

		assert.False(t, rawArgs.IsSet())
		assert.Equal(t, 0, rawArgs.Len())
	})

	t.Run("RawArgs set", func(t *testing.T) {
		t.Parallel()

		var rawArgs conga.RawArgs

		conga.RawArgsContainer(&rawArgs).SetArgs([]string{"ls", "-la", "/tmp"})

		assert.True(t, rawArgs.IsSet())
		assert.Equal(t, 3, rawArgs.Len())
		assert.Equal(t, "ls -la /tmp", rawArgs.String())
	})

	t.Run("Container nil safety", func(t *testing.T) {
		t.Parallel()

		var (
			nilOptFlag  *conga.OptionalFlag[string]
			nilReqFlag  *conga.RequiredFlag[int]
			nilOptArg   *conga.OptionalArg[bool]
			nilReqArg   *conga.RequiredArg[float64]
			nilCfgFile  *conga.ConfigFile
			nilCounter  *conga.Counter
			nilDashArgs *conga.DashArgs
			nilRawArgs  *conga.RawArgs
		)

		assert.Equal(t, "", nilOptFlag.String())
		assert.Equal(t, "", nilReqFlag.String())
		assert.Equal(t, "", nilOptArg.String())
		assert.Equal(t, "", nilReqArg.String())
		assert.Equal(t, "", nilCfgFile.String())
		assert.Equal(t, "0", nilCounter.String())
		assert.Equal(t, "", nilDashArgs.String())
		assert.Equal(t, "", nilRawArgs.String())
	})
}

type dummyCmd struct {
	executed bool
}

func (d *dummyCmd) Run(ctx context.Context) error {
	d.executed = true

	return nil
}

func TestCommandContainer(t *testing.T) {
	t.Parallel()

	var cmd conga.Command[dummyCmd]

	assert.False(t, cmd.IsSelected())

	cmdContainer := conga.CommandContainer(&cmd)
	cmdContainer.MarkSelected()

	assert.True(t, cmd.IsSelected())
	assert.NotZero(t, cmd.Get())
	assert.Equal(t, any(cmd.Get()), cmdContainer.InnerValue())
}

func TestDomainTypes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello"), 0644)
	assert.NoError(t, err)

	t.Run("File with existing tag", func(t *testing.T) {
		t.Parallel()

		t.Run("valid file", func(t *testing.T) {
			t.Parallel()

			var file conga.File

			decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

			assert.NoError(t, file.UnmarshalTextWithContext([]byte(testFile), decodeContext))

			err := file.Validate(t.Context())

			assert.NoError(t, err)
			assert.Equal(t, testFile, file.String())
		})

		t.Run("nonexistent file", func(t *testing.T) {
			t.Parallel()

			missingPath := filepath.Join(tmpDir, "nonexistent.txt")

			var file conga.File

			decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

			assert.NoError(t, file.UnmarshalTextWithContext([]byte(missingPath), decodeContext))

			err := file.Validate(t.Context())

			assert.Error(t, err)
			assert.IsError(t, err, fs.ErrNotExist)
		})

		t.Run("directory rejected", func(t *testing.T) {
			t.Parallel()

			var file conga.File

			decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

			assert.NoError(t, file.UnmarshalTextWithContext([]byte(tmpDir), decodeContext))

			err := file.Validate(t.Context())

			assert.Error(t, err)
			assert.Equal(t, fmt.Sprintf("%q is a directory, but a file was expected", tmpDir), err.Error())
		})

		t.Run("existing tag absent skips stat", func(t *testing.T) {
			t.Parallel()

			missingPath := filepath.Join(tmpDir, "nonexistent.txt")

			var file conga.File

			decodeContext := conga.DecodeContext{}

			assert.NoError(t, file.UnmarshalTextWithContext([]byte(missingPath), decodeContext))

			err := file.Validate(t.Context())

			assert.NoError(t, err)
		})

		t.Run("dash without capability tag rejected", func(t *testing.T) {
			t.Parallel()

			var file conga.File

			decodeContext := conga.DecodeContext{}

			assert.NoError(t, file.UnmarshalTextWithContext([]byte("-"), decodeContext))

			err := file.Validate(t.Context())

			assert.Error(t, err)
			assert.Equal(t, `"-" requires the stdin or stdout tag on the field`, err.Error())
		})

		t.Run("dash accepted with stdin tag", func(t *testing.T) {
			t.Parallel()

			var file conga.File

			decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"stdin": "true", "existing": "true"}}

			assert.NoError(t, file.UnmarshalTextWithContext([]byte("-"), decodeContext))

			err := file.Validate(t.Context())

			assert.NoError(t, err)
		})
	})

	t.Run("Dir with existing tag", func(t *testing.T) {
		t.Parallel()

		t.Run("valid directory", func(t *testing.T) {
			t.Parallel()

			var dir conga.Dir

			decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

			assert.NoError(t, dir.UnmarshalTextWithContext([]byte(tmpDir), decodeContext))

			err := dir.Validate(t.Context())

			assert.NoError(t, err)
		})

		t.Run("regular file rejected", func(t *testing.T) {
			t.Parallel()

			var dir conga.Dir

			decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

			assert.NoError(t, dir.UnmarshalTextWithContext([]byte(testFile), decodeContext))

			err := dir.Validate(t.Context())

			assert.Error(t, err)
			assert.Equal(t, fmt.Sprintf("%q is a file, but a directory was expected", testFile), err.Error())
		})

		t.Run("existing tag absent skips stat", func(t *testing.T) {
			t.Parallel()

			var dir conga.Dir

			decodeContext := conga.DecodeContext{}

			assert.NoError(t, dir.UnmarshalTextWithContext([]byte("/tmp/nonexistent-dir"), decodeContext))

			err := dir.Validate(t.Context())

			assert.NoError(t, err)
		})
	})

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		var file conga.File

		err := file.UnmarshalText([]byte("/tmp/nonexistent-file.txt"))

		assert.NoError(t, err)
		assert.Equal(t, "/tmp/nonexistent-file.txt", file.String())

		var _ conga.TagAwarePredictable = &file

		assert.NotZero(t, file.Predict(nil))
	})

	t.Run("Dir", func(t *testing.T) {
		t.Parallel()

		var dir conga.Dir

		err := dir.UnmarshalText([]byte("/tmp/nonexistent-dir"))

		assert.NoError(t, err)
		assert.Equal(t, "/tmp/nonexistent-dir", dir.String())

		var _ conga.Predictable = &dir

		assert.NotZero(t, dir.Predict())
	})

	t.Run("Predictable interface conformance", func(t *testing.T) {
		t.Parallel()

		var (
			file conga.File
			dir  conga.Dir

			_ conga.TagAwarePredictable = &file
			_ conga.Predictable         = &dir
		)
	})

	t.Run("TagSpecProvider interface conformance", func(t *testing.T) {
		t.Parallel()

		var (
			file conga.File
			dir  conga.Dir

			_ conga.TagSpecProvider = &file
			_ conga.TagSpecProvider = &dir
		)

		fileSpecs := file.TagSpecs()

		// Validate hooks are function values and cannot be compared
		// structurally; the glob hook's behavior is asserted in
		// TestFileGlobPredict.
		for index := range fileSpecs {
			fileSpecs[index].Validate = nil
		}

		assert.Equal(t, []conga.TagSpec{
			{Name: "existing", Kind: conga.TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
			{Name: "stdin", Kind: conga.TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
			{Name: "stdout", Kind: conga.TagKindBool, Excludes: []string{"stdin", "existing", "executable", "symlink"}},
			{Name: "executable", Kind: conga.TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "symlink", "glob"}},
			{Name: "symlink", Kind: conga.TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "executable", "glob"}},
			{Name: "glob", Kind: conga.TagKindString, Excludes: []string{"executable", "symlink"}},
		}, fileSpecs)

		dirSpecs := dir.TagSpecs()

		assert.Equal(t, []conga.TagSpec{{Name: "existing", Kind: conga.TagKindBool}}, dirSpecs)
	})

	t.Run("zero value on domain types", func(t *testing.T) {
		t.Parallel()

		var (
			zeroFile     conga.File
			zeroDir      conga.Dir
			zeroByteSize conga.ByteSize
			zeroContent  conga.FileContent
		)

		assert.Equal(t, "", zeroFile.String())
		assert.Equal(t, "", zeroDir.String())
		assert.Equal(t, "0B", zeroByteSize.String())
		assert.Equal(t, int64(0), zeroByteSize.Bytes())
		assert.Equal(t, "", zeroContent.String())
	})

	t.Run("File with executable tag unmarshal stores paths without probing", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		binPath := filepath.Join(tmpDir, "myexec")
		assert.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755))

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"executable": "true"}}

		var exec conga.File

		err := exec.UnmarshalTextWithContext([]byte(binPath), decodeContext)

		assert.NoError(t, err)
		assert.Equal(t, binPath, exec.String())

		nonExistentPath := filepath.Join(tmpDir, "does-not-exist")

		var nonExistent conga.File

		err = nonExistent.UnmarshalTextWithContext([]byte(nonExistentPath), decodeContext)

		assert.NoError(t, err)
		assert.Equal(t, nonExistentPath, nonExistent.String())
	})

	t.Run("File with executable tag validate existing regular file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		binPath := filepath.Join(tmpDir, "myexec")
		assert.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755))

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"executable": "true"}}

		var exec conga.File

		assert.NoError(t, exec.UnmarshalTextWithContext([]byte(binPath), decodeContext))

		err := exec.Validate(t.Context())

		assert.NoError(t, err)
	})

	t.Run("File with executable tag validate missing path", func(t *testing.T) {
		t.Parallel()

		missingPath := filepath.Join(t.TempDir(), "does-not-exist")

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"executable": "true"}}

		var exec conga.File

		assert.NoError(t, exec.UnmarshalTextWithContext([]byte(missingPath), decodeContext))

		err := exec.Validate(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, fs.ErrNotExist)
	})

	t.Run("File with executable tag validate directory rejected", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"executable": "true"}}

		var exec conga.File

		assert.NoError(t, exec.UnmarshalTextWithContext([]byte(tmpDir), decodeContext))

		err := exec.Validate(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, fmt.Sprintf("%q is a directory, but an executable file was expected", tmpDir), err.Error())
	})

	t.Run("File with symlink tag unmarshal stores expanded paths without probing", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		regularPath := filepath.Join(tmpDir, "regular.txt")
		assert.NoError(t, os.WriteFile(regularPath, []byte("data"), 0600))

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"symlink": "true"}}

		var symlink conga.File

		err := symlink.UnmarshalTextWithContext([]byte(regularPath), decodeContext)

		assert.NoError(t, err)
		assert.Equal(t, regularPath, symlink.String())
	})

	t.Run("File with symlink tag validate symbolic link accepted", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		assert.NoError(t, os.WriteFile(target, []byte("data"), 0600))

		symlinkPath := filepath.Join(tmpDir, "link.txt")
		if err := os.Symlink(target, symlinkPath); err != nil {
			t.Skip("symlinks not supported")
		}

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"symlink": "true"}}

		var symlink conga.File

		assert.NoError(t, symlink.UnmarshalTextWithContext([]byte(symlinkPath), decodeContext))

		err := symlink.Validate(t.Context())

		assert.NoError(t, err)
	})

	t.Run("File with symlink tag validate non-symlink rejected", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		assert.NoError(t, os.WriteFile(target, []byte("data"), 0600))

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"symlink": "true"}}

		var symlink conga.File

		assert.NoError(t, symlink.UnmarshalTextWithContext([]byte(target), decodeContext))

		err := symlink.Validate(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, fmt.Sprintf("%q is not a symbolic link", target), err.Error())
	})

	t.Run("ByteSize table-driven test cases", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name        string
			input       string
			expected    int64
			expectError bool
		}{
			{name: "pure integer without unit", input: "1024", expected: 1024, expectError: false},
			{name: "zero integer", input: "0", expected: 0, expectError: false},
			{name: "exact bytes B unit", input: "500B", expected: 500, expectError: false},
			{name: "SI kilobytes lowercase k", input: "1kB", expected: 1000, expectError: false},
			{name: "SI kilobytes uppercase K", input: "2KB", expected: 2000, expectError: false},
			{name: "IEC kibibytes", input: "1KiB", expected: 1024, expectError: false},
			{name: "SI megabytes", input: "10MB", expected: 10 * 1000 * 1000, expectError: false},
			{name: "IEC mebibytes", input: "1MiB", expected: 1024 * 1024, expectError: false},
			{name: "SI gigabytes", input: "2GB", expected: 2 * 1000 * 1000 * 1000, expectError: false},
			{name: "IEC gibibytes", input: "1GiB", expected: 1024 * 1024 * 1024, expectError: false},
			{name: "SI terabytes", input: "1TB", expected: 1000 * 1000 * 1000 * 1000, expectError: false},
			{name: "IEC tebibytes", input: "1TiB", expected: 1024 * 1024 * 1024 * 1024, expectError: false},
			{name: "SI petabytes", input: "1PB", expected: 1000 * 1000 * 1000 * 1000 * 1000, expectError: false},
			{name: "IEC pebibytes", input: "1PiB", expected: 1024 * 1024 * 1024 * 1024 * 1024, expectError: false},
			{name: "fractional SI megabytes", input: "1.5MB", expected: int64(1.5 * 1000 * 1000), expectError: false},
			{name: "fractional IEC gibibytes", input: "0.5GiB", expected: int64(0.5 * 1024 * 1024 * 1024), expectError: false},
			{name: "fractional trailing zeros resolve exactly", input: "1.500GB", expected: 1500 * 1000 * 1000, expectError: false},
			{name: "outer whitespace trimmed", input: "  256MiB  ", expected: 256 * 1024 * 1024, expectError: false},
			{name: "512 mebibytes", input: "512MiB", expected: 512 * 1024 * 1024, expectError: false},
			{name: "2 gibibytes", input: "2GiB", expected: 2 * 1024 * 1024 * 1024, expectError: false},
			{name: "empty input rejected", input: "", expected: 0, expectError: true},
			{name: "raw fractional integer rejected", input: "1.5", expected: 0, expectError: true},
			{name: "leading dot without digits rejected", input: ".B", expected: 0, expectError: true},
			{name: "leading dot fraction rejected", input: ".5MiB", expected: 0, expectError: true},
			{name: "trailing dot rejected", input: "1.", expected: 0, expectError: true},
			{name: "multiple dots rejected", input: "1.2.3B", expected: 0, expectError: true},
			{name: "fractional byte below one rejected", input: "0.5B", expected: 0, expectError: true},
			{name: "fractional byte above one rejected", input: "1.5B", expected: 0, expectError: true},
			{name: "inexact binary fraction rejected", input: "0.1GiB", expected: 0, expectError: true},
			{name: "inexact decimal sub-byte fraction rejected", input: "0.0000001MB", expected: 0, expectError: true},
			{name: "lowercase byte unit rejected", input: "500b", expected: 0, expectError: true},
			{name: "naked K prefix rejected", input: "4K", expected: 0, expectError: true},
			{name: "naked M prefix rejected", input: "10M", expected: 0, expectError: true},
			{name: "naked G prefix rejected", input: "1G", expected: 0, expectError: true},
			{name: "naked Gi prefix rejected", input: "10Gi", expected: 0, expectError: true},
			{name: "lowercase kib rejected", input: "100kib", expected: 0, expectError: true},
			{name: "uppercase MIB rejected", input: "512MIB", expected: 0, expectError: true},
			{name: "megabit casing rejected", input: "10Mb", expected: 0, expectError: true},
			{name: "lowercase megabyte rejected", input: "10mb", expected: 0, expectError: true},
			{name: "lowercase gigabyte rejected", input: "1gb", expected: 0, expectError: true},
			{name: "internal whitespace rejected", input: "256 MiB", expected: 0, expectError: true},
			{name: "invalid number", input: "abcMB", expected: 0, expectError: true},
			{name: "invalid unit", input: "100XYZ", expected: 0, expectError: true},
			{name: "negative value", input: "-5MB", expected: 0, expectError: true},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var byteSize conga.ByteSize

				err := byteSize.UnmarshalText([]byte(testCase.input))
				if testCase.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, testCase.expected, byteSize.Bytes())
				}
			})
		}
	})

	t.Run("ByteSize string IEC formatting", func(t *testing.T) {
		t.Parallel()

		formatCases := []struct {
			name     string
			bytes    int64
			expected string
		}{
			{name: "zero bytes", bytes: 0, expected: "0B"},
			{name: "sub-kibibyte bytes", bytes: 512, expected: "512B"},
			{name: "kibibyte boundary", bytes: 1024, expected: "1KiB"},
			{name: "fractional kibibytes", bytes: 1536, expected: "1536B"},
			{name: "mebibyte boundary", bytes: 1024 * 1024, expected: "1MiB"},
			{name: "512 mebibytes", bytes: 512 * 1024 * 1024, expected: "512MiB"},
			{name: "gibibyte boundary", bytes: 1024 * 1024 * 1024, expected: "1GiB"},
			{name: "2 gibibytes", bytes: 2 * 1024 * 1024 * 1024, expected: "2GiB"},
			{name: "tebibyte boundary", bytes: 1024 * 1024 * 1024 * 1024, expected: "1TiB"},
			{name: "pebibyte boundary", bytes: 1024 * 1024 * 1024 * 1024 * 1024, expected: "1PiB"},
			{name: "negative kibibyte", bytes: -1024, expected: "-1KiB"},
			{name: "negative mebibyte", bytes: -1024 * 1024, expected: "-1MiB"},
			{name: "negative sub-kibibyte", bytes: -512, expected: "-512B"},
			{name: "indivisible remainder", bytes: 1048577, expected: "1048577B"},
			{name: "boundary minus one", bytes: 1048575, expected: "1048575B"},
			{name: "math MinInt64 safe", bytes: math.MinInt64, expected: "-8192PiB"},
		}

		for _, formatCase := range formatCases {
			t.Run(formatCase.name, func(t *testing.T) {
				t.Parallel()

				byteSize := conga.NewByteSize(formatCase.bytes)

				assert.Equal(t, formatCase.expected, byteSize.String())
			})
		}
	})
}

// TestFileExecutableTagResolvesPathRelativeNames verifies that $PATH-relative
// names resolve to their absolute location at parse time for fields declaring
// the executable capability. It mutates process-wide PATH state via t.Setenv,
// so it does not run in parallel.
func TestFileExecutableTagResolvesPathRelativeNames(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "pathtool")
	assert.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755))
	t.Setenv("PATH", tmpDir)

	decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"executable": "true"}}

	var exec conga.File

	err := exec.UnmarshalTextWithContext([]byte("pathtool"), decodeContext)

	assert.NoError(t, err)
	assert.Equal(t, binPath, exec.String())
}

func TestFileExecutableTagValidateNotExecutable(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("execute-bit check is not enforced on Windows")
	}

	tmpDir := t.TempDir()
	progPath := filepath.Join(tmpDir, "prog")
	assert.NoError(t, os.WriteFile(progPath, []byte("#!/bin/sh\n"), 0o644))

	decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"executable": "true"}}

	var prog conga.File

	assert.NoError(t, prog.UnmarshalTextWithContext([]byte(progPath), decodeContext))

	err := prog.Validate(t.Context())

	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("%q is not executable", progPath), err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

// TestDomainTypeHelp verifies that every built-in domain type contributes a
// default help description via Help(tags), and that File/Dir tailor the
// description to the field's declared tags.
func TestDomainTypeHelp(t *testing.T) {
	t.Parallel()

	var (
		file     conga.File
		dir      conga.Dir
		byteSize conga.ByteSize

		_ conga.TagAwareHelpProvider = &file
		_ conga.TagAwareHelpProvider = &dir
		_ conga.TagAwareHelpProvider = &byteSize
	)

	t.Run("static descriptions", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Path to a file", file.Help(nil))
		assert.Equal(t, "Path to a directory", dir.Help(nil))
		assert.Equal(t, "Byte size (e.g., 10MiB, 1.5GB, 500KiB)", byteSize.Help(nil))
	})

	t.Run("File with executable and symlink tags", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Path to an executable", file.Help(conga.FieldTags{"executable": "true"}))
		assert.Equal(t, "Path to a symbolic link", file.Help(conga.FieldTags{"symlink": "true"}))
	})

	t.Run("File composes from declared tags", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			tags     conga.FieldTags
			expected string
		}{
			{name: "no tags", tags: nil, expected: "Path to a file"},
			{name: "existing", tags: conga.FieldTags{"existing": "true"}, expected: "Path to an existing file"},
			{name: "stdin", tags: conga.FieldTags{"stdin": "true"}, expected: "Path to a file or '-' for stdin"},
			{name: "stdout", tags: conga.FieldTags{"stdout": "true"}, expected: "Path to a file or '-' for stdout"},
			{name: "existing and stdin", tags: conga.FieldTags{"existing": "true", "stdin": "true"}, expected: "Path to an existing file or '-' for stdin"},
			{name: "explicit false", tags: conga.FieldTags{"existing": "false"}, expected: "Path to a file"},
			{name: "glob", tags: conga.FieldTags{"glob": "Makefile,Dockerfile"}, expected: "Path to a file matching 'Makefile,Dockerfile'"},
			{name: "existing and glob", tags: conga.FieldTags{"existing": "true", "glob": "*.yaml"}, expected: "Path to an existing file matching '*.yaml'"},
			{name: "glob and stdin", tags: conga.FieldTags{"glob": "*.yaml", "stdin": "true"}, expected: "Path to a file matching '*.yaml' or '-' for stdin"},
			{name: "glob trims pattern elements", tags: conga.FieldTags{"glob": " *.yaml , Dockerfile "}, expected: "Path to a file matching '*.yaml,Dockerfile'"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, testCase.expected, file.Help(testCase.tags))
			})
		}
	})

	t.Run("Dir composes from declared tags", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Path to a directory", dir.Help(nil))
		assert.Equal(t, "Path to an existing directory", dir.Help(conga.FieldTags{"existing": "true"}))
	})
}

func TestFileGlobPredict(t *testing.T) {
	t.Parallel()

	t.Run("glob tag filters predicted files by base name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("all:"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.go"), []byte("package main"), 0o600))
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "deploy"), 0o755))

		var file conga.File

		pred := file.Predict(conga.FieldTags{"glob": "Makefile,*.yaml"})
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		values := slicex.Map(result.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "Makefile"))
		assert.True(t, slices.Contains(values, "deploy/"), "directories must be kept for traversal")
		assert.False(t, slices.Contains(values, "app.go"))
	})

	t.Run("absent glob tag predicts all files", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.go"), []byte("package main"), 0o600))

		var file conga.File

		pred := file.Predict(nil)
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		values := slicex.Map(result.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "app.go"))
	})

	t.Run("malformed glob value yields no file candidates", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.go"), []byte("package main"), 0o600))

		var file conga.File

		pred := file.Predict(conga.FieldTags{"glob": "["})
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Zero(t, result.Items)
	})

	t.Run("glob hook rejects malformed pattern", func(t *testing.T) {
		t.Parallel()

		var file conga.File

		tagSpecs := file.TagSpecs()
		globSpecIndex := slices.IndexFunc(tagSpecs, func(tagSpec conga.TagSpec) bool {
			return tagSpec.Name == "glob"
		})

		assert.True(t, globSpecIndex >= 0)
		assert.Error(t, tagSpecs[globSpecIndex].Validate("["))
		assert.NoError(t, tagSpecs[globSpecIndex].Validate("Makefile,*.yaml"))
	})
}

func TestContainerAccessors(t *testing.T) {
	t.Parallel()

	t.Run("OptionalFlag accessors", func(t *testing.T) {
		t.Parallel()

		var boolFlag conga.OptionalFlag[bool]

		assert.False(t, boolFlag.IsSet())
		assert.Equal(t, "false", boolFlag.String())

		conga.OptionalFlagContainer(&boolFlag).SetValue(true, conga.SourceCLI, "--verbose")

		assert.True(t, boolFlag.IsSet())
		assert.Equal(t, "true", boolFlag.String())

		var stringFlag conga.OptionalFlag[string]

		assert.False(t, stringFlag.IsSet())
		assert.Equal(t, "", stringFlag.String())

		conga.OptionalFlagContainer(&stringFlag).SetValue("hello", conga.SourceCLI, "--name")

		assert.True(t, stringFlag.IsSet())
		assert.Equal(t, "hello", stringFlag.String())
	})

	t.Run("RequiredFlag accessors", func(t *testing.T) {
		t.Parallel()

		var reqFlag conga.RequiredFlag[int]

		assert.False(t, reqFlag.IsSet())
		assert.Equal(t, "0", reqFlag.String())

		conga.RequiredFlagContainer(&reqFlag).SetValue(42, conga.SourceCLI, "--port")

		assert.True(t, reqFlag.IsSet())
		assert.True(t, reqFlag.Changed())
		assert.Equal(t, 42, reqFlag.Get())
		assert.Equal(t, conga.SourceCLI, reqFlag.Source())
		assert.Equal(t, "--port", reqFlag.SourceDetail())
		assert.Equal(t, "42", reqFlag.String())
	})

	t.Run("Arg accessors", func(t *testing.T) {
		t.Parallel()

		var arg conga.RequiredArg[string]

		assert.False(t, arg.IsSet())
		assert.Equal(t, "", arg.String())

		conga.RequiredArgContainer(&arg).SetValue("hello", conga.SourceCLI, "arg0")

		assert.True(t, arg.IsSet())
		assert.True(t, arg.Changed())
		assert.Equal(t, "hello", arg.Get())
		assert.Equal(t, conga.SourceCLI, arg.Source())
		assert.Equal(t, "arg0", arg.SourceDetail())
		assert.Equal(t, "hello", arg.String())
	})

	t.Run("OptionalArg accessors", func(t *testing.T) {
		t.Parallel()

		var optArg conga.OptionalArg[string]

		assert.False(t, optArg.IsSet())
		assert.Equal(t, "fallback", optArg.GetOr("fallback"))
		assert.Equal(t, "", optArg.String())

		conga.OptionalArgContainer(&optArg).SetValue("world", conga.SourceDefault, "")

		assert.True(t, optArg.IsSet())
		assert.False(t, optArg.Changed())
		assert.Equal(t, "world", optArg.Get())
		assert.Equal(t, "world", optArg.GetOr("fallback"))
		assert.Equal(t, conga.SourceDefault, optArg.Source())
		assert.Equal(t, "", optArg.SourceDetail())
		assert.Equal(t, "world", optArg.String())
	})

	t.Run("Counter accessors", func(t *testing.T) {
		t.Parallel()

		var counter conga.Counter

		assert.False(t, counter.IsSet())
		assert.Equal(t, 5, counter.GetOr(5))
		assert.Equal(t, "0", counter.String())

		conga.CounterContainer(&counter).SetValue(3, conga.SourceCLI, "-vvv")

		assert.True(t, counter.IsSet())
		assert.True(t, counter.Changed())
		assert.Equal(t, 3, counter.Get())
		assert.Equal(t, 3, counter.GetOr(5))
		assert.Equal(t, conga.SourceCLI, counter.Source())
		assert.Equal(t, "-vvv", counter.SourceDetail())
		assert.Equal(t, "3", counter.String())
	})

	t.Run("ConfigFile accessors", func(t *testing.T) {
		t.Parallel()

		var cfg conga.ConfigFile

		assert.False(t, cfg.IsSet())
		assert.Equal(t, 0, len(cfg.Paths()))
		assert.Equal(t, "", cfg.Path())
		assert.Equal(t, "", cfg.String())

		conga.ConfigFileContainer(&cfg).SetValue("custom.json", conga.SourceEnv, "CONFIG_FILE")

		assert.True(t, cfg.IsSet())
		assert.True(t, cfg.Changed())
		assert.Equal(t, "custom.json", cfg.Path())
		assert.Equal(t, []string{"custom.json"}, cfg.Paths())
		assert.Equal(t, conga.SourceEnv, cfg.Source())
		assert.Equal(t, "CONFIG_FILE", cfg.SourceDetail())
		assert.Equal(t, "custom.json", cfg.String())
	})

	t.Run("ConfigFile multi-value accessors", func(t *testing.T) {
		t.Parallel()

		var cfgs conga.ConfigFile

		assert.False(t, cfgs.IsSet())
		assert.Equal(t, 0, len(cfgs.Paths()))

		conga.ConfigFileContainer(&cfgs).SetValue([]string{"a.json", "b.json"}, conga.SourceCLI, "--config")

		assert.True(t, cfgs.IsSet())
		assert.True(t, cfgs.Changed())
		assert.Equal(t, "b.json", cfgs.Path())
		assert.Equal(t, []string{"a.json", "b.json"}, cfgs.Paths())
		assert.Equal(t, conga.SourceCLI, cfgs.Source())
		assert.Equal(t, "--config", cfgs.SourceDetail())
		assert.Equal(t, "a.json, b.json", cfgs.String())
	})

	t.Run("ConfigFile Paths returns a copy", func(t *testing.T) {
		t.Parallel()

		var cfg conga.ConfigFile

		conga.ConfigFileContainer(&cfg).SetValue([]string{"a.json", "b.json"}, conga.SourceCLI, "--config")

		paths := cfg.Paths()
		paths[0] = "mutated.json"

		assert.Equal(t, []string{"a.json", "b.json"}, cfg.Paths())
	})
}

func TestFileHelpers(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "readme.txt")
	err := os.WriteFile(testFile, []byte("hello world"), 0644)
	assert.NoError(t, err)

	var file conga.File
	assert.NoError(t, file.UnmarshalText([]byte(testFile)))

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		reader, err := file.Reader()
		assert.NoError(t, err)

		defer func() { _ = reader.Close() }()

		data, err := io.ReadAll(reader)

		assert.NoError(t, err)
		assert.Equal(t, "hello world", string(data))
	})

	t.Run("Stat", func(t *testing.T) {
		t.Parallel()

		info, err := file.Stat()

		assert.NoError(t, err)
		assert.Equal(t, int64(len("hello world")), info.Size())
		assert.Equal(t, false, info.IsDir())
	})

	var missingFile conga.File
	assert.NoError(t, missingFile.UnmarshalText([]byte(filepath.Join(tmpDir, "missing.txt"))))

	t.Run("Reader nonexistent", func(t *testing.T) {
		t.Parallel()

		_, err := missingFile.Reader()

		assert.Error(t, err)
	})

	t.Run("Stat nonexistent", func(t *testing.T) {
		t.Parallel()

		_, err := missingFile.Stat()

		assert.Error(t, err)
	})
}

func TestFileContent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "payload.txt")
	err := os.WriteFile(testFile, []byte("payload"), 0600)
	assert.NoError(t, err)

	var content conga.FileContent

	err = content.UnmarshalText([]byte(testFile))

	assert.NoError(t, err)
	assert.Equal(t, "payload", content.String())
	assert.Equal(t, []byte("payload"), content.Bytes())
	assert.Equal(t, "Path to a file (or '-' for stdin)", content.Help())
	assert.NotZero(t, content.Predict())

	t.Run("nonexistent file", func(t *testing.T) {
		t.Parallel()

		var missing conga.FileContent

		err := missing.UnmarshalText([]byte(filepath.Join(tmpDir, "missing.txt")))

		assert.Error(t, err)
	})

	t.Run("directory rejected", func(t *testing.T) {
		t.Parallel()

		var dir conga.FileContent

		err := dir.UnmarshalText([]byte(tmpDir))

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
	})
}

func TestFileContentSizeLimit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	largeFile := filepath.Join(tmpDir, "large.txt")
	file, err := os.Create(largeFile)
	assert.NoError(t, err)

	err = file.Truncate(16*1024*1024 + 1)
	assert.NoError(t, err)
	assert.NoError(t, file.Close())

	var content conga.FileContent

	err = content.UnmarshalText([]byte(largeFile))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum limit")
}

//nolint:paralleltest // Swaps os.Stdin process-wide.
func TestFileContentStdin(t *testing.T) {
	originalStdin := os.Stdin

	pipeRead, pipeWrite, err := os.Pipe()
	assert.NoError(t, err)

	os.Stdin = pipeRead

	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = pipeRead.Close()
		_ = pipeWrite.Close()
	})

	_, err = pipeWrite.WriteString("stdin payload")
	assert.NoError(t, err)
	assert.NoError(t, pipeWrite.Close())

	var content conga.FileContent

	err = content.UnmarshalText([]byte("-"))

	assert.NoError(t, err)
	assert.Equal(t, "stdin payload", content.String())
	assert.Equal(t, []byte("stdin payload"), content.Bytes())
}

//nolint:paralleltest // Swaps os.Stdin process-wide.
func TestFileContentStdinLimitExceeded(t *testing.T) {
	originalStdin := os.Stdin

	pipeRead, pipeWrite, err := os.Pipe()
	assert.NoError(t, err)

	os.Stdin = pipeRead

	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = pipeRead.Close()
		_ = pipeWrite.Close()
	})

	go func() {
		chunk := make([]byte, 64*1024)
		for range 257 {
			_, _ = pipeWrite.Write(chunk)
		}

		_ = pipeWrite.Close()
	}()

	var content conga.FileContent

	err = content.UnmarshalText([]byte("-"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum limit")
}

func TestFileStdio(t *testing.T) {
	t.Parallel()

	t.Run("dash value", func(t *testing.T) {
		t.Parallel()

		var file conga.File

		err := file.UnmarshalText([]byte("-"))

		assert.NoError(t, err)
		assert.True(t, file.IsStdio())
		assert.Equal(t, "-", file.String())
		assert.Equal(t, "Path to a file", file.Help(nil))
		assert.NotZero(t, file.Predict(nil))
	})

	t.Run("disk value", func(t *testing.T) {
		t.Parallel()

		var file conga.File

		err := file.UnmarshalText([]byte("/tmp/input.bin"))

		assert.NoError(t, err)
		assert.False(t, file.IsStdio())
		assert.Equal(t, "/tmp/input.bin", file.String())
	})
}

//nolint:paralleltest // Swaps os.Stdin process-wide.
func TestFileReaderStdinNoClose(t *testing.T) {
	originalStdin := os.Stdin

	pipeRead, pipeWrite, err := os.Pipe()
	assert.NoError(t, err)

	os.Stdin = pipeRead

	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = pipeRead.Close()
		_ = pipeWrite.Close()
	})

	_, err = pipeWrite.WriteString("abc")
	assert.NoError(t, err)

	var file conga.File
	assert.NoError(t, file.UnmarshalText([]byte("-")))

	reader, err := file.Reader()
	assert.NoError(t, err)

	assert.NoError(t, reader.Close())

	buffer := make([]byte, 3)
	_, err = io.ReadFull(reader, buffer)

	assert.NoError(t, err)
	assert.Equal(t, "abc", string(buffer))
}

func TestFileWriter(t *testing.T) {
	t.Parallel()

	t.Run("Writer writes file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "out.txt")

		var file conga.File
		assert.NoError(t, file.UnmarshalText([]byte(path)))

		writer, err := file.Writer()
		assert.NoError(t, err)

		_, err = writer.Write([]byte("first"))
		assert.NoError(t, err)
		assert.NoError(t, writer.Close())

		data, err := os.ReadFile(path)

		assert.NoError(t, err)
		assert.Equal(t, "first", string(data))
	})

	t.Run("Writer truncates existing file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "out.txt")
		assert.NoError(t, os.WriteFile(path, []byte("previous content"), 0600))

		var file conga.File
		assert.NoError(t, file.UnmarshalText([]byte(path)))

		writer, err := file.Writer()
		assert.NoError(t, err)

		_, err = writer.Write([]byte("new"))
		assert.NoError(t, err)
		assert.NoError(t, writer.Close())

		data, err := os.ReadFile(path)

		assert.NoError(t, err)
		assert.Equal(t, "new", string(data))
	})
}

//nolint:paralleltest // Swaps os.Stdout process-wide.
func TestFileWriterStdoutNoClose(t *testing.T) {
	originalStdout := os.Stdout

	pipeRead, pipeWrite, err := os.Pipe()
	assert.NoError(t, err)

	os.Stdout = pipeWrite

	t.Cleanup(func() {
		os.Stdout = originalStdout
		_ = pipeRead.Close()
		_ = pipeWrite.Close()
	})

	var file conga.File
	assert.NoError(t, file.UnmarshalText([]byte("-")))

	writer, err := file.Writer()
	assert.NoError(t, err)

	_, err = writer.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	_, err = os.Stdout.WriteString(" world")
	assert.NoError(t, err)
	assert.NoError(t, pipeWrite.Close())

	data, err := io.ReadAll(pipeRead)

	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

func TestFileTypesEndToEnd(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Content conga.OptionalFlag[conga.FileContent] `help:"File content flag" name:"content"`
		Input   conga.OptionalFlag[conga.File]        `existing:"true"          help:"Input file flag" name:"input"  stdin:"true"`
		Output  conga.OptionalFlag[conga.File]        `help:"Output file flag"  name:"output"          stdout:"true"`
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "data.txt")
	err := os.WriteFile(inputPath, []byte("cli payload"), 0600)
	assert.NoError(t, err)

	outputPath := filepath.Join(tmpDir, "result.txt")

	cli := new(CLI)
	app := conga.New(cli).
		Name("filetypes").
		Args([]string{"--content", inputPath, "--input", inputPath, "--output", outputPath})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "cli payload", cli.Content.Get().String())
	assert.Equal(t, []byte("cli payload"), cli.Content.Get().Bytes())
	inputFile := cli.Input.Get()
	outputFile := cli.Output.Get()

	assert.Equal(t, inputPath, inputFile.String())
	assert.Equal(t, outputPath, outputFile.String())
}

func TestContainer_Flag_Source(t *testing.T) {
	t.Parallel()

	sources := []container.Source{
		container.SourceCLI,
		container.SourceEnv,
		container.SourceConfig,
		container.SourceDefault,
	}

	for _, src := range sources {
		t.Run(src.String(), func(t *testing.T) {
			t.Parallel()

			optionalFlag := conga.OptionalFlag[string]{}
			requiredFlag := conga.RequiredFlag[string]{}
			configFile := conga.ConfigFile{}
			counter := conga.Counter{}

			// The engine state views carry the value binding and provenance
			// contract for every public flag container.
			flags := []struct {
				container interface {
					SetValue(value any, source conga.ValueSource, detail string)
					Source() conga.ValueSource
				}
				value any
			}{
				{conga.OptionalFlagContainer(&optionalFlag), "value"},
				{conga.RequiredFlagContainer(&requiredFlag), "value"},
				{conga.ConfigFileContainer(&configFile), []string{"config.json"}},
				{conga.CounterContainer(&counter), 1},
			}

			for _, flag := range flags {
				flag.container.SetValue(flag.value, src, "detail")

				assert.Equal(t, src, flag.container.Source())
			}
		})
	}
}

func TestIssue22DomainTypeValueStringer(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")
	assert.NoError(t, os.WriteFile(filePath, []byte("key: value\n"), 0o600))

	var (
		fileFlag     conga.OptionalFlag[conga.File]
		dirFlag      conga.OptionalFlag[conga.Dir]
		byteSizeFlag conga.OptionalFlag[conga.ByteSize]
		contentFlag  conga.OptionalFlag[conga.FileContent]
	)

	var file conga.File
	assert.NoError(t, file.UnmarshalText([]byte(filePath)))
	conga.OptionalFlagContainer(&fileFlag).SetValue(file, conga.SourceCLI, "--file")

	var dir conga.Dir
	assert.NoError(t, dir.UnmarshalText([]byte(tmpDir)))
	conga.OptionalFlagContainer(&dirFlag).SetValue(dir, conga.SourceCLI, "--dir")

	conga.OptionalFlagContainer(&byteSizeFlag).SetValue(conga.NewByteSize(1024), conga.SourceCLI, "--size")
	conga.OptionalFlagContainer(&contentFlag).SetValue(conga.FileContent("payload-data"), conga.SourceCLI, "--content")

	t.Run("File value Stringer output avoids struct dump and pointer leakage", func(t *testing.T) {
		t.Parallel()

		fileValue := fileFlag.Get()
		formattedPercentV := fmt.Sprintf("%v", fileValue)
		formattedSprint := fmt.Sprint(fileValue)

		assert.Equal(t, filePath, formattedPercentV)
		assert.Equal(t, filePath, formattedSprint)
		assert.False(t, slices.Contains([]byte(formattedPercentV), '{'))
		assert.False(t, slices.Contains([]byte(formattedPercentV), '}'))

		// Inline chaining on unaddressable temporary
		assert.Equal(t, filePath, fileFlag.Get().String())
	})

	t.Run("Dir value Stringer output avoids struct dump", func(t *testing.T) {
		t.Parallel()

		dirValue := dirFlag.Get()
		formattedPercentV := fmt.Sprintf("%v", dirValue)
		formattedSprint := fmt.Sprint(dirValue)

		assert.Equal(t, tmpDir, formattedPercentV)
		assert.Equal(t, tmpDir, formattedSprint)

		// Inline chaining on unaddressable temporary
		assert.Equal(t, tmpDir, dirFlag.Get().String())
	})

	t.Run("ByteSize value Stringer output avoids raw integer", func(t *testing.T) {
		t.Parallel()

		sizeValue := byteSizeFlag.Get()
		formattedPercentV := fmt.Sprintf("%v", sizeValue)
		formattedSprint := fmt.Sprint(sizeValue)

		assert.Equal(t, "1KiB", formattedPercentV)
		assert.Equal(t, "1KiB", formattedSprint)

		// Inline chaining on unaddressable temporary
		assert.Equal(t, "1KiB", byteSizeFlag.Get().String())
		assert.Equal(t, int64(1024), byteSizeFlag.Get().Bytes())
	})

	t.Run("FileContent value Stringer output", func(t *testing.T) {
		t.Parallel()

		contentValue := contentFlag.Get()
		assert.Equal(t, "payload-data", fmt.Sprintf("%v", contentValue))
		assert.Equal(t, "payload-data", contentFlag.Get().String())
		assert.Equal(t, []byte("payload-data"), contentFlag.Get().Bytes())
	})
}

func TestDomainTypesTextMarshaler(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.txt")
	assert.NoError(t, os.WriteFile(filePath, []byte("content"), 0o600))

	var file conga.File
	assert.NoError(t, file.UnmarshalText([]byte(filePath)))

	var dir conga.Dir
	assert.NoError(t, dir.UnmarshalText([]byte(tmpDir)))

	var byteSize conga.ByteSize
	assert.NoError(t, byteSize.UnmarshalText([]byte("1MiB")))

	content := conga.FileContent("sample content")

	t.Run("MarshalText outputs expected bytes", func(t *testing.T) {
		t.Parallel()

		fileBytes, fileErr := file.MarshalText()
		assert.NoError(t, fileErr)
		assert.Equal(t, []byte(filePath), fileBytes)

		dirBytes, dirErr := dir.MarshalText()
		assert.NoError(t, dirErr)
		assert.Equal(t, []byte(tmpDir), dirBytes)

		sizeBytes, sizeErr := byteSize.MarshalText()
		assert.NoError(t, sizeErr)
		assert.Equal(t, []byte("1MiB"), sizeBytes)

		contentBytes, contentErr := content.MarshalText()
		assert.NoError(t, contentErr)
		assert.Equal(t, []byte("sample content"), contentBytes)
	})

	t.Run("MarshalText to UnmarshalText roundtrip", func(t *testing.T) {
		t.Parallel()

		var roundtripFile conga.File

		fileBytes, _ := file.MarshalText()
		assert.NoError(t, roundtripFile.UnmarshalText(fileBytes))
		assert.Equal(t, file.String(), roundtripFile.String())

		var roundtripDir conga.Dir

		dirBytes, _ := dir.MarshalText()
		assert.NoError(t, roundtripDir.UnmarshalText(dirBytes))
		assert.Equal(t, dir.String(), roundtripDir.String())

		var roundtripSize conga.ByteSize

		sizeBytes, _ := byteSize.MarshalText()
		assert.NoError(t, roundtripSize.UnmarshalText(sizeBytes))
		assert.Equal(t, byteSize.Bytes(), roundtripSize.Bytes())
	})
}

func TestDomainTypesJSONSerializationRoundtrip(t *testing.T) {
	t.Parallel()

	type domainConfig struct {
		Input  conga.File     `json:"input"`
		Output conga.Dir      `json:"output"`
		Memory conga.ByteSize `json:"memory"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	assert.NoError(t, os.WriteFile(filePath, []byte("payload"), 0o600))

	var file conga.File
	assert.NoError(t, file.UnmarshalText([]byte(filePath)))

	var dir conga.Dir
	assert.NoError(t, dir.UnmarshalText([]byte(tmpDir)))

	initial := domainConfig{
		Input:  file,
		Output: dir,
		Memory: conga.NewByteSize(1024 * 1024),
	}

	encoded, err := json.Marshal(initial)
	assert.NoError(t, err)

	// Ensure File and Dir do not serialize to empty object "{}"
	jsonString := string(encoded)
	assert.Contains(t, jsonString, fmt.Sprintf(`"input":%q`, filePath))
	assert.Contains(t, jsonString, fmt.Sprintf(`"output":%q`, tmpDir))
	assert.Contains(t, jsonString, `"memory":"1MiB"`)

	var decoded domainConfig

	err = json.Unmarshal(encoded, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, initial.Input.String(), decoded.Input.String())
	assert.Equal(t, initial.Output.String(), decoded.Output.String())
	assert.Equal(t, initial.Memory.Bytes(), decoded.Memory.Bytes())

	// FileContent: Marshal outputs text rather than base64; Unmarshal loads file path from JSON
	content := conga.FileContent("inline payload")

	encodedContent, err := json.Marshal(content)
	assert.NoError(t, err)
	assert.Equal(t, `"inline payload"`, string(encodedContent))

	var decodedContent conga.FileContent

	err = json.Unmarshal([]byte(fmt.Sprintf("%q", filePath)), &decodedContent)
	assert.NoError(t, err)
	assert.Equal(t, "payload", decodedContent.String())
}

func TestDomainTypesInterfaceAssertions(t *testing.T) {
	t.Parallel()

	// Assert value types satisfy fmt.Stringer and encoding.TextMarshaler
	var (
		_ fmt.Stringer           = conga.File{}
		_ encoding.TextMarshaler = conga.File{}

		_ fmt.Stringer           = conga.Dir{}
		_ encoding.TextMarshaler = conga.Dir{}

		_ fmt.Stringer           = conga.ByteSize{}
		_ encoding.TextMarshaler = conga.ByteSize{}

		_ fmt.Stringer           = conga.FileContent("")
		_ encoding.TextMarshaler = conga.FileContent("")
	)

	// Assert pointer types satisfy encoding.TextUnmarshaler
	var (
		_ encoding.TextUnmarshaler = (*conga.File)(nil)
		_ encoding.TextUnmarshaler = (*conga.Dir)(nil)
		_ encoding.TextUnmarshaler = (*conga.ByteSize)(nil)
		_ encoding.TextUnmarshaler = (*conga.FileContent)(nil)
	)
}

type invalidByteSizeUnitsCLI struct {
	noopRunner

	Memory conga.OptionalFlag[conga.ByteSize] `name:"memory" units:"hex"`
}

type invalidByteSizePrecisionCLI struct {
	noopRunner

	Memory conga.OptionalFlag[conga.ByteSize] `name:"memory" precision:"9"`
}

type invalidByteSizeDisplayCLI struct {
	noopRunner

	Memory conga.OptionalFlag[conga.ByteSize] `display:"hex" name:"memory"`
}

// TestByteSizeEchoesInputFamily verifies the default (untagged) render model:
// parsed values echo the unit family they were written in, exact-byte input
// included, while constructed values keep the canonical IEC integer model.
func TestByteSizeEchoesInputFamily(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "decimal fraction renders as integer decimal", input: "1.5GB", expected: "1500MB"},
		{name: "binary fraction renders as integer binary", input: "0.5GiB", expected: "512MiB"},
		{name: "decimal integer preserved", input: "10MB", expected: "10MB"},
		{name: "decimal lowercase k canonicalized", input: "2KB", expected: "2kB"},
		{name: "binary integer preserved", input: "2GiB", expected: "2GiB"},
		{name: "bare integer echoes exact bytes", input: "1024", expected: "1024B"},
		{name: "bare indivisible integer emits bytes", input: "1536", expected: "1536B"},
		{name: "bare large integer emits bytes", input: "1048577", expected: "1048577B"},
		{name: "byte suffix emits bytes", input: "512B", expected: "512B"},
		{name: "byte suffix does not abbreviate", input: "1048576B", expected: "1048576B"},
		{name: "byte suffix below kibibyte stays bytes", input: "2048B", expected: "2048B"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var byteSize conga.ByteSize

			assert.NoError(t, byteSize.UnmarshalText([]byte(testCase.input)))
			assert.Equal(t, testCase.expected, byteSize.String())
		})
	}
}

// TestByteSizePrecisionRendersExactFractions verifies that the precision tag
// enables exact fractional rendering and that its absence keeps integer multiples.
func TestByteSizePrecisionRendersExactFractions(t *testing.T) {
	t.Parallel()

	t.Run("fraction with matching precision", func(t *testing.T) {
		t.Parallel()

		var byteSize conga.ByteSize

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"display": "decimal", "precision": "1"}}

		assert.NoError(t, byteSize.UnmarshalTextWithContext([]byte("1.5GB"), decodeContext))
		assert.Equal(t, "1.5GB", byteSize.String())
	})

	t.Run("fraction without precision falls back to integer multiple", func(t *testing.T) {
		t.Parallel()

		var byteSize conga.ByteSize

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"display": "decimal"}}

		assert.NoError(t, byteSize.UnmarshalTextWithContext([]byte("1.5GB"), decodeContext))
		assert.Equal(t, "1500MB", byteSize.String())
	})

	t.Run("binary fraction with precision", func(t *testing.T) {
		t.Parallel()

		var byteSize conga.ByteSize

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"precision": "1"}}

		assert.NoError(t, byteSize.UnmarshalTextWithContext([]byte("0.5GiB"), decodeContext))
		assert.Equal(t, "0.5GiB", byteSize.String())
	})
}

// TestByteSizeDisplayTagForcesFamily verifies that the display tag overrides the
// family captured from the input, falling back to exact bytes when no unit of the
// forced family divides the value.
func TestByteSizeDisplayTagForcesFamily(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		display  string
		expected string
	}{
		{name: "decimal input forced binary falls back to bytes", input: "10MB", display: "binary", expected: "10000000B"},
		{name: "binary input forced decimal falls back to bytes", input: "1GiB", display: "decimal", expected: "1073741824B"},
		{name: "decimal megabyte forced binary falls back to bytes", input: "1MB", display: "binary", expected: "1000000B"},
		{name: "binary exact bytes forced binary abbreviates", input: "2048B", display: "binary", expected: "2KiB"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var byteSize conga.ByteSize

			decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"display": testCase.display}}

			assert.NoError(t, byteSize.UnmarshalTextWithContext([]byte(testCase.input), decodeContext))
			assert.Equal(t, testCase.expected, byteSize.String())
		})
	}
}

// TestByteSizeUnitsTagRestrictsAcceptedFamilies verifies the units tag accepts or
// rejects a parsed unit family and that absent/both accepts every unit.
func TestByteSizeUnitsTagRestrictsAcceptedFamilies(t *testing.T) {
	t.Parallel()

	t.Run("binary only rejects decimal", func(t *testing.T) {
		t.Parallel()

		var byteSize conga.ByteSize

		err := byteSize.UnmarshalTextWithContext([]byte("10MB"), conga.DecodeContext{Tags: conga.FieldTags{"units": "binary"}})

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, `byte size unit "MB" in "10MB" is not allowed: the field accepts binary units only`, err.Error())
	})

	t.Run("decimal only rejects binary", func(t *testing.T) {
		t.Parallel()

		var byteSize conga.ByteSize

		err := byteSize.UnmarshalTextWithContext([]byte("10MiB"), conga.DecodeContext{Tags: conga.FieldTags{"units": "decimal"}})

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, `byte size unit "MiB" in "10MiB" is not allowed: the field accepts decimal units only`, err.Error())
	})

	t.Run("binary accepts binary", func(t *testing.T) {
		t.Parallel()

		var byteSize conga.ByteSize

		assert.NoError(t, byteSize.UnmarshalTextWithContext([]byte("10MiB"), conga.DecodeContext{Tags: conga.FieldTags{"units": "binary"}}))
		assert.Equal(t, "10MiB", byteSize.String())
	})

	t.Run("both accepts decimal", func(t *testing.T) {
		t.Parallel()

		var byteSize conga.ByteSize

		assert.NoError(t, byteSize.UnmarshalTextWithContext([]byte("10MB"), conga.DecodeContext{Tags: conga.FieldTags{"units": "both"}}))
		assert.Equal(t, "10MB", byteSize.String())
	})

	t.Run("decimal only renders neutral input as exact bytes", func(t *testing.T) {
		t.Parallel()

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"units": "decimal"}}

		var suffixed conga.ByteSize

		assert.NoError(t, suffixed.UnmarshalTextWithContext([]byte("1024B"), decodeContext))
		assert.Equal(t, "1024B", suffixed.String())

		var bare conga.ByteSize

		assert.NoError(t, bare.UnmarshalTextWithContext([]byte("1024"), decodeContext))
		assert.Equal(t, "1024B", bare.String())
	})

	t.Run("binary only renders neutral input as exact bytes", func(t *testing.T) {
		t.Parallel()

		decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"units": "binary"}}

		var suffixed conga.ByteSize

		assert.NoError(t, suffixed.UnmarshalTextWithContext([]byte("1000B"), decodeContext))
		assert.Equal(t, "1000B", suffixed.String())

		var bare conga.ByteSize

		assert.NoError(t, bare.UnmarshalTextWithContext([]byte("1000"), decodeContext))
		assert.Equal(t, "1000B", bare.String())
	})
}

// TestByteSizeTagSchemaErrors verifies that invalid tag values fail schema build
// with a self-describing message and the ErrSchema category.
func TestByteSizeTagSchemaErrors(t *testing.T) {
	t.Parallel()

	t.Run("units", func(t *testing.T) {
		t.Parallel()

		err := conga.New[invalidByteSizeUnitsCLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct invalidByteSizeUnitsCLI field Memory: schema error: invalid struct tag: tag "units": unknown units value "hex" (want "binary", "decimal", or "both")`, err.Error())
	})

	t.Run("precision", func(t *testing.T) {
		t.Parallel()

		err := conga.New[invalidByteSizePrecisionCLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct invalidByteSizePrecisionCLI field Memory: schema error: invalid struct tag: tag "precision": invalid precision value "9" (want 0, 1, 2, or 3)`, err.Error())
	})

	t.Run("display", func(t *testing.T) {
		t.Parallel()

		err := conga.New[invalidByteSizeDisplayCLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct invalidByteSizeDisplayCLI field Memory: schema error: invalid struct tag: tag "display": unknown display value "hex" (want "binary", "decimal", or "input")`, err.Error())
	})
}

// TestByteSizeHelpTailorsToTags verifies the type-derived help description adapts
// to the declared units/display/precision tags.
func TestByteSizeHelpTailorsToTags(t *testing.T) {
	t.Parallel()

	var byteSize conga.ByteSize

	assert.Equal(t, "Byte size (e.g., 10MiB, 1.5GB, 500KiB)", byteSize.Help(nil))
	assert.Equal(t, "Byte size in binary units (e.g., 10KiB, 512MiB)", byteSize.Help(conga.FieldTags{"units": "binary"}))
	assert.Equal(t, "Byte size in decimal units (e.g., 10kB, 512MB)", byteSize.Help(conga.FieldTags{"units": "decimal"}))
	assert.Equal(t, "Byte size (e.g., 10MiB, 1.5GB, 500KiB), shown in the entered unit family", byteSize.Help(conga.FieldTags{"display": "input"}))
	assert.Equal(t, "Byte size (e.g., 10MiB, 1.5GB, 500KiB), up to 2 decimal places", byteSize.Help(conga.FieldTags{"precision": "2"}))
}

// TestByteSizeMarshalTextRoundTrip verifies MarshalText/UnmarshalText preserve the
// exact byte count for constructed non-negative values.
func TestByteSizeMarshalTextRoundTrip(t *testing.T) {
	t.Parallel()

	for _, value := range []int64{0, 1, 1023, 1024, 1536, 1048576, 1048577, 536870912, 1500000000, 1 << 40} {
		byteSize := conga.NewByteSize(value)

		text, err := byteSize.MarshalText()
		assert.NoError(t, err)

		var reparsed conga.ByteSize
		assert.NoError(t, reparsed.UnmarshalText(text))
		assert.Equal(t, value, reparsed.Bytes())
	}
}

// TestByteSizePrecisionRendersLosslessly verifies that every value in the sweep
// reparses to the identical byte count, for both render families and every precision.
func TestByteSizePrecisionRendersLosslessly(t *testing.T) {
	t.Parallel()

	var mismatch string

	for _, display := range []string{"binary", "decimal"} {
		for precision := range 4 {
			tags := conga.FieldTags{"display": display, "precision": strconv.Itoa(precision)}

			for value := range 1 << 16 {
				var byteSize conga.ByteSize

				if err := byteSize.UnmarshalTextWithContext([]byte(strconv.Itoa(value)), conga.DecodeContext{Tags: tags}); err != nil {
					mismatch = fmt.Sprintf("decode %d (%s/%d): %v", value, display, precision, err)

					break
				}

				rendered := byteSize.String()

				var reparsed conga.ByteSize

				if err := reparsed.UnmarshalText([]byte(rendered)); err != nil {
					mismatch = fmt.Sprintf("reparse %q (%s/%d): %v", rendered, display, precision, err)

					break
				}

				if reparsed.Bytes() != int64(value) {
					mismatch = fmt.Sprintf("value %d rendered %q and reparsed as %d (%s/%d)", value, rendered, reparsed.Bytes(), display, precision)

					break
				}
			}

			if mismatch != "" {
				break
			}
		}

		if mismatch != "" {
			break
		}
	}

	assert.Equal(t, "", mismatch)
}

// TestByteSizeNotComparable verifies that ByteSize deliberately rejects == and
// map keys: the render family and precision are not part of a size's identity,
// so callers compare through Equal, Compare, or Bytes.
func TestByteSizeNotComparable(t *testing.T) {
	t.Parallel()

	assert.False(t, reflect.TypeFor[conga.ByteSize]().Comparable())
}

// TestByteSizeEqualAndCompare verifies that quantity equality and ordering
// ignore the captured render family and precision.
func TestByteSizeEqualAndCompare(t *testing.T) {
	t.Parallel()

	var parsedMiB conga.ByteSize
	assert.NoError(t, parsedMiB.UnmarshalText([]byte("1MiB")))

	var parsedBytes conga.ByteSize
	assert.NoError(t, parsedBytes.UnmarshalText([]byte("1048576B")))

	constructed := conga.NewByteSize(1048576)

	assert.True(t, constructed.Equal(parsedMiB))
	assert.True(t, parsedMiB.Equal(parsedBytes))
	assert.True(t, parsedBytes.Equal(constructed))
	assert.Equal(t, 0, constructed.Compare(parsedMiB))

	smaller := conga.NewByteSize(1024)

	assert.Equal(t, -1, smaller.Compare(constructed))
	assert.Equal(t, 1, constructed.Compare(smaller))
}

// TestByteSizeNegativeTextAsymmetry pins the documented asymmetry: programmatic
// negative values render signed but have no parsed text form.
func TestByteSizeNegativeTextAsymmetry(t *testing.T) {
	t.Parallel()

	negative := conga.NewByteSize(-1024)

	assert.Equal(t, "-1KiB", negative.String())
	assert.Equal(t, int64(-1024), negative.Bytes())

	var parsed conga.ByteSize

	err := parsed.UnmarshalText([]byte("-1KiB"))

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `invalid byte size "-1KiB"`, err.Error())
}
