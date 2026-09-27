package spec

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// fileTagSpecs mirrors the File domain type's registered tag vocabulary so
// the shared tag machinery can be exercised without importing the root
// package.
func fileTagSpecs() []TagSpec {
	return []TagSpec{
		{Name: "existing", Kind: TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
		{Name: "stdin", Kind: TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
		{Name: "stdout", Kind: TagKindBool, Excludes: []string{"stdin", "existing", "executable", "symlink"}},
		{Name: "executable", Kind: TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "symlink", "glob"}},
		{Name: "symlink", Kind: TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "executable", "glob"}},
		{Name: "glob", Kind: TagKindString, Excludes: []string{"executable", "symlink"}, Validate: validateGlobTagSpecValue},
	}
}

// validateGlobTagSpecValue adapts ParseGlobPatterns to the TagSpec.Validate
// hook, mirroring the root package's File tag registration.
func validateGlobTagSpecValue(value string) error {
	_, err := ParseGlobPatterns(value)

	return err
}

func TestFieldTagSpecsForField(t *testing.T) {
	t.Parallel()

	fileType := reflect.TypeFor[congaFileStub]()
	sliceType := reflect.TypeFor[[]congaFileStub]()

	t.Run("element type resolves pointer-receiver provider", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, fileTagSpecs(), fieldTagSpecsForField(fileType))
	})

	t.Run("slice element resolves provider", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, fileTagSpecs(), fieldTagSpecsForField(sliceType))
	})

	t.Run("unregistered type returns nil", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, fieldTagSpecsForField(reflect.TypeFor[string]()))
	})

	t.Run("nil type returns nil", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, fieldTagSpecsForField(nil))
	})
}

func TestParseFieldTags(t *testing.T) {
	t.Parallel()

	fileSpecs := fileTagSpecs()

	t.Run("parses declared boolean tags canonically", func(t *testing.T) {
		t.Parallel()

		tag := reflect.StructTag(`existing:"true" stdin:"true"`)

		fieldTags, err := ParseFieldTags(tag, fileSpecs)

		assert.NoError(t, err)
		assert.Equal(t, FieldTags{"existing": "true", "stdin": "true"}, fieldTags)
		assert.True(t, fieldTags.Bool("existing"))
		assert.True(t, fieldTags.Has("stdin"))
		assert.False(t, fieldTags.Has("stdout"))
	})

	t.Run("bare presence maps to true", func(t *testing.T) {
		t.Parallel()

		fieldTags, err := ParseFieldTags(reflect.StructTag(`existing:""`), fileSpecs)

		assert.NoError(t, err)
		assert.Equal(t, FieldTags{"existing": "true"}, fieldTags)
	})

	t.Run("explicit false is parsed", func(t *testing.T) {
		t.Parallel()

		fieldTags, err := ParseFieldTags(reflect.StructTag(`existing:"false"`), fileSpecs)

		assert.NoError(t, err)
		assert.Equal(t, FieldTags{"existing": "false"}, fieldTags)
		assert.False(t, fieldTags.Bool("existing"))
	})

	t.Run("undeclared tags yield nil", func(t *testing.T) {
		t.Parallel()

		fieldTags, err := ParseFieldTags(reflect.StructTag(`help:"Path"`), fileSpecs)

		assert.NoError(t, err)
		assert.Zero(t, fieldTags)
	})

	t.Run("empty specs yield nil", func(t *testing.T) {
		t.Parallel()

		fieldTags, err := ParseFieldTags(reflect.StructTag(`existing:"true"`), nil)

		assert.NoError(t, err)
		assert.Zero(t, fieldTags)
	})

	t.Run("invalid boolean value rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`existing:"yes"`), fileSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, (`schema error: invalid struct tag: tag "existing" expects "true" or "false", got "yes"`), err.Error())
	})

	t.Run("stdin and stdout combination rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`stdin:"true" stdout:"true"`), fileSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, (`schema error: invalid struct tag: tag "stdin" cannot be combined with tag "stdout"`), err.Error())
	})

	t.Run("existing and stdout combination rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`existing:"true" stdout:"true"`), fileSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, (`schema error: invalid struct tag: tag "existing" cannot be combined with tag "stdout"`), err.Error())
	})

	t.Run("existing and stdin combination accepted", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`existing:"true" stdin:"true"`), fileSpecs)

		assert.NoError(t, err)
	})

	t.Run("parses declared string tag verbatim", func(t *testing.T) {
		t.Parallel()

		fieldTags, err := ParseFieldTags(reflect.StructTag(`glob:"Makefile,Dockerfile"`), fileSpecs)

		assert.NoError(t, err)
		assert.Equal(t, FieldTags{"glob": "Makefile,Dockerfile"}, fieldTags)
		assert.Equal(t, "Makefile,Dockerfile", fieldTags.String("glob"))
	})

	t.Run("absent string tag is not declared", func(t *testing.T) {
		t.Parallel()

		fieldTags, err := ParseFieldTags(reflect.StructTag(`existing:"true"`), fileSpecs)

		assert.NoError(t, err)
		assert.False(t, fieldTags.Has("glob"))
	})

	t.Run("empty string value rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`glob:"   "`), fileSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, `schema error: invalid struct tag: tag "glob" cannot be empty`, err.Error())
	})

	t.Run("validate hook rejection propagates exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`glob:"["`), fileSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, `schema error: invalid struct tag: glob pattern "[" is malformed: syntax error in pattern`, err.Error())
	})

	t.Run("validate hook accepts valid glob patterns", func(t *testing.T) {
		t.Parallel()

		fieldTags, err := ParseFieldTags(reflect.StructTag(`glob:" *.yaml , Dockerfile "`), fileSpecs)

		assert.NoError(t, err)
		assert.Equal(t, FieldTags{"glob": " *.yaml , Dockerfile "}, fieldTags)
	})

	t.Run("validate hook plain error is classified as invalid struct tag", func(t *testing.T) {
		t.Parallel()

		hookSpecs := []TagSpec{
			{Name: "mode", Kind: TagKindString, Validate: func(string) error {
				return errors.New("must be one of fast, slow")
			}},
		}

		_, err := ParseFieldTags(reflect.StructTag(`mode:"warp"`), hookSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.IsError(t, err, ErrSchema)
		assert.Equal(t, `schema error: invalid struct tag: tag "mode": must be one of fast, slow`, err.Error())
	})

	t.Run("validate hook error wrapping ErrSchema propagates unchanged", func(t *testing.T) {
		t.Parallel()

		hookSpecs := []TagSpec{
			{Name: "mode", Kind: TagKindString, Validate: func(value string) error {
				return fmt.Errorf("%w: mode %q is not supported", ErrInvalidStructTag, value)
			}},
		}

		_, err := ParseFieldTags(reflect.StructTag(`mode:"warp"`), hookSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, `schema error: invalid struct tag: mode "warp" is not supported`, err.Error())
	})

	t.Run("glob and executable combination rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`executable:"true" glob:"*.yaml"`), fileSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, `schema error: invalid struct tag: tag "executable" cannot be combined with tag "glob"`, err.Error())
	})

	t.Run("glob and symlink combination rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseFieldTags(reflect.StructTag(`symlink:"true" glob:"*.conf"`), fileSpecs)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, `schema error: invalid struct tag: tag "symlink" cannot be combined with tag "glob"`, err.Error())
	})
}

func TestDefaultTypeHelp(t *testing.T) {
	t.Parallel()

	t.Run("pointer-receiver provider resolves with declared tags", func(t *testing.T) {
		t.Parallel()

		helpText := DefaultTypeHelp(reflect.TypeFor[congaFileStub](), FieldTags{"existing": "true", "stdin": "true"})

		assert.Equal(t, "Path to an existing file or '-' for stdin", helpText)
	})

	t.Run("pointer-receiver provider without tags", func(t *testing.T) {
		t.Parallel()

		helpText := DefaultTypeHelp(reflect.TypeFor[congaFileStub](), nil)

		assert.Equal(t, "Path to a file", helpText)
	})

	t.Run("value-receiver parameterless form resolves", func(t *testing.T) {
		t.Parallel()

		helpText := DefaultTypeHelp(reflect.TypeFor[valueHelpStub](), nil)

		assert.Equal(t, "value help", helpText)
	})

	t.Run("pointer-receiver parameterless form resolves", func(t *testing.T) {
		t.Parallel()

		helpText := DefaultTypeHelp(reflect.TypeFor[pointerHelpStub](), nil)

		assert.Equal(t, "pointer help", helpText)
	})

	t.Run("slice element type is unwrapped", func(t *testing.T) {
		t.Parallel()

		helpText := DefaultTypeHelp(reflect.TypeFor[[]congaFileStub](), FieldTags{"stdout": "true"})

		assert.Equal(t, "Path to a file or '-' for stdout", helpText)
	})

	t.Run("type without help reports empty", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", DefaultTypeHelp(reflect.TypeFor[string](), nil))
	})

	t.Run("nil type reports empty", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", DefaultTypeHelp(nil, nil))
	})
}

// congaFileStub mirrors the conga.File domain type so the fieldTagSpecsForField
// dispatch and DefaultTypeHelp resolution can be exercised without importing
// the root package.
type congaFileStub string

func (f *congaFileStub) TagSpecs() []TagSpec {
	return fileTagSpecs()
}

func (f *congaFileStub) Help(tags FieldTags) string {
	helpText := "Path to a file"
	if tags.Bool("existing") {
		helpText = "Path to an existing file"
	}

	if globSpecs, err := ParseGlobPatterns(tags.String("glob")); err == nil && len(globSpecs) > 0 {
		helpText += " matching '" + strings.Join(globSpecs, ",") + "'"
	}

	switch {
	case tags.Has("stdin"):
		return helpText + " or '-' for stdin"
	case tags.Has("stdout"):
		return helpText + " or '-' for stdout"
	default:
		return helpText
	}
}

// valueHelpStub exercises the value-receiver HelpProvider resolution path.
type valueHelpStub string

func (v valueHelpStub) Help() string {
	return "value help"
}

// pointerHelpStub exercises the pointer-receiver parameterless HelpProvider
// resolution path.
type pointerHelpStub string

func (p *pointerHelpStub) Help() string {
	return "pointer help"
}
