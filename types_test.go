package conga_test

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

	t.Run("nil safety on pointer methods", func(t *testing.T) {
		t.Parallel()

		var (
			nilFile     *conga.File
			nilDir      *conga.Dir
			nilByteSize *conga.ByteSize
		)

		assert.Equal(t, "", nilFile.String())
		assert.Equal(t, "", nilDir.String())
		assert.Equal(t, "0 B", nilByteSize.String())
		assert.Equal(t, int64(0), nilByteSize.Bytes())
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
			{name: "outer whitespace trimmed", input: "  256MiB  ", expected: 256 * 1024 * 1024, expectError: false},
			{name: "512 mebibytes", input: "512MiB", expected: 512 * 1024 * 1024, expectError: false},
			{name: "2 gibibytes", input: "2GiB", expected: 2 * 1024 * 1024 * 1024, expectError: false},
			{name: "empty input rejected", input: "", expected: 0, expectError: true},
			{name: "raw fractional integer rejected", input: "1.5", expected: 0, expectError: true},
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
			{name: "zero bytes", bytes: 0, expected: "0 B"},
			{name: "sub-kibibyte bytes", bytes: 512, expected: "512 B"},
			{name: "kibibyte boundary", bytes: 1024, expected: "1.00 KiB"},
			{name: "fractional kibibytes", bytes: 1536, expected: "1.50 KiB"},
			{name: "mebibyte boundary", bytes: 1024 * 1024, expected: "1.00 MiB"},
			{name: "512 mebibytes", bytes: 512 * 1024 * 1024, expected: "512.00 MiB"},
			{name: "gibibyte boundary", bytes: 1024 * 1024 * 1024, expected: "1.00 GiB"},
			{name: "2 gibibytes", bytes: 2 * 1024 * 1024 * 1024, expected: "2.00 GiB"},
			{name: "tebibyte boundary", bytes: 1024 * 1024 * 1024 * 1024, expected: "1.00 TiB"},
			{name: "pebibyte boundary", bytes: 1024 * 1024 * 1024 * 1024 * 1024, expected: "1.00 PiB"},
		}

		for _, formatCase := range formatCases {
			t.Run(formatCase.name, func(t *testing.T) {
				t.Parallel()

				byteSize := conga.ByteSize(formatCase.bytes)

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
		_ conga.HelpProvider         = &byteSize
	)

	t.Run("static descriptions", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Path to a file", file.Help(nil))
		assert.Equal(t, "Path to a directory", dir.Help(nil))
		assert.Equal(t, "Byte size (e.g., 10MiB, 1.5GB, 500KiB)", byteSize.Help())
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
	assert.Equal(t, "Path to a file (or '-' for stdin) to read into memory", content.Help())
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
