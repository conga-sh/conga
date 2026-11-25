package spec

import (
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/internal/intx"
)

// ByteSize is a Bytes() int64 text dialect: its own UnmarshalText grammar
// supplies bound parsing while Bytes() supplies the ordered projection.
type ByteSize int64

func (b *ByteSize) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}

	*b = ByteSize(parsed)

	return nil
}

func (b *ByteSize) Bytes() int64 {
	return int64(*b)
}

// Celsius is a signed text dialect without a Bytes() projection: bounds parse
// through its own grammar and compare as int64.
type Celsius int64

func (c *Celsius) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseInt(strings.TrimSuffix(string(text), "C"), 10, 64)
	if err != nil {
		return err
	}

	*c = Celsius(parsed)

	return nil
}

// Count is an unsigned text dialect: bounds compare as uint64, so values above
// math.MaxInt64 keep their exact ordering.
type Count uint64

func (c *Count) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseUint(strings.TrimSuffix(string(text), "x"), 10, 64)
	if err != nil {
		return err
	}

	*c = Count(parsed)

	return nil
}

// Ratio is a float text dialect: bounds compare as float64.
type Ratio float64

func (r *Ratio) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseFloat(strings.TrimSuffix(string(text), "%"), 64)
	if err != nil {
		return err
	}

	*r = Ratio(parsed)

	return nil
}

// Opaque implements encoding.TextUnmarshaler on a struct underlying kind
// without a Bytes() projection, so it is not rangeable.
type Opaque struct{}

func (o *Opaque) UnmarshalText(text []byte) error { return nil }

// Plain is a numeric type without its own text grammar: it keeps the built-in
// integer bound parsing.
type Plain int64

type selfReferentialSlice []selfReferentialSlice

type selfReferentialMap map[string]selfReferentialMap

func TestTextRangeDomain(t *testing.T) {
	t.Parallel()

	t.Run("Bytes projection is signed with byte size bound kind", func(t *testing.T) {
		t.Parallel()

		domain, boundKind, ok := TextRangeDomain(reflect.TypeFor[ByteSize]())

		assert.True(t, ok)
		assert.Equal(t, RangeSigned, domain)
		assert.Equal(t, "byte size", boundKind)
	})

	t.Run("signed dialect is signed with value bound kind", func(t *testing.T) {
		t.Parallel()

		domain, boundKind, ok := TextRangeDomain(reflect.TypeFor[Celsius]())

		assert.True(t, ok)
		assert.Equal(t, RangeSigned, domain)
		assert.Equal(t, "value", boundKind)
	})

	t.Run("unsigned dialect compares as uint64", func(t *testing.T) {
		t.Parallel()

		domain, boundKind, ok := TextRangeDomain(reflect.TypeFor[Count]())

		assert.True(t, ok)
		assert.Equal(t, RangeUnsigned, domain)
		assert.Equal(t, "value", boundKind)
	})

	t.Run("float dialect compares as float64", func(t *testing.T) {
		t.Parallel()

		domain, boundKind, ok := TextRangeDomain(reflect.TypeFor[Ratio]())

		assert.True(t, ok)
		assert.Equal(t, RangeFloat, domain)
		assert.Equal(t, "value", boundKind)
	})

	t.Run("pointer element type rejected", func(t *testing.T) {
		t.Parallel()

		_, _, ok := TextRangeDomain(reflect.TypeFor[*ByteSize]())

		assert.False(t, ok)
	})

	t.Run("text unmarshaler without ordered projection rejected", func(t *testing.T) {
		t.Parallel()

		_, _, ok := TextRangeDomain(reflect.TypeFor[Opaque]())

		assert.False(t, ok)
	})

	t.Run("numeric type without text grammar rejected", func(t *testing.T) {
		t.Parallel()

		_, _, ok := TextRangeDomain(reflect.TypeFor[Plain]())

		assert.False(t, ok)
	})

	t.Run("non-numeric type without text grammar rejected", func(t *testing.T) {
		t.Parallel()

		_, _, ok := TextRangeDomain(reflect.TypeFor[string]())

		assert.False(t, ok)
	})
}

func TestValidateRangeSpec_TextDialects(t *testing.T) {
	t.Parallel()

	t.Run("valid byte size range accepted", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "100", MaxStr: "200", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize](), nil, nil)

		assert.NoError(t, err)
	})

	t.Run("inverted byte size range rejected with exact message", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "200", MaxStr: "100", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `min bound "200" cannot be greater than max bound "100"`, err.Error())
	})

	t.Run("unparseable byte size bound rejected with exact message", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "banana", HasMin: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `invalid byte size min bound "banana"`, err.Error())
	})

	t.Run("a bound its parser rejects without a note reports the parser's message as the note", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "5XB", HasMin: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize](), nil, nil)

		assert.Equal(t, `invalid byte size min bound "5XB"`, err.Error())
		assert.Equal(t, `strconv.ParseInt: parsing "5XB": invalid syntax`, errlines.Note(err))
		assert.Equal(t, "", errlines.Hint(err))
		assert.IsError(t, err, strconv.ErrSyntax)
	})

	t.Run("signed dialect bounds parse with its own grammar", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "10C", MaxStr: "30C", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Celsius](), nil, nil)

		assert.NoError(t, err)
	})

	t.Run("signed dialect unparseable bound rejected with value kind", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "bananaC", HasMin: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Celsius](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `invalid value min bound "bananaC"`, err.Error())
		assert.Equal(t, `strconv.ParseInt: parsing "banana": invalid syntax`, errlines.Note(err))
		assert.Equal(t, "", errlines.Hint(err))
		assert.IsError(t, err, strconv.ErrSyntax)
	})

	t.Run("unsigned dialect bounds above int64 max compare exactly", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{
			MinStr: "9223372036854775808x",
			MaxStr: "9223372036854775809x",
			HasMin: true,
			HasMax: true,
		}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Count](), nil, nil)

		assert.NoError(t, err)
	})

	t.Run("unsigned dialect inversion above int64 max detected", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{
			MinStr: "9223372036854775809x",
			MaxStr: "9223372036854775808x",
			HasMin: true,
			HasMax: true,
		}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Count](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `min bound "9223372036854775809x" cannot be greater than max bound "9223372036854775808x"`, err.Error())
	})

	t.Run("float dialect bounds parse with its own grammar", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "10%", MaxStr: "90%", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Ratio](), nil, nil)

		assert.NoError(t, err)
	})

	t.Run("float dialect inversion detected", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "90%", MaxStr: "10%", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Ratio](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `min bound "90%" cannot be greater than max bound "10%"`, err.Error())
	})

	t.Run("text dialect without ordered projection rejected with exact message", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "1", MaxStr: "2", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Opaque](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `range tag is not supported on type spec.Opaque`, err.Error())
	})

	t.Run("numeric type without text grammar keeps integer parsing", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "1", MaxStr: "10", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Plain](), nil, nil)

		assert.NoError(t, err)
	})

	t.Run("numeric type without text grammar rejects non-integer bound", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "banana", HasMin: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[Plain](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `invalid integer min bound "banana"`, err.Error())
		assert.Equal(t, "", errlines.Note(err))
		assert.Equal(t, "", errlines.Hint(err))
		assert.IsError(t, err, strconv.ErrSyntax)
	})

	t.Run("byte-size-named struct without text grammar rejected", func(t *testing.T) {
		t.Parallel()

		type ByteSize struct{}

		rangeSpec := &Range{MinStr: "1", MaxStr: "2", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize](), nil, nil)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `range tag is not supported on type spec.ByteSize`, err.Error())
	})
}

func TestProjectTextValue(t *testing.T) {
	t.Parallel()

	t.Run("Bytes projection wins over numeric kind", func(t *testing.T) {
		t.Parallel()

		projected, ok := ProjectTextValue[int64](reflect.TypeFor[ByteSize](), reflect.ValueOf(ByteSize(4096)))

		assert.True(t, ok)
		assert.Equal(t, int64(4096), projected)
	})

	t.Run("signed underlying projects as int64", func(t *testing.T) {
		t.Parallel()

		projected, ok := ProjectTextValue[int64](reflect.TypeFor[Celsius](), reflect.ValueOf(Celsius(-7)))

		assert.True(t, ok)
		assert.Equal(t, int64(-7), projected)
	})

	t.Run("unsigned underlying projects without clamping", func(t *testing.T) {
		t.Parallel()

		projected, ok := ProjectTextValue[uint64](reflect.TypeFor[Count](), reflect.ValueOf(Count(math.MaxUint64)))

		assert.True(t, ok)
		assert.Equal(t, uint64(math.MaxUint64), projected)
	})

	t.Run("float underlying projects as float64", func(t *testing.T) {
		t.Parallel()

		projected, ok := ProjectTextValue[float64](reflect.TypeFor[Ratio](), reflect.ValueOf(Ratio(0.5)))

		assert.True(t, ok)
		assert.Equal(t, 0.5, projected)
	})

	t.Run("pointer value accepted", func(t *testing.T) {
		t.Parallel()

		value := ByteSize(7)

		projected, ok := ProjectTextValue[int64](reflect.TypeFor[ByteSize](), reflect.ValueOf(&value))

		assert.True(t, ok)
		assert.Equal(t, int64(7), projected)
	})

	t.Run("mismatched domain returns false", func(t *testing.T) {
		t.Parallel()

		_, ok := ProjectTextValue[uint64](reflect.TypeFor[ByteSize](), reflect.ValueOf(ByteSize(1)))

		assert.False(t, ok)
	})

	t.Run("numeric type without text grammar returns false", func(t *testing.T) {
		t.Parallel()

		_, ok := ProjectTextValue[int64](reflect.TypeFor[Plain](), reflect.ValueOf(Plain(1)))

		assert.False(t, ok)
	})
}

func TestParseRangeSpec_Valid(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		tag      string
		expected Range
	}{
		{
			tag: "1..10",
			expected: Range{
				MinStr: "1",
				MaxStr: "10",
				HasMin: true,
				HasMax: true,
				Exact:  false,
			},
		},
		{
			tag: "-5..5",
			expected: Range{
				MinStr: "-5",
				MaxStr: "5",
				HasMin: true,
				HasMax: true,
				Exact:  false,
			},
		},
		{
			tag: "100ms..5s",
			expected: Range{
				MinStr: "100ms",
				MaxStr: "5s",
				HasMin: true,
				HasMax: true,
				Exact:  false,
			},
		},
		{
			tag: "1MB..10GB",
			expected: Range{
				MinStr: "1MB",
				MaxStr: "10GB",
				HasMin: true,
				HasMax: true,
				Exact:  false,
			},
		},
		{
			tag: "1..",
			expected: Range{
				MinStr: "1",
				MaxStr: "",
				HasMin: true,
				HasMax: false,
				Exact:  false,
			},
		},
		{
			tag: "0..",
			expected: Range{
				MinStr: "0",
				MaxStr: "",
				HasMin: true,
				HasMax: false,
				Exact:  false,
			},
		},
		{
			tag: "..100",
			expected: Range{
				MinStr: "",
				MaxStr: "100",
				HasMin: false,
				HasMax: true,
				Exact:  false,
			},
		},
		{
			tag: "..500MB",
			expected: Range{
				MinStr: "",
				MaxStr: "500MB",
				HasMin: false,
				HasMax: true,
				Exact:  false,
			},
		},
		{
			tag: "5",
			expected: Range{
				MinStr: "5",
				MaxStr: "5",
				HasMin: true,
				HasMax: true,
				Exact:  true,
			},
		},
		{
			tag: "8080",
			expected: Range{
				MinStr: "8080",
				MaxStr: "8080",
				HasMin: true,
				HasMax: true,
				Exact:  true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.tag, func(t *testing.T) {
			t.Parallel()

			result, err := ParseRangeSpec(testCase.tag)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected.MinStr, result.MinStr)
			assert.Equal(t, testCase.expected.MaxStr, result.MaxStr)
			assert.Equal(t, testCase.expected.HasMin, result.HasMin)
			assert.Equal(t, testCase.expected.HasMax, result.HasMax)
			assert.Equal(t, testCase.expected.Exact, result.Exact)
		})
	}
}

func TestParseRangeSpec_Invalid(t *testing.T) {
	t.Parallel()

	invalidTags := []string{
		"",
		"   ",
		"..",
		"1..2..3",
	}

	for _, tag := range invalidTags {
		t.Run("invalid_"+tag, func(t *testing.T) {
			t.Parallel()

			_, err := ParseRangeSpec(tag)

			assert.Error(t, err)
			assert.True(t, IsSchemaError(err))
		})
	}
}

// A triple dot is a syntax error, not a min bound "1" with max bound ".5".
func TestParseRangeSpec_TripleDotRejected(t *testing.T) {
	t.Parallel()

	_, err := ParseRangeSpec("1...5")

	assert.True(t, IsSchemaError(err))
	assert.Equal(t, `invalid range syntax "1...5"`, err.Error())
	assert.Equal(t, `expected min..max, min.., or ..max`, errlines.Note(err))
}

func TestSplitTagList(t *testing.T) {
	t.Parallel()

	t.Run("tag missing returns nil", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string(nil), SplitTagList(reflect.StructTag(`other:"val"`), "env"))
	})

	t.Run("empty tag returns nil", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string(nil), SplitTagList(reflect.StructTag(`env:""`), "env"))
	})

	t.Run("single item", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"APP_PORT"}, SplitTagList(reflect.StructTag(`env:"APP_PORT"`), "env"))
	})

	t.Run("multiple items trimmed", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"HOST", "PORT", "ADDR"}, SplitTagList(reflect.StructTag(`env:" HOST , PORT , ADDR "`), "env"))
	})
}

func TestParseStructTagKeys(t *testing.T) {
	t.Parallel()

	t.Run("multiple keys parsed in order", func(t *testing.T) {
		t.Parallel()

		keys, err := parseStructTagKeys(reflect.StructTag(`default:"8080" name:"port" range:"1..10"`))

		assert.NoError(t, err)
		assert.Equal(t, []string{"default", "name", "range"}, keys)
	})

	t.Run("escaped quotes inside value", func(t *testing.T) {
		t.Parallel()

		keys, err := parseStructTagKeys(reflect.StructTag(`help:"say \"hi\" now" name:"port"`))

		assert.NoError(t, err)
		assert.Equal(t, []string{"help", "name"}, keys)
	})

	t.Run("missing colon rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseStructTagKeys(reflect.StructTag(`keywithoutcolon name:"port"`))

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `malformed struct tag "keywithoutcolon name:\"port\""`, err.Error())
	})

	t.Run("missing value rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseStructTagKeys(reflect.StructTag(`name`))

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `malformed struct tag "name"`, err.Error())
	})

	t.Run("unterminated quote rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseStructTagKeys(reflect.StructTag(`name:"unterminated`))

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `malformed struct tag "name:\"unterminated"`, err.Error())
	})

	t.Run("unquoted value rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseStructTagKeys(reflect.StructTag(`name:bare`))

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `malformed struct tag "name:bare"`, err.Error())
	})

	t.Run("empty tag returns nil", func(t *testing.T) {
		t.Parallel()

		keys, err := parseStructTagKeys(reflect.StructTag(``))

		assert.NoError(t, err)
		assert.Equal(t, []string(nil), keys)
	})
}

func TestParseGlobPatterns(t *testing.T) {
	t.Parallel()

	t.Run("splits and trims comma-separated patterns", func(t *testing.T) {
		t.Parallel()

		patterns, err := ParseGlobPatterns(" *.yaml , Dockerfile ")

		assert.NoError(t, err)
		assert.Equal(t, []string{"*.yaml", "Dockerfile"}, patterns)
	})

	t.Run("wildcard-free pattern is valid", func(t *testing.T) {
		t.Parallel()

		patterns, err := ParseGlobPatterns("Makefile,Dockerfile")

		assert.NoError(t, err)
		assert.Equal(t, []string{"Makefile", "Dockerfile"}, patterns)
	})

	t.Run("character class pattern is valid", func(t *testing.T) {
		t.Parallel()

		patterns, err := ParseGlobPatterns("test[0-9].go")

		assert.NoError(t, err)
		assert.Equal(t, []string{"test[0-9].go"}, patterns)
	})

	t.Run("empty element rejected with exact message", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{",", "*.yaml,", ",*.yaml", "*.yaml,,Dockerfile", "*.yaml, ,Dockerfile"} {
			_, err := ParseGlobPatterns(value)

			assert.Error(t, err)
			assert.True(t, IsSchemaError(err))
			assert.Equal(t, `empty glob pattern element`, err.Error())
			assert.Equal(t, `expected comma-separated patterns like "Makefile,Dockerfile" or "*.yaml"`, errlines.Note(err))
		}
	})

	t.Run("forward path separator rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseGlobPatterns("src/*.go")

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `glob pattern "src/*.go" must not contain path separators`, err.Error())
		assert.Equal(t, `patterns match file names during completion traversal`, errlines.Note(err))
	})

	t.Run("backslash path separator rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseGlobPatterns(`Make\file`)

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, "glob pattern \"Make\\\\file\" must not contain path separators", err.Error())
		assert.Equal(t, "patterns match file names during completion traversal", errlines.Note(err))
	})

	t.Run("malformed pattern rejected with exact message", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"[", "a[b", "*[a-"} {
			_, err := ParseGlobPatterns(value)

			assert.Error(t, err)
			assert.True(t, IsSchemaError(err))
			assert.IsError(t, err, filepath.ErrBadPattern)
			assert.Equal(t, `glob pattern "`+value+`" is malformed: syntax error in pattern`, err.Error())
		}
	})
}

func TestValidateConfigFileConfigTag(t *testing.T) {
	t.Parallel()

	t.Run("config dash tag accepted", func(t *testing.T) {
		t.Parallel()

		err := ValidateConfigFileConfigTag(reflect.StructTag(`config:"-"`))

		assert.NoError(t, err)
	})

	t.Run("multi tag accepted", func(t *testing.T) {
		t.Parallel()

		err := ValidateConfigFileConfigTag(reflect.StructTag(`multi:"true"`))

		assert.NoError(t, err)
	})

	t.Run("non-dash value rejected with exact message", func(t *testing.T) {
		t.Parallel()

		err := ValidateConfigFileConfigTag(reflect.StructTag(`config:"server"`))

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `tag "config" on config file flag must be "-", got "server"`, err.Error())
	})
}

func TestParseBootstrapTag(t *testing.T) {
	t.Parallel()

	t.Run("absent", func(t *testing.T) {
		t.Parallel()

		bootstrap, err := parseBootstrapTag(reflect.StructTag(``), "", &Options{})

		assert.NoError(t, err)
		assert.False(t, bootstrap)
	})

	t.Run("requires Bootstrapper", func(t *testing.T) {
		t.Parallel()

		_, err := parseBootstrapTag(reflect.StructTag(`bootstrap:"true"`), "", &Options{})

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper`, err.Error())
		assert.Equal(t, `implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag`, errlines.Hint(err))
	})

	t.Run("rejects feature combination", func(t *testing.T) {
		t.Parallel()

		_, err := parseBootstrapTag(reflect.StructTag(`bootstrap:"true"`), "strict", &Options{HasBootstrapper: true})

		assert.Error(t, err)
		assert.True(t, IsSchemaError(err))
		assert.Equal(t, `tag "bootstrap" cannot be combined with tag "feature"`, err.Error())
		assert.Equal(t, `the bootstrap phase runs before feature gates are resolved`, errlines.Note(err))
	})

	t.Run("accepted with Bootstrapper", func(t *testing.T) {
		t.Parallel()

		bootstrap, err := parseBootstrapTag(reflect.StructTag(`bootstrap:""`), "", &Options{HasBootstrapper: true})

		assert.NoError(t, err)
		assert.True(t, bootstrap)
	})
}

func TestDefaultArgName(t *testing.T) {
	t.Parallel()

	t.Run("name tag wins", func(t *testing.T) {
		t.Parallel()

		name := DefaultArgName("TargetFile", reflect.StructTag(`name:"target"`))

		assert.Equal(t, "target", name)
	})

	t.Run("absent name tag yields kebab case placeholder", func(t *testing.T) {
		t.Parallel()

		name := DefaultArgName("TargetFile", reflect.StructTag(``))

		assert.Equal(t, "<target-file>", name)
	})
}

func TestIsBootstrapType(t *testing.T) {
	t.Parallel()

	allowed := []reflect.Type{
		reflect.TypeFor[string](),
		reflect.TypeFor[bool](),
		reflect.TypeFor[int](),
		reflect.TypeFor[uint64](),
		reflect.TypeFor[float64](),
		reflect.TypeFor[[]string](),
		reflect.TypeFor[map[string]string](),
		reflect.TypeFor[map[int][]bool](),
	}
	for _, targetType := range allowed {
		assert.True(t, isBootstrapType(targetType), targetType.String())
	}

	rejected := []reflect.Type{
		reflect.TypeFor[*string](),
		reflect.TypeFor[struct{}](),
		reflect.TypeFor[[]struct{}](),
		reflect.TypeFor[map[string]struct{}](),
		reflect.TypeFor[selfReferentialSlice](),
		reflect.TypeFor[selfReferentialMap](),
	}
	for _, targetType := range rejected {
		assert.False(t, isBootstrapType(targetType), targetType.String())
	}
}

func TestSchemaError_FormattingAndUnwrap(t *testing.T) {
	t.Parallel()

	t.Run("owner prefixes the message and the hint stays separate", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "bad schema", schemaError("bad schema").Error())
		assert.Equal(t, "bad schema", schemaError("bad schema").withHint("fix it").Error())
		assert.Equal(t, "fix it", schemaError("bad schema").withHint("fix it").Hint())
		assert.Equal(t, "struct CLI: runner not implemented", schemaError("runner not implemented").
			ownedBy("CLI", "").
			withHint("implement Run").Error())
		assert.Equal(t, "struct CLI field Port: bad flag", schemaError("bad flag").ownedBy("CLI", "Port").withHint("try this").Error())
	})

	t.Run("note and hint stay separate from the message", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "bad schema", schemaError("bad schema").withNote("why it fails").Error())
		assert.Equal(t, "why it fails", schemaError("bad schema").withNote("why it fails").Note())

		err := schemaError("bad flag").
			ownedBy("CLI", "Port").
			withHint("fix it").
			withNote("why it fails")

		assert.Equal(t, "struct CLI field Port: bad flag", err.Error())
		assert.Equal(t, "why it fails", err.Note())
		assert.Equal(t, "fix it", err.Hint())
	})

	t.Run("accessors split the message, note, and hint", func(t *testing.T) {
		t.Parallel()

		err := schemaError("bad flag").ownedBy("CLI", "Port").withNote("why it fails").withHint("fix it")

		assert.Equal(t, "struct CLI field Port: bad flag", err.Message())
		assert.Equal(t, "why it fails", err.Note())
		assert.Equal(t, "fix it", err.Hint())
	})

	t.Run("WithOwner sets owner once and leaves owned error unchanged", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, WithOwner(nil, "CLI", "Port"))

		unowned := schemaError("bad flag").withHint("fix it")
		owned := WithOwner(unowned, "CLI", "Port")
		assert.Equal(t, "struct CLI field Port: bad flag", owned.Error())
		assert.Equal(t, "fix it", errlines.Hint(owned))
		assert.Equal(t, "bad flag", unowned.Error())
		assert.Equal(t, "fix it", errlines.Hint(unowned))

		stillOwned := WithOwner(owned, "Other", "Field")
		assert.Equal(t, "struct CLI field Port: bad flag", stillOwned.Error())
		assert.Equal(t, "fix it", errlines.Hint(stillOwned))

		plainCause := errors.New("raw error")
		wrapped := WithOwner(plainCause, "CLI", "Port")
		assert.True(t, IsSchemaError(wrapped))
		assert.IsError(t, wrapped, plainCause)
		assert.Equal(t, "struct CLI field Port: raw error", wrapped.Error())
	})

	t.Run("Unwrap returns nil when there is no cause", func(t *testing.T) {
		t.Parallel()

		err := schemaError("bad flag").ownedBy("CLI", "Port").withHint("fix it")
		assert.Zero(t, errors.Unwrap(err))
	})

	t.Run("feature default and enum preserve real cause through Unwrap", func(t *testing.T) {
		t.Parallel()

		badDefault := ValidateFeatureFieldTags(&FeatureField{Name: "depth", IsInt: true, Enum: []string{"1"}}, reflect.StructTag(`enum:"1" default:"02"`))
		assert.True(t, IsSchemaError(badDefault))
		_, hasIntegerCause := errors.AsType[*intx.Error](badDefault)
		assert.True(t, hasIntegerCause)
		assert.Equal(t, `invalid default "02" for feature "depth"`, badDefault.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(badDefault))
		assert.Equal(t, `write 0o2 for octal or 2 for decimal`, errlines.Hint(badDefault))

		badEnum := ValidateFeatureFieldTags(&FeatureField{Name: "depth", IsInt: true}, reflect.StructTag(`enum:"abc"`))
		assert.True(t, IsSchemaError(badEnum))
		_, hasEnumCause := errors.AsType[*intx.Error](badEnum)
		assert.True(t, hasEnumCause)
		assert.Equal(t, `invalid feature enum value "abc"`, badEnum.Error())
	})
}

type tagPiecesError struct {
	message string
	note    string
	hint    string
}

func (e *tagPiecesError) Error() string { return e.message }
func (e *tagPiecesError) Note() string  { return e.note }
func (e *tagPiecesError) Hint() string  { return e.hint }

func TestParseStrictStringTag_PreservesNoteAndHint(t *testing.T) {
	t.Parallel()

	cause := &tagPiecesError{
		message: "unknown unit",
		note:    "accepted units: B, kB",
		hint:    "use B or kB",
	}

	_, _, err := ParseStrictStringTag(reflect.StructTag(`units:"bad"`), TagSpec{
		Name: "units",
		Kind: TagKindString,
		Validate: func(string) error {
			return cause
		},
	})

	assert.True(t, IsSchemaError(err))
	assert.IsError(t, err, cause)
	assert.Equal(t, `tag "units": unknown unit`, err.Error())

	schemaErr, ok := errors.AsType[*SchemaError](err)
	assert.True(t, ok)
	assert.Equal(t, "accepted units: B, kB", schemaErr.Note())
	assert.Equal(t, "use B or kB", schemaErr.Hint())
}

func TestValidateScalarRange_BoundErrors(t *testing.T) {
	t.Parallel()

	type Float32 float32

	tests := []struct {
		name       string
		elemType   reflect.Type
		raw        string
		message    string
		note       string
		hint       string
		causeCheck func(t *testing.T, err error)
	}{
		{
			name:     "integer syntax",
			elemType: reflect.TypeFor[int](),
			raw:      "banana",
			message:  `invalid integer min bound "banana"`,
			causeCheck: func(t *testing.T, err error) {
				t.Helper()
				assert.IsError(t, err, strconv.ErrSyntax)
			},
		},
		{
			name:     "float syntax",
			elemType: reflect.TypeFor[float64](),
			raw:      "abc",
			message:  `invalid float min bound "abc"`,
			causeCheck: func(t *testing.T, err error) {
				t.Helper()
				assert.IsError(t, err, strconv.ErrSyntax)
			},
		},
		{
			name:     "duration unit",
			elemType: reflect.TypeFor[time.Duration](),
			raw:      "1x",
			message:  `invalid duration min bound "1x"`,
			causeCheck: func(t *testing.T, err error) {
				t.Helper()
				assert.Equal(t, `time: unknown unit "x" in duration "1x"`, errors.Unwrap(err).Error())
			},
		},
		{
			name:     "integer leading zero",
			elemType: reflect.TypeFor[int](),
			raw:      "07",
			message:  `invalid integer min bound "07"`,
			note:     "a leading zero is ambiguous",
			hint:     "write 0o7 for octal or 7 for decimal",
			causeCheck: func(t *testing.T, err error) {
				t.Helper()

				cause, ok := errors.AsType[*intx.Error](err)
				assert.True(t, ok)
				assert.Equal(t, intx.KindLeadingZero, cause.Kind)
			},
		},
		{
			name:     "integer out of range",
			elemType: reflect.TypeFor[int](),
			raw:      "99999999999999999999",
			message:  `invalid integer min bound "99999999999999999999"`,
			note:     "is out of range",
			causeCheck: func(t *testing.T, err error) {
				t.Helper()
				assert.IsError(t, err, strconv.ErrRange)
			},
		},
		{
			name:     "float32 out of range",
			elemType: reflect.TypeFor[Float32](),
			raw:      "1e39",
			message:  `invalid float min bound "1e39"`,
			note:     "is out of range",
			causeCheck: func(t *testing.T, err error) {
				t.Helper()
				assert.IsError(t, err, strconv.ErrRange)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rangeSpec := &Range{MinStr: test.raw, HasMin: true}

			err := validateRangeSpec(rangeSpec, test.elemType, nil, nil)

			assert.True(t, IsSchemaError(err))
			assert.Equal(t, test.message, err.Error())
			assert.Equal(t, test.note, errlines.Note(err))
			assert.Equal(t, test.hint, errlines.Hint(err))
			test.causeCheck(t, err)
		})
	}
}

func TestBoundError_TextDialectNotes(t *testing.T) {
	t.Parallel()

	t.Run("the parser's note and hint show through", func(t *testing.T) {
		t.Parallel()

		cause := noteHintCauseError{}
		err := boundError("byte size", "min", "5XB", cause)

		assert.Equal(t, `invalid byte size min bound "5XB"`, err.Error())
		assert.Equal(t, "cause note", err.Note())
		assert.Equal(t, "cause hint", err.Hint())
		assert.IsError(t, err, cause)
	})

	t.Run("a parser with no note supplies its message", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("not a temperature")
		err := boundError("value", "max", "hot", cause)

		assert.Equal(t, `invalid value max bound "hot"`, err.Error())
		assert.Equal(t, "not a temperature", err.Note())
		assert.Equal(t, "", err.Hint())
		assert.IsError(t, err, cause)
	})
}

func TestCountBoundError_KeepsItsCause(t *testing.T) {
	t.Parallel()

	t.Run("a leading zero takes the integer syntax's pieces", func(t *testing.T) {
		t.Parallel()

		err := ValidateCountRangeSpec(&Range{MinStr: "07", HasMin: true}, "len")

		assert.Equal(t, `invalid len min bound "07"`, err.Error())
		assert.Equal(t, "a leading zero is ambiguous", errlines.Note(err))
		assert.Equal(t, "write 0o7 for octal or 7 for decimal", errlines.Hint(err))

		_, ok := errors.AsType[*intx.Error](err)
		assert.True(t, ok)
	})

	t.Run("any other failure says what a bound must be", func(t *testing.T) {
		t.Parallel()

		err := ValidateCountRangeSpec(&Range{MaxStr: "-1", HasMax: true}, "items")

		assert.Equal(t, `invalid items max bound "-1"`, err.Error())
		assert.Equal(t, "must be non-negative integer", errlines.Note(err))
		assert.Equal(t, "", errlines.Hint(err))
		assert.IsError(t, err, strconv.ErrSyntax)
	})
}

func TestIntegerEnumDecimal_KeepsItsCause(t *testing.T) {
	t.Parallel()

	t.Run("out of range", func(t *testing.T) {
		t.Parallel()

		_, err := integerEnumDecimal("300", reflect.TypeFor[uint8]())

		assert.Equal(t, "enum value \"300\" is out of range for uint8", err.Error())
		assert.IsError(t, err, strconv.ErrRange)
	})

	t.Run("syntax", func(t *testing.T) {
		t.Parallel()

		_, err := integerEnumDecimal("07", reflect.TypeFor[int]())

		assert.Equal(t, "enum value \"07\" is not a valid int", err.Error())
		assert.Equal(t, "a leading zero is ambiguous", errlines.Note(err))
		assert.Equal(t, "write 0o7 for octal or 7 for decimal", errlines.Hint(err))

		_, ok := errors.AsType[*intx.Error](err)
		assert.True(t, ok)
	})
}

// noteHintCauseError is an error that supplies a note and a hint.
type noteHintCauseError struct{}

func (noteHintCauseError) Error() string { return "cause" }
func (noteHintCauseError) Note() string  { return "cause note" }
func (noteHintCauseError) Hint() string  { return "cause hint" }

// noteHinter mirrors conga.Noter and conga.Hinter, which this package cannot
// import.
type noteHinter interface {
	error
	Note() string
	Hint() string
}

func TestSchemaError_PiecesReachTheCause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *SchemaError
		wantNote string
		wantHint string
	}{
		{
			name:     "own values win",
			err:      schemaError("bad").withNote("own note").withHint("own hint").withCause(noteHintCauseError{}),
			wantNote: "own note",
			wantHint: "own hint",
		},
		{
			name:     "own empty, the cause supplies them",
			err:      schemaError("bad").withCause(noteHintCauseError{}),
			wantNote: "cause note",
			wantHint: "cause hint",
		},
		{
			name:     "own note only, the cause supplies the hint",
			err:      schemaError("bad").withNote("own note").withCause(noteHintCauseError{}),
			wantNote: "own note",
			wantHint: "cause hint",
		},
		{
			name: "both empty",
			err:  schemaError("bad").withCause(errors.New("plain")),
		},
		{
			name: "no cause",
			err:  schemaError("bad"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.wantNote, test.err.Note())
			assert.Equal(t, test.wantHint, test.err.Hint())

			found, ok := errors.AsType[noteHinter](test.err)
			assert.True(t, ok)
			assert.Equal(t, test.wantNote, found.Note())
			assert.Equal(t, test.wantHint, found.Hint())

			assert.Equal(t, test.wantNote, errlines.Note(test.err))
			assert.Equal(t, test.wantHint, errlines.Hint(test.err))
		})
	}
}

func TestWithOwner_KeepsTheCausesPieces(t *testing.T) {
	t.Parallel()

	err := WithOwner(noteHintCauseError{}, "CLI", "Port")

	assert.Equal(t, "struct CLI field Port: cause", err.Error())
	assert.Equal(t, "cause note", errlines.Note(err))
	assert.Equal(t, "cause hint", errlines.Hint(err))
	assert.IsError(t, err, noteHintCauseError{})
}
