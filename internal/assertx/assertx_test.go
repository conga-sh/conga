package assertx_test

import (
	"fmt"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/assertx"
)

type mockTB struct {
	testing.TB

	failed   bool
	fatalMsg string
}

func (m *mockTB) Helper() {}

func (m *mockTB) Fatal(args ...any) {
	m.failed = true
	m.fatalMsg = fmt.Sprint(args...)
}

func (m *mockTB) Fatalf(format string, args ...any) {
	m.failed = true
	m.fatalMsg = fmt.Sprintf(format, args...)
}

func TestPanicsWithValue_Success(t *testing.T) {
	t.Parallel()

	assertx.PanicsWithValue(t, "expected panic", func() {
		panic("expected panic")
	})
}

func TestPanicsWithValue_NoPanic(t *testing.T) {
	t.Parallel()

	mock := &mockTB{}

	assertx.PanicsWithValue(mock, "expected panic", func() {})

	assert.True(t, mock.failed)
	assert.Equal(t, "Expected function to panic", mock.fatalMsg)
}

func TestPanicsWithValue_WrongValue(t *testing.T) {
	t.Parallel()

	mock := &mockTB{}

	assertx.PanicsWithValue(mock, "expected panic", func() {
		panic("different panic")
	})

	assert.True(t, mock.failed)
}

func TestUnreachable_NoArgs(t *testing.T) {
	t.Parallel()

	mock := &mockTB{}

	assertx.Unreachable(mock)

	assert.True(t, mock.failed)
	assert.Equal(t, "Unreachable code reached", mock.fatalMsg)
}

func TestUnreachable_WithArgs(t *testing.T) {
	t.Parallel()

	mock := &mockTB{}

	assertx.Unreachable(mock, "custom branch reason")

	assert.True(t, mock.failed)
	assert.Equal(t, "Unreachable code reached: custom branch reason", mock.fatalMsg)
}
