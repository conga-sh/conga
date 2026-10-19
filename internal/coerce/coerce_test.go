package coerce_test

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/spec"
)

func TestCoerce(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Value[string]("hello")

		assert.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("bool true", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Value[bool]("true")

		assert.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("bool false variations", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []string{"false", "False", "FALSE", "f", "F", "0"} {
			result, err := coerce.Value[bool](raw)

			assert.NoError(t, err)
			assert.Equal(t, false, result)
		}
	})

	t.Run("bool true variations", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []string{"true", "True", "TRUE", "t", "T", "1", ""} {
			result, err := coerce.Value[bool](raw)

			assert.NoError(t, err)
			assert.Equal(t, true, result)
		}
	})

	t.Run("bool invalid", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[bool]("not-a-bool")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("bool invalid edge cases", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []string{" true ", " false ", "2", "-1", "00", "yes", "no", "on", "off", "enabled", "disabled"} {
			_, err := coerce.Value[bool](raw)

			assert.Error(t, err)
			assert.IsError(t, err, coerce.ErrCoerce)
		}
	})

	t.Run("int", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Value[int]("8080")

		assert.NoError(t, err)
		assert.Equal(t, 8080, result)
	})

	t.Run("duration", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Value[time.Duration]("45s")

		assert.NoError(t, err)
		assert.Equal(t, 45*time.Second, result)
	})

	t.Run("duration invalid", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[time.Duration]("not-a-duration")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("float64", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Value[float64]("3.1415")

		assert.NoError(t, err)
		assert.Equal(t, 3.1415, result)
	})

	t.Run("custom TextUnmarshaler pointer", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Value[customTextType]("testing")

		assert.NoError(t, err)
		assert.Equal(t, "custom:testing", result.value)
	})

	t.Run("custom TextUnmarshaler targetType is pointer", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Value[*customTextType]("testing-ptr")

		assert.NoError(t, err)
		assert.Equal(t, "custom:testing-ptr", result.value)
	})

	t.Run("custom TextUnmarshaler targetType is pointer error", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[*customTextErrType]("fail")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("custom TextUnmarshaler already ErrCoerce error", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[customTextCoerceErrType]("fail-direct")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})
}

func TestCoerce_UnsignedIntegers(t *testing.T) {
	t.Parallel()

	t.Run("uint boundary values", func(t *testing.T) {
		t.Parallel()

		zeroValue, errZero := coerce.Value[uint]("0")
		maxValue, errMax := coerce.Value[uint](strconv.FormatUint(math.MaxUint, 10))

		assert.NoError(t, errZero)
		assert.Equal(t, uint(0), zeroValue)
		assert.NoError(t, errMax)
		assert.Equal(t, uint(math.MaxUint), maxValue)
	})

	t.Run("uint8 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		zeroValue, errZero := coerce.Value[uint8]("0")
		maxValue, errMax := coerce.Value[uint8]("255")
		_, errOver := coerce.Value[uint8]("256")
		_, errNeg := coerce.Value[uint8]("-1")

		assert.NoError(t, errZero)
		assert.Equal(t, uint8(0), zeroValue)
		assert.NoError(t, errMax)
		assert.Equal(t, uint8(math.MaxUint8), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
		assert.Error(t, errNeg)
		assert.IsError(t, errNeg, coerce.ErrCoerce)
	})

	t.Run("uint16 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		zeroValue, errZero := coerce.Value[uint16]("0")
		maxValue, errMax := coerce.Value[uint16]("65535")
		_, errOver := coerce.Value[uint16]("65536")
		_, errNeg := coerce.Value[uint16]("-1")

		assert.NoError(t, errZero)
		assert.Equal(t, uint16(0), zeroValue)
		assert.NoError(t, errMax)
		assert.Equal(t, uint16(math.MaxUint16), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
		assert.Error(t, errNeg)
		assert.IsError(t, errNeg, coerce.ErrCoerce)
	})

	t.Run("uint32 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		zeroValue, errZero := coerce.Value[uint32]("0")
		maxValue, errMax := coerce.Value[uint32]("4294967295")
		_, errOver := coerce.Value[uint32]("4294967296")
		_, errNeg := coerce.Value[uint32]("-1")

		assert.NoError(t, errZero)
		assert.Equal(t, uint32(0), zeroValue)
		assert.NoError(t, errMax)
		assert.Equal(t, uint32(math.MaxUint32), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
		assert.Error(t, errNeg)
		assert.IsError(t, errNeg, coerce.ErrCoerce)
	})

	t.Run("uint64 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		zeroValue, errZero := coerce.Value[uint64]("0")
		maxValue, errMax := coerce.Value[uint64]("18446744073709551615")
		_, errOver := coerce.Value[uint64]("18446744073709551616")
		_, errNeg := coerce.Value[uint64]("-1")

		assert.NoError(t, errZero)
		assert.Equal(t, uint64(0), zeroValue)
		assert.NoError(t, errMax)
		assert.Equal(t, uint64(math.MaxUint64), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
		assert.Error(t, errNeg)
		assert.IsError(t, errNeg, coerce.ErrCoerce)
	})

	t.Run("malformed unsigned int", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[uint]("abc")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})
}

func TestCoerce_SignedIntegersAndFloats(t *testing.T) {
	t.Parallel()

	t.Run("int8 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		minValue, errMin := coerce.Value[int8]("-128")
		maxValue, errMax := coerce.Value[int8]("127")
		_, errOver := coerce.Value[int8]("128")
		_, errUnder := coerce.Value[int8]("-129")

		assert.NoError(t, errMin)
		assert.Equal(t, int8(math.MinInt8), minValue)
		assert.NoError(t, errMax)
		assert.Equal(t, int8(math.MaxInt8), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
		assert.Error(t, errUnder)
		assert.IsError(t, errUnder, coerce.ErrCoerce)
	})

	t.Run("int16 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		minValue, errMin := coerce.Value[int16]("-32768")
		maxValue, errMax := coerce.Value[int16]("32767")
		_, errOver := coerce.Value[int16]("32768")
		_, errUnder := coerce.Value[int16]("-32769")

		assert.NoError(t, errMin)
		assert.Equal(t, int16(math.MinInt16), minValue)
		assert.NoError(t, errMax)
		assert.Equal(t, int16(math.MaxInt16), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
		assert.Error(t, errUnder)
		assert.IsError(t, errUnder, coerce.ErrCoerce)
	})

	t.Run("int32 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		minValue, errMin := coerce.Value[int32]("-2147483648")
		maxValue, errMax := coerce.Value[int32]("2147483647")
		_, errOver := coerce.Value[int32]("2147483648")

		assert.NoError(t, errMin)
		assert.Equal(t, int32(math.MinInt32), minValue)
		assert.NoError(t, errMax)
		assert.Equal(t, int32(math.MaxInt32), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
	})

	t.Run("int64 boundaries and overflow", func(t *testing.T) {
		t.Parallel()

		minValue, errMin := coerce.Value[int64]("-9223372036854775808")
		maxValue, errMax := coerce.Value[int64]("9223372036854775807")
		_, errOver := coerce.Value[int64]("9223372036854775808")

		assert.NoError(t, errMin)
		assert.Equal(t, int64(math.MinInt64), minValue)
		assert.NoError(t, errMax)
		assert.Equal(t, int64(math.MaxInt64), maxValue)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
	})

	t.Run("signed int malformed input", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[int]("not-an-int")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("float32 boundary and malformed", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.Value[float32]("1.25")
		_, errMalformed := coerce.Value[float32]("1.2.3")
		_, errPrefix := coerce.Value[float32]("++1")
		_, errOver := coerce.Value[float32]("1e40")

		assert.NoError(t, err)
		assert.Equal(t, float32(1.25), value)
		assert.Error(t, errMalformed)
		assert.IsError(t, errMalformed, coerce.ErrCoerce)
		assert.Error(t, errPrefix)
		assert.IsError(t, errPrefix, coerce.ErrCoerce)
		assert.Error(t, errOver)
		assert.IsError(t, errOver, coerce.ErrCoerce)
	})

	t.Run("float64 malformed inputs", func(t *testing.T) {
		t.Parallel()

		_, errOneE := coerce.Value[float64]("1e")
		_, errDoubleSign := coerce.Value[float64]("+-1")

		assert.Error(t, errOneE)
		assert.IsError(t, errOneE, coerce.ErrCoerce)
		assert.Error(t, errDoubleSign)
		assert.IsError(t, errDoubleSign, coerce.ErrCoerce)
	})
}

func TestCoerce_NetworkingAndURL(t *testing.T) {
	t.Parallel()

	t.Run("url pointer valid", func(t *testing.T) {
		t.Parallel()

		parsed, err := coerce.Value[*url.URL]("https://example.com:8443/api?key=val#section")

		assert.NoError(t, err)
		assert.Equal(t, "https", parsed.Scheme)
		assert.Equal(t, "example.com:8443", parsed.Host)
		assert.Equal(t, "/api", parsed.Path)
		assert.Equal(t, "key=val", parsed.RawQuery)
		assert.Equal(t, "section", parsed.Fragment)
	})

	t.Run("url pointer missing scheme", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[*url.URL]("example.com/path")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("url pointer parse error", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[*url.URL]("://bad")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("url value valid", func(t *testing.T) {
		t.Parallel()

		parsed, err := coerce.Value[url.URL]("http://localhost:3000")

		assert.NoError(t, err)
		assert.Equal(t, "http", parsed.Scheme)
		assert.Equal(t, "localhost:3000", parsed.Host)
	})

	t.Run("url value missing scheme", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[url.URL]("localhost/path")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("net.IP valid and invalid", func(t *testing.T) {
		t.Parallel()

		ip4, err4 := coerce.Value[net.IP]("192.168.1.1")
		ip6, err6 := coerce.Value[net.IP]("2001:db8::1")
		_, errBad := coerce.Value[net.IP]("999.999.999.999")

		assert.NoError(t, err4)
		assert.Equal(t, net.ParseIP("192.168.1.1"), ip4)
		assert.NoError(t, err6)
		assert.Equal(t, net.ParseIP("2001:db8::1"), ip6)
		assert.Error(t, errBad)
		assert.IsError(t, errBad, coerce.ErrCoerce)
	})

	t.Run("netip.Addr valid and invalid", func(t *testing.T) {
		t.Parallel()

		addr, err := coerce.Value[netip.Addr]("10.0.0.1")
		_, errBad := coerce.Value[netip.Addr]("not-an-ip")

		assert.NoError(t, err)
		assert.Equal(t, netip.MustParseAddr("10.0.0.1"), addr)
		assert.Error(t, errBad)
		assert.IsError(t, errBad, coerce.ErrCoerce)
	})

	t.Run("netip.Prefix valid and invalid", func(t *testing.T) {
		t.Parallel()

		prefix, err := coerce.Value[netip.Prefix]("192.168.1.0/24")
		_, errBad := coerce.Value[netip.Prefix]("192.168.1.0/33")

		assert.NoError(t, err)
		assert.Equal(t, netip.MustParsePrefix("192.168.1.0/24"), prefix)
		assert.Error(t, errBad)
		assert.IsError(t, errBad, coerce.ErrCoerce)
	})
}

func TestCoerce_TextUnmarshalerAndReflection(t *testing.T) {
	t.Parallel()

	t.Run("TextUnmarshaler value receiver", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.Value[customTextValType]("value-receiver")

		assert.NoError(t, err)
		assert.Equal(t, customTextValType{}, value)
	})

	t.Run("TextUnmarshaler returns custom error", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Value[customTextErrType]("trigger-error")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
		assert.Equal(t, "custom unmarshal failed", err.Error())
	})

	t.Run("unsupported channel type", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Scalar("val", reflect.TypeFor[chan int]())

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrUnsupportedType)
	})

	t.Run("unsupported func type", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Scalar("val", reflect.TypeFor[func()]())

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrUnsupportedType)
	})

	t.Run("unsupported complex type", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Scalar("val", reflect.TypeFor[complex128]())

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrUnsupportedType)
	})

	t.Run("unsupported raw struct type", func(t *testing.T) {
		t.Parallel()

		type unmarshaledStruct struct {
			A string
		}

		_, err := coerce.Scalar("val", reflect.TypeFor[unmarshaledStruct]())

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrUnsupportedType)
	})
}

func TestCoerceSlice(t *testing.T) {
	t.Parallel()

	t.Run("comma separated ints", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.SliceWith([]string{"10,20,30"}, reflect.TypeFor[[]int](), ",", nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typed, ok := value.([]int)
		assert.True(t, ok)
		assert.Equal(t, []int{10, 20, 30}, typed)
	})

	t.Run("multiple string items", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.SliceWith([]string{"apple", "banana", "cherry"}, reflect.TypeFor[[]string](), ",", nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typed, ok := value.([]string)
		assert.True(t, ok)
		assert.Equal(t, []string{"apple", "banana", "cherry"}, typed)
	})

	t.Run("sep none preserves commas", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.SliceWith([]string{
			"Accept: text/html,application/xhtml+xml",
			"Authorization: Bearer token123",
		}, reflect.TypeFor[[]string](), "none", nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typed, ok := value.([]string)
		assert.True(t, ok)
		assert.Equal(t, []string{
			"Accept: text/html,application/xhtml+xml",
			"Authorization: Bearer token123",
		}, typed)
	})

	t.Run("sep none preserves string whitespace", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.SliceWith([]string{"MSG=  hello  "}, reflect.TypeFor[[]string](), "none", nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typed, ok := value.([]string)
		assert.True(t, ok)
		assert.Equal(t, []string{"MSG=  hello  "}, typed)
	})

	t.Run("sep none still trims non-string elements", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.SliceWith([]string{" 1 "}, reflect.TypeFor[[]int](), "none", nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typed, ok := value.([]int)
		assert.True(t, ok)
		assert.Equal(t, []int{1}, typed)
	})

	t.Run("empty sep keeps trimming string whitespace", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.SliceWith([]string{"  hello  "}, reflect.TypeFor[[]string](), "", nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typed, ok := value.([]string)
		assert.True(t, ok)
		assert.Equal(t, []string{"hello"}, typed)
	})
}

func TestCoerceMap(t *testing.T) {
	t.Parallel()

	t.Run("multiple key value pairs", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Map([]string{"category=work", "priority=high"}, ",")

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"category": "work",
			"priority": "high",
		}, result)
	})

	t.Run("comma separated key value pairs", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Map([]string{"env=prod,region=us-east-1"}, ",")

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"env":    "prod",
			"region": "us-east-1",
		}, result)
	})

	t.Run("invalid key value pair missing equals", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Map([]string{"invalid-pair"}, ",")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("empty key in pair", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Map([]string{"=value"}, ",")

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("sep disabled variations in map", func(t *testing.T) {
		t.Parallel()

		for _, disabledSep := range []string{"none", "off", "-", ""} {
			result, err := coerce.Map([]string{"filter=a,b,c"}, disabledSep)

			assert.NoError(t, err)
			assert.Equal(t, map[string]string{
				"filter": "a,b,c",
			}, result)
		}
	})

	t.Run("custom separator in map and slice", func(t *testing.T) {
		t.Parallel()

		resMap, err := coerce.Map([]string{"a=1;b=2"}, ";")

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"a": "1", "b": "2"}, resMap)

		resSlice, err := coerce.SliceWith([]string{"x;y;z"}, reflect.TypeFor[[]string](), ";", nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typedSlice, ok := resSlice.([]string)
		assert.True(t, ok)
		assert.Equal(t, []string{"x", "y", "z"}, typedSlice)
	})

	t.Run("value whitespace is trimmed", func(t *testing.T) {
		t.Parallel()

		result, err := coerce.Map([]string{"key = padded value"}, ",")

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "padded value"}, result)
	})
}

func TestCoerceURLAndNumericEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("url pointer valid", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.Scalar("https://example.com/path", reflect.TypeFor[*url.URL]())

		assert.NoError(t, err)

		parsed, ok := value.(*url.URL)
		assert.True(t, ok)
		assert.Equal(t, "https://example.com/path", parsed.String())
	})

	t.Run("url value valid", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.Scalar("https://example.com/val", reflect.TypeFor[url.URL]())

		assert.NoError(t, err)

		parsed, ok := value.(url.URL)
		assert.True(t, ok)
		assert.Equal(t, "https://example.com/val", parsed.String())
	})

	t.Run("url missing scheme rejected", func(t *testing.T) {
		t.Parallel()

		_, errPtr := coerce.Scalar("just-a-host", reflect.TypeFor[*url.URL]())
		_, errValue := coerce.Scalar("just-a-host", reflect.TypeFor[url.URL]())

		assert.Error(t, errPtr)
		assert.IsError(t, errPtr, coerce.ErrCoerce)
		assert.Error(t, errValue)
		assert.IsError(t, errValue, coerce.ErrCoerce)
	})

	t.Run("url invalid syntax rejected", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.Scalar("http://[fe80::1%en0]:invalid-port", reflect.TypeFor[*url.URL]())

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("signed int bit-sizes and boundaries", func(t *testing.T) {
		t.Parallel()

		val8, err := coerce.Scalar("127", reflect.TypeFor[int8]())
		assert.NoError(t, err)

		typed8, ok := val8.(int8)
		assert.True(t, ok)
		assert.Equal(t, int8(127), typed8)

		_, err8Overflow := coerce.Scalar("128", reflect.TypeFor[int8]())
		assert.Error(t, err8Overflow)
		assert.IsError(t, err8Overflow, coerce.ErrCoerce)

		val16, err := coerce.Scalar("32767", reflect.TypeFor[int16]())
		assert.NoError(t, err)

		typed16, ok := val16.(int16)
		assert.True(t, ok)
		assert.Equal(t, int16(32767), typed16)

		val32, err := coerce.Scalar("2147483647", reflect.TypeFor[int32]())
		assert.NoError(t, err)

		typed32, ok := val32.(int32)
		assert.True(t, ok)
		assert.Equal(t, int32(2147483647), typed32)

		val64, err := coerce.Scalar("9223372036854775807", reflect.TypeFor[int64]())
		assert.NoError(t, err)

		typed64, ok := val64.(int64)
		assert.True(t, ok)
		assert.Equal(t, int64(9223372036854775807), typed64)
	})

	t.Run("unsigned int bit-sizes and boundaries", func(t *testing.T) {
		t.Parallel()

		valueU8, err := coerce.Scalar("255", reflect.TypeFor[uint8]())
		assert.NoError(t, err)

		typedU8, ok := valueU8.(uint8)
		assert.True(t, ok)
		assert.Equal(t, uint8(255), typedU8)

		_, err = coerce.Scalar("256", reflect.TypeFor[uint8]())
		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)

		_, err = coerce.Scalar("-1", reflect.TypeFor[uint]())
		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("float bit-sizes and boundaries", func(t *testing.T) {
		t.Parallel()

		val32, err := coerce.Scalar("3.14", reflect.TypeFor[float32]())
		assert.NoError(t, err)

		typed32, ok := val32.(float32)
		assert.True(t, ok)
		assert.Equal(t, float32(3.14), typed32)

		val64, err := coerce.Scalar("2.718281828459045", reflect.TypeFor[float64]())
		assert.NoError(t, err)

		typed64, ok := val64.(float64)
		assert.True(t, ok)
		assert.Equal(t, 2.718281828459045, typed64)

		_, err = coerce.Scalar("not-a-number", reflect.TypeFor[float64]())
		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})
}

func TestCoerce_CustomRegistry(t *testing.T) {
	t.Parallel()

	parseID := func(raw string) (any, error) {
		if !strings.HasPrefix(raw, "id-") {
			return nil, errors.New("must start with id- prefix")
		}

		return testCustomID{value: raw}, nil
	}

	parseIDPtr := func(raw string) (any, error) {
		if !strings.HasPrefix(raw, "ptr-") {
			return nil, errors.New("must start with ptr- prefix")
		}

		return &testCustomID{value: raw}, nil
	}

	reg := coerce.Registry{
		reflect.TypeFor[testCustomID]():  parseID,
		reflect.TypeFor[*testCustomID](): parseIDPtr,
	}

	t.Run("scalar direct match valid", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.ScalarWith("id-123", reflect.TypeFor[testCustomID](), reg, coerce.DecodeContext{})
		assert.NoError(t, err)

		typed, ok := value.(testCustomID)
		assert.True(t, ok)
		assert.Equal(t, "id-123", typed.value)
	})

	t.Run("scalar direct match error", func(t *testing.T) {
		t.Parallel()

		_, err := coerce.ScalarWith("bad-prefix", reflect.TypeFor[testCustomID](), reg, coerce.DecodeContext{})
		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("pointer target resolved from scalar parser", func(t *testing.T) {
		t.Parallel()

		scalarRegOnly := coerce.Registry{
			reflect.TypeFor[testCustomID](): parseID,
		}

		value, err := coerce.ScalarWith("id-789", reflect.TypeFor[*testCustomID](), scalarRegOnly, coerce.DecodeContext{})
		assert.NoError(t, err)

		typed, ok := value.(*testCustomID)
		assert.True(t, ok)
		assert.Equal(t, "id-789", typed.value)
	})

	t.Run("scalar target resolved from pointer parser", func(t *testing.T) {
		t.Parallel()

		ptrRegOnly := coerce.Registry{
			reflect.TypeFor[*testCustomID](): parseIDPtr,
		}

		value, err := coerce.ScalarWith("ptr-456", reflect.TypeFor[testCustomID](), ptrRegOnly, coerce.DecodeContext{})
		assert.NoError(t, err)

		typed, ok := value.(testCustomID)
		assert.True(t, ok)
		assert.Equal(t, "ptr-456", typed.value)
	})

	t.Run("slice parsing with custom registry", func(t *testing.T) {
		t.Parallel()

		rawSlice := []string{"id-1", "id-2, id-3"}
		value, err := coerce.SliceWith(rawSlice, reflect.TypeFor[[]testCustomID](), ",", reg, coerce.DecodeContext{})
		assert.NoError(t, err)

		slice, ok := value.([]testCustomID)
		assert.True(t, ok)
		assert.Equal(t, 3, len(slice))
		assert.Equal(t, "id-1", slice[0].value)
		assert.Equal(t, "id-2", slice[1].value)
		assert.Equal(t, "id-3", slice[2].value)
	})

	t.Run("slice parsing error on invalid element", func(t *testing.T) {
		t.Parallel()

		rawSlice := []string{"id-1", "bad-element"}
		_, err := coerce.SliceWith(rawSlice, reflect.TypeFor[[]testCustomID](), ",", reg, coerce.DecodeContext{})
		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("format override takes precedence over default behavior", func(t *testing.T) {
		t.Parallel()

		overrideReg := coerce.Registry{
			reflect.TypeFor[int](): func(raw string) (any, error) {
				if !strings.HasPrefix(raw, "0x") {
					return nil, errors.New("must have 0x prefix")
				}

				parsed, err := strconv.ParseInt(raw[2:], 16, 64)
				if err != nil {
					return nil, err
				}

				return int(parsed), nil
			},
		}

		value, err := coerce.ScalarWith("0x1f", reflect.TypeFor[int](), overrideReg, coerce.DecodeContext{})

		assert.NoError(t, err)
		assert.Equal(t, 31, value)
	})

	t.Run("unregistered type falls back to standard scalar parsing", func(t *testing.T) {
		t.Parallel()

		reg := coerce.Registry{
			reflect.TypeFor[string](): func(raw string) (any, error) {
				return raw, nil
			},
		}

		value, err := coerce.ScalarWith("42", reflect.TypeFor[int](), reg, coerce.DecodeContext{})

		assert.NoError(t, err)
		assert.Equal(t, 42, value)
	})

	t.Run("parser returning non-assignable type yields error", func(t *testing.T) {
		t.Parallel()

		mismatchReg := coerce.Registry{
			reflect.TypeFor[int](): func(_ string) (any, error) {
				return "incompatible-string", nil
			},
		}

		_, err := coerce.ScalarWith("123", reflect.TypeFor[int](), mismatchReg, coerce.DecodeContext{})

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})

	t.Run("parser returning convertible but non-assignable type yields error", func(t *testing.T) {
		t.Parallel()

		mismatchReg := coerce.Registry{
			reflect.TypeFor[int](): func(_ string) (any, error) {
				return int64(42), nil
			},
		}

		_, err := coerce.ScalarWith("42", reflect.TypeFor[int](), mismatchReg, coerce.DecodeContext{})

		assert.Error(t, err)
		assert.IsError(t, err, coerce.ErrCoerce)
	})
}

type customTextType struct {
	value string
}

func (c *customTextType) UnmarshalText(text []byte) error {
	c.value = "custom:" + string(text)

	return nil
}

type customTextValType struct{}

func (c customTextValType) UnmarshalText(text []byte) error {
	return nil
}

type customTextErrType struct{}

func (c *customTextErrType) UnmarshalText(text []byte) error {
	return errors.New("custom unmarshal failed")
}

type customTextCoerceErrType struct{}

func (c *customTextCoerceErrType) UnmarshalText(text []byte) error {
	return fmt.Errorf("%w: direct coerce error", coerce.ErrCoerce)
}

// testTaggedValue implements both plain and context-aware text unmarshaling;
// the engine must prefer the context-aware form.
type testTaggedValue string

func (v *testTaggedValue) UnmarshalTextWithContext(text []byte, decodeContext coerce.DecodeContext) error {
	value := string(text)
	if decodeContext.Tags.Bool("upper") {
		value = strings.ToUpper(value)
	}

	*v = testTaggedValue(value)

	return nil
}

func (v *testTaggedValue) UnmarshalText(text []byte) error {
	*v = testTaggedValue("plain:" + string(text))

	return nil
}

func TestCoerceContextAwareTextUnmarshaler(t *testing.T) {
	t.Parallel()

	t.Run("scalar prefers context-aware unmarshal with declared tags", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.ScalarWith("abc", reflect.TypeFor[testTaggedValue](), nil, coerce.DecodeContext{Tags: spec.FieldTags{"upper": "true"}})

		assert.NoError(t, err)

		typed, ok := value.(testTaggedValue)
		assert.True(t, ok)
		assert.Equal(t, testTaggedValue("ABC"), typed)
	})

	t.Run("scalar prefers context-aware unmarshal without tags", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.ScalarWith("abc", reflect.TypeFor[testTaggedValue](), nil, coerce.DecodeContext{})

		assert.NoError(t, err)

		typed, ok := value.(testTaggedValue)
		assert.True(t, ok)
		assert.Equal(t, testTaggedValue("abc"), typed)
	})

	t.Run("slice elements receive the field tags via the decode context", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.SliceWith([]string{"a,b"}, reflect.TypeFor[[]testTaggedValue](), ",", nil, coerce.DecodeContext{Tags: spec.FieldTags{"upper": "true"}})

		assert.NoError(t, err)

		typed, ok := value.([]testTaggedValue)
		assert.True(t, ok)
		assert.Equal(t, []testTaggedValue{"A", "B"}, typed)
	})

	t.Run("Scalar delivers zero context to the context-aware unmarshaler", func(t *testing.T) {
		t.Parallel()

		value, err := coerce.Scalar("abc", reflect.TypeFor[testTaggedValue]())

		assert.NoError(t, err)

		typed, ok := value.(testTaggedValue)
		assert.True(t, ok)
		assert.Equal(t, testTaggedValue("abc"), typed)
	})
}

type testCustomID struct {
	value string
}
