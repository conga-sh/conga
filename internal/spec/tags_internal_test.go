package spec

import (
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// ByteSize mirrors the root ByteSize domain type by name: validateRangeSpec
// dispatches byte-size handling on the element type's bare name, so a local
// fixture with the same name is required to exercise that branch.
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

func TestValidateRangeSpec_ByteSizeDispatch(t *testing.T) {
	t.Parallel()

	t.Run("valid byte size range accepted", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "100", MaxStr: "200", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize]())

		assert.NoError(t, err)
	})

	t.Run("inverted byte size range rejected with exact message", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "200", MaxStr: "100", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize]())

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidRangeSyntax)
		assert.Equal(t, "schema error: invalid range syntax: min bound \"200\" cannot be greater than max bound \"100\"", err.Error())
	})

	t.Run("unparseable byte size bound rejected with exact message", func(t *testing.T) {
		t.Parallel()

		rangeSpec := &Range{MinStr: "banana", HasMin: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize]())

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidRangeSyntax)
		assert.Equal(t, "schema error: invalid range syntax: invalid byte size min bound \"banana\"", err.Error())
	})

	t.Run("byte-size-named type without TextUnmarshaler silently accepts bounds", func(t *testing.T) {
		t.Parallel()

		type ByteSize struct{}

		rangeSpec := &Range{MinStr: "200", MaxStr: "100", HasMin: true, HasMax: true}

		err := validateRangeSpec(rangeSpec, reflect.TypeFor[ByteSize]())

		assert.NoError(t, err)
	})
}

func TestExtractByteSize(t *testing.T) {
	t.Parallel()

	t.Run("Bytes fast path", func(t *testing.T) {
		t.Parallel()

		size := ByteSize(4096)

		assert.Equal(t, int64(4096), extractByteSize(&size))
	})

	t.Run("signed integer fallback", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, int64(-7), extractByteSize(int64(-7)))
		assert.Equal(t, int64(9), extractByteSize(int(9)))
		assert.Equal(t, int64(5), extractByteSize(ByteSize(5)))
	})

	t.Run("unsigned integer fallback", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, int64(11), extractByteSize(uint64(11)))
		assert.Equal(t, int64(3), extractByteSize(uint8(3)))
	})

	t.Run("uint beyond int64 max clamps to MaxInt64", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, int64(math.MaxInt64), extractByteSize(uint64(math.MaxInt64+1)))
	})

	t.Run("pointer dereferenced before extraction", func(t *testing.T) {
		t.Parallel()

		raw := uint64(15)

		assert.Equal(t, int64(15), extractByteSize(&raw))
	})

	t.Run("unsupported kind returns zero", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, int64(0), extractByteSize(struct{ Name string }{Name: "x"}))
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
			assert.IsError(t, err, ErrInvalidRangeSyntax)
		})
	}
}

func TestParsePresenceOrTrueTag(t *testing.T) {
	t.Parallel()

	t.Run("tag missing returns false", func(t *testing.T) {
		t.Parallel()

		assert.False(t, parsePresenceOrTrueTag(reflect.StructTag(`other:"val"`), "hidden"))
	})

	t.Run("bare tag returns true", func(t *testing.T) {
		t.Parallel()

		assert.True(t, parsePresenceOrTrueTag(reflect.StructTag(`hidden:""`), "hidden"))
	})

	t.Run("explicit true returns true", func(t *testing.T) {
		t.Parallel()

		assert.True(t, parsePresenceOrTrueTag(reflect.StructTag(`hidden:"true"`), "hidden"))
	})

	t.Run("explicit false returns false", func(t *testing.T) {
		t.Parallel()

		assert.False(t, parsePresenceOrTrueTag(reflect.StructTag(`hidden:"false"`), "hidden"))
	})

	t.Run("arbitrary string returns false", func(t *testing.T) {
		t.Parallel()

		assert.False(t, parsePresenceOrTrueTag(reflect.StructTag(`hidden:"yes"`), "hidden"))
	})
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
		assert.IsError(t, err, ErrInvalidStructTag)
	})

	t.Run("missing value rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseStructTagKeys(reflect.StructTag(`name`))

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
	})

	t.Run("unterminated quote rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseStructTagKeys(reflect.StructTag(`name:"unterminated`))

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
	})

	t.Run("unquoted value rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseStructTagKeys(reflect.StructTag(`name:bare`))

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
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
			assert.IsError(t, err, ErrInvalidStructTag)
			assert.Equal(t, `schema error: invalid struct tag: empty glob pattern element (expected comma-separated patterns like "Makefile,Dockerfile" or "*.yaml")`, err.Error())
		}
	})

	t.Run("forward path separator rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseGlobPatterns("src/*.go")

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, `schema error: invalid struct tag: glob pattern "src/*.go" must not contain path separators (patterns match file names during completion traversal)`, err.Error())
	})

	t.Run("backslash path separator rejected with exact message", func(t *testing.T) {
		t.Parallel()

		_, err := ParseGlobPatterns(`Make\file`)

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
		assert.Equal(t, "schema error: invalid struct tag: glob pattern \"Make\\\\file\" must not contain path separators (patterns match file names during completion traversal)", err.Error())
	})

	t.Run("malformed pattern rejected with exact message", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"[", "a[b", "*[a-"} {
			_, err := ParseGlobPatterns(value)

			assert.Error(t, err)
			assert.IsError(t, err, ErrInvalidStructTag)
			assert.Equal(t, `schema error: invalid struct tag: glob pattern "`+value+`" is malformed: syntax error in pattern`, err.Error())
		}
	})
}
