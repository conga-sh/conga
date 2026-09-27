package dotenv_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/internal/dotenv"
	"github.com/conga-sh/conga/internal/filex"
	"github.com/conga-sh/conga/internal/sentinel"
)

var (
	// Environment file payloads: parser rules and loader inputs under test.
	//go:embed testdata/basic.env
	fixtureBasic []byte

	//go:embed testdata/comments.env
	fixtureComments []byte

	//go:embed testdata/crlf.env
	fixtureCRLF []byte

	//go:embed testdata/escapes.env
	fixtureEscapes []byte

	//go:embed testdata/expansion.env
	fixtureExpansion []byte

	//go:embed testdata/garbage_after_quote.env
	fixtureGarbageAfterQuote []byte

	//go:embed testdata/hierarchy_base.env
	fixtureHierarchyBase []byte

	//go:embed testdata/hierarchy_local.env
	fixtureHierarchyLocal []byte

	//go:embed testdata/load_non_destructive.env
	fixtureLoadNonDestructive []byte

	//go:embed testdata/multi_file_first.env
	fixtureMultiFileFirst []byte

	//go:embed testdata/multi_file_second.env
	fixtureMultiFileSecond []byte

	//go:embed testdata/multiline.env
	fixtureMultiline []byte

	//go:embed testdata/overload.env
	fixtureOverload []byte

	//go:embed testdata/unclosed_double.env
	fixtureUnclosedDouble []byte

	//go:embed testdata/unclosed_single.env
	fixtureUnclosedSingle []byte

	//go:embed testdata/unicode.env
	fixtureUnicode []byte
)

func TestDotenv_ParseBasic(t *testing.T) {
	t.Parallel()

	result, err := dotenv.Parse(fixtureBasic)

	assert.NoError(t, err)
	assert.Equal(t, "bar", result["FOO"])
	assert.Equal(t, "qux", result["BAZ"])
	assert.Equal(t, "literal $VAR", result["SINGLE"])
	assert.Equal(t, "", result["EMPTY"])
	assert.Equal(t, "exported_val", result["EXPORT_KEY"])
	assert.Equal(t, "true", result["EXPORTED"])
}

func TestDotenv_ParseEscapeSequences(t *testing.T) {
	t.Parallel()

	result, err := dotenv.Parse(fixtureEscapes)

	assert.NoError(t, err)
	assert.Equal(t, "line1\nline2", result["NEWLINE"])
	assert.Equal(t, "col1\tcol2", result["TAB"])
	assert.Equal(t, "hello \"world\"", result["QUOTE"])
	assert.Equal(t, "path\\to\\dir", result["SLASH"])
}

func TestDotenv_VariableExpansion(t *testing.T) {
	t.Parallel()

	result, err := dotenv.Parse(fixtureExpansion)

	assert.NoError(t, err)
	assert.Equal(t, "localhost", result["HOST"])
	assert.Equal(t, "8080", result["PORT"])
	assert.Equal(t, "http://localhost:8080/api", result["BASE_URL"])
	assert.Equal(t, "Hello localhost", result["GREETING"])
}

func TestDotenv_InlineComments(t *testing.T) {
	t.Parallel()

	result, err := dotenv.Parse(fixtureComments)

	assert.NoError(t, err)
	assert.Equal(t, "value", result["UNQUOTED"])
	assert.Equal(t, "value # not a comment", result["DOUBLE"])
	assert.Equal(t, "value # also not a comment", result["SINGLE"])
}

func TestDotenv_MultiLineDoubleQuoted(t *testing.T) {
	t.Parallel()

	result, err := dotenv.Parse(fixtureMultiline)

	assert.NoError(t, err)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----", result["CERT"])
}

func TestDotenv_SyntaxErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing equals", func(t *testing.T) {
		t.Parallel()

		raw := "INVALID_LINE_WITHOUT_EQUALS"

		_, err := dotenv.Parse([]byte(raw))

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrDotenv)
		assert.IsError(t, err, sentinel.ErrDotenvParse)
		assert.Equal(t, "line 1: dotenv error: parsing .env file: missing '=' in statement", err.Error())
	})

	t.Run("invalid start character in key", func(t *testing.T) {
		t.Parallel()

		raw := "123INVALID=value"

		_, err := dotenv.Parse([]byte(raw))

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrDotenv)
		assert.IsError(t, err, sentinel.ErrDotenvParse)
		assert.Equal(t, "line 1: dotenv error: parsing .env file: invalid character '1' at start of key", err.Error())
	})

	t.Run("unclosed double quote", func(t *testing.T) {
		t.Parallel()

		raw := `UNCLOSED="some value without closing quote`

		_, err := dotenv.Parse([]byte(raw))

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrDotenv)
		assert.IsError(t, err, sentinel.ErrDotenvParse)
		assert.Equal(t, "line 1: dotenv error: parsing .env file: unclosed double quote", err.Error())
	})

	t.Run("unclosed single quote", func(t *testing.T) {
		t.Parallel()

		raw := `UNCLOSED='some value without closing quote`

		_, err := dotenv.Parse([]byte(raw))

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrDotenv)
		assert.IsError(t, err, sentinel.ErrDotenvParse)
		assert.Equal(t, "line 1: dotenv error: parsing .env file: unclosed single quote", err.Error())
	})
}

func TestDotenvError_Contract(t *testing.T) {
	t.Parallel()

	parseCause := sentinel.ErrDotenvParse

	t.Run("Msg takes precedence", func(t *testing.T) {
		t.Parallel()

		err := &dotenv.DotenvError{Path: "a.env", Msg: "custom message", Cause: parseCause}

		assert.Equal(t, "custom message", err.Error())
		assert.True(t, err.Is(sentinel.ErrDotenv))
		assert.True(t, err.Is(sentinel.ErrDotenvParse))
	})

	t.Run("Path and Cause fallback format", func(t *testing.T) {
		t.Parallel()

		err := &dotenv.DotenvError{Path: "a.env", Cause: parseCause}

		assert.Equal(t, `dotenv file "a.env": dotenv error: parsing .env file`, err.Error())
		assert.True(t, err.Is(sentinel.ErrDotenvParse))
	})

	t.Run("bare error fallback and Unwrap default", func(t *testing.T) {
		t.Parallel()

		err := &dotenv.DotenvError{}

		assert.Equal(t, "dotenv error", err.Error())
		assert.True(t, err.Is(sentinel.ErrDotenv))
		assert.True(t, errors.Is(err, sentinel.ErrDotenv))
	})

	t.Run("Is rejects unrelated targets", func(t *testing.T) {
		t.Parallel()

		err := &dotenv.DotenvError{Path: "a.env", Cause: parseCause}

		assert.False(t, err.Is(sentinel.ErrDotenvRead))
		assert.False(t, err.Is(errors.New("unrelated")))
	})
}

func TestDotenv_LoadNonDestructive(t *testing.T) {
	t.Setenv("TEST_EXISTING_OS_KEY", "os_original_value")

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, fixtureLoadNonDestructive, 0o600)
	assert.NoError(t, err)

	err = dotenv.Load(envPath)

	assert.NoError(t, err)
	assert.Equal(t, "os_original_value", os.Getenv("TEST_EXISTING_OS_KEY"))
	assert.Equal(t, "from_dotenv_new", os.Getenv("TEST_NEW_KEY"))
}

func TestDotenv_TildeHomeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, []byte("TEST_TILDE_EXPANSION=success\n"), 0o600)
	assert.NoError(t, err)

	err = dotenv.Load("~/.env")

	assert.NoError(t, err)
	assert.Equal(t, "success", os.Getenv("TEST_TILDE_EXPANSION"))
}

func TestDotenv_LoadExplicitMissingFile(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "nonexistent.env")

	err := dotenv.Load(missingPath)

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrDotenv)
	assert.IsError(t, err, sentinel.ErrDotenvRead)
}

func TestDotenv_Overload(t *testing.T) {
	t.Setenv("TEST_OVERWRITE_KEY", "initial_value")
	t.Setenv("TEST_INITIAL_KEY", "keep_value")

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, fixtureOverload, 0o600)
	assert.NoError(t, err)

	err = dotenv.Overload(envPath)

	assert.NoError(t, err)
	assert.Equal(t, "overloaded_value", os.Getenv("TEST_OVERWRITE_KEY"))
	assert.Equal(t, "keep_value", os.Getenv("TEST_INITIAL_KEY"))
	assert.Equal(t, "brand_new", os.Getenv("TEST_ANOTHER_NEW_KEY"))
}

func TestDotenv_OverloadExplicitMissingFile(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "nonexistent.env")

	err := dotenv.Overload(missingPath)

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrDotenv)
	assert.IsError(t, err, sentinel.ErrDotenvRead)
}

func TestDotenv_LoadMultipleFiles(t *testing.T) {
	t.Setenv("TEST_MULTI_EXISTING", "original")

	tmpDir := t.TempDir()
	env1 := filepath.Join(tmpDir, "file1.env")
	env2 := filepath.Join(tmpDir, "file2.env")

	err := os.WriteFile(env1, fixtureMultiFileFirst, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(env2, fixtureMultiFileSecond, 0o600)
	assert.NoError(t, err)

	err = dotenv.Load(env1, env2)

	assert.NoError(t, err)
	assert.Equal(t, "original", os.Getenv("TEST_MULTI_EXISTING"))
	assert.Equal(t, "val1", os.Getenv("TEST_MULTI_KEY1"))
	assert.Equal(t, "val2", os.Getenv("TEST_MULTI_KEY2"))
}

func TestDotenv_OverloadMultipleFiles(t *testing.T) {
	t.Setenv("TEST_MULTI_OVERLOAD", "original")

	tmpDir := t.TempDir()
	env1 := filepath.Join(tmpDir, "file1.env")
	env2 := filepath.Join(tmpDir, "file2.env")

	err := os.WriteFile(env1, []byte("TEST_MULTI_OVERLOAD=file1\n"), 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(env2, []byte("TEST_MULTI_OVERLOAD=file2\n"), 0o600)
	assert.NoError(t, err)

	err = dotenv.Overload(env1, env2)

	assert.NoError(t, err)
	assert.Equal(t, "file2", os.Getenv("TEST_MULTI_OVERLOAD"))
}

func TestDotenv_ParseCRLF(t *testing.T) {
	t.Parallel()

	result, err := dotenv.Parse(fixtureCRLF)

	assert.NoError(t, err)
	assert.Equal(t, "bar", result["FOO"])
	assert.Equal(t, "qux", result["BAZ"])
	assert.Equal(t, "first\nsecond", result["MULTILINE"])
}

func TestDotenv_ParseErrors(t *testing.T) {
	t.Parallel()

	t.Run("unclosed double quote", func(t *testing.T) {
		t.Parallel()

		_, err := dotenv.Parse(fixtureUnclosedDouble)

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrDotenvParse)
		assert.Contains(t, err.Error(), "unclosed double quote")
	})

	t.Run("unclosed single quote", func(t *testing.T) {
		t.Parallel()

		_, err := dotenv.Parse(fixtureUnclosedSingle)

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrDotenvParse)
		assert.Contains(t, err.Error(), "unclosed single quote")
	})

	t.Run("garbage after closing quote", func(t *testing.T) {
		t.Parallel()

		_, err := dotenv.Parse(fixtureGarbageAfterQuote)

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrDotenvParse)
		assert.Contains(t, err.Error(), "unexpected characters after closing quote")
	})
}

func TestDotenv_FileSizeLimitExceeded(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	largeFile := filepath.Join(dir, "large.env")

	f, err := os.Create(largeFile)
	assert.NoError(t, err)
	err = f.Truncate(16*1024*1024 + 1)
	assert.NoError(t, err)
	err = f.Close()
	assert.NoError(t, err)

	err = dotenv.Load(largeFile)

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrDotenvRead)
	assert.IsError(t, err, filex.ErrFileSizeLimitExceeded)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestDotenv_ParseUnicode(t *testing.T) {
	t.Parallel()

	result, err := dotenv.Parse(fixtureUnicode)

	assert.NoError(t, err)
	assert.Equal(t, "héllo wörld 🚀", result["DOUBLE"])
	assert.Equal(t, "héllo wörld 🚀", result["SINGLE"])
	assert.Equal(t, "héllo_wörld_🚀", result["UNQUOTED"])
	assert.Equal(t, "日本語と中文", result["CJK"])
	assert.Equal(t, "héllo\n\"wörld\"", result["ESC"])
}

//nolint:paralleltest // t.Chdir and t.Setenv cannot be used with t.Parallel
func TestDotenv_LoadDefaultHierarchy_BothFilesOverloadLocalWins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, ".env"), fixtureHierarchyBase, 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, ".env.local"), fixtureHierarchyLocal, 0o600)
	assert.NoError(t, err)

	err = dotenv.Overload()

	assert.NoError(t, err)
	assert.Equal(t, "from_local", os.Getenv("TEST_DOTENV_HIER_KEY"))
	assert.Equal(t, "local", os.Getenv("TEST_DOTENV_HIER_LOCAL_ONLY"))
}

//nolint:paralleltest // t.Chdir and t.Setenv cannot be used with t.Parallel
func TestDotenv_LoadDefaultHierarchy_LoadKeepsFirstFileValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("TEST_DOTENV_HIER_FIRST=from_env\n"), 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, ".env.local"), []byte("TEST_DOTENV_HIER_FIRST=from_local\n"), 0o600)
	assert.NoError(t, err)

	err = dotenv.Load()

	assert.NoError(t, err)
	assert.Equal(t, "from_env", os.Getenv("TEST_DOTENV_HIER_FIRST"))
}

//nolint:paralleltest // t.Chdir and t.Setenv cannot be used with t.Parallel
func TestDotenv_LoadDefaultHierarchy_LocalOnlyFileLoaded(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, ".env.local"), []byte("TEST_DOTENV_HIER_LOCAL=only_local\n"), 0o600)
	assert.NoError(t, err)

	err = dotenv.Load()

	assert.NoError(t, err)
	assert.Equal(t, "only_local", os.Getenv("TEST_DOTENV_HIER_LOCAL"))
}

//nolint:paralleltest // t.Chdir and t.Setenv cannot be used with t.Parallel
func TestDotenv_LoadDefaultHierarchy_NoFilesIsSilentNoOp(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := dotenv.Load()

	assert.NoError(t, err)
	assert.Equal(t, "", os.Getenv("TEST_DOTENV_HIER_UNSET"))
}

//nolint:paralleltest // t.Chdir and t.Setenv cannot be used with t.Parallel
func TestDotenv_LoadDefaultHierarchy_MalformedFilePropagates(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, ".env"), fixtureUnclosedDouble, 0o600)
	assert.NoError(t, err)

	err = dotenv.Load()

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrDotenv)
	assert.IsError(t, err, sentinel.ErrDotenvParse)
	assert.Contains(t, err.Error(), "cannot parse .env file")
}
