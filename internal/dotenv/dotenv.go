package dotenv

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/conga-sh/conga/internal/filex"
	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/sentinel"
)

const maxDotenvFileSize = 16 * 1024 * 1024

// DotenvError represents an error reading or parsing a .env file.
type DotenvError struct {
	Path  string
	Msg   string
	Cause error
}

func (e *DotenvError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}

	if e.Path != "" && e.Cause != nil {
		return fmt.Sprintf("dotenv file %q: %v", e.Path, e.Cause)
	}

	return "dotenv error"
}

func (e *DotenvError) Unwrap() error {
	if e.Cause != nil {
		return e.Cause
	}

	return sentinel.ErrDotenv
}

func (e *DotenvError) Is(target error) bool {
	if target == sentinel.ErrDotenv {
		return true
	}

	if e.Cause != nil {
		return errors.Is(e.Cause, target)
	}

	return false
}

// Load loads environment variables from .env files into the current process environment.
//
// 12-Factor Preservation:
// Existing OS environment variables are never overwritten.
//
// Hierarchy:
// When paths is empty, Load automatically loads ".env" followed by ".env.local" from the
// current working directory if they exist (silently ignoring missing files).
// When explicit paths are provided, each file must exist or a DotenvError is returned.
func Load(paths ...string) error {
	return loadFiles(false, paths...)
}

// Overload loads environment variables from .env files into the current process environment,
// overwriting existing environment variables.
//
// Live Reload:
// Used during dynamic configuration reloading to ensure on-disk .env updates take effect.
//
// Hierarchy:
// When paths is empty, Overload automatically loads ".env" followed by ".env.local" from the
// current working directory if they exist (silently ignoring missing files).
// When explicit paths are provided, each file must exist or a DotenvError is returned.
func Overload(paths ...string) error {
	return loadFiles(true, paths...)
}

// Parse parses raw .env file contents into a key-value map.
// Supports comments, export prefixes, unquoted, single-quoted, and double-quoted values with escape sequences.
func Parse(data []byte) (map[string]string, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	result := make(map[string]string)

	lineNum := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("%w: %w", sentinel.ErrDotenvParse, err)
		}

		lineNum++
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and full comment lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if errors.Is(err, io.EOF) {
				break
			}

			continue
		}

		key, value, isLiteral, parseErr := parseLine(trimmed, reader, &lineNum)
		if parseErr != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, parseErr)
		}

		if key != "" {
			if isLiteral {
				result[key] = value
			} else {
				// Expand variables: ${VAR} or $VAR
				result[key] = expandValue(value, result)
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return result, nil
}

func loadFiles(overwrite bool, paths ...string) error {
	if len(paths) == 0 {
		return loadDefaultHierarchy(overwrite)
	}

	for _, path := range paths {
		if err := loadSingleFile(path, true, overwrite); err != nil {
			return err
		}
	}

	return nil
}

func loadDefaultHierarchy(overwrite bool) error {
	candidateFiles := []string{".env", ".env.local"}

	for _, filename := range candidateFiles {
		if _, err := os.Stat(filename); err == nil {
			if err := loadSingleFile(filename, false, overwrite); err != nil {
				return err
			}
		}
	}

	return nil
}

func loadSingleFile(path string, mustExist bool, overwrite bool) error {
	cleanPath := pathx.Expand(path)

	data, err := filex.ReadLimited(cleanPath, maxDotenvFileSize)
	if err != nil {
		if !mustExist && errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return &DotenvError{
			Path:  path,
			Msg:   fmt.Sprintf("cannot read .env file %q: %v", path, err),
			Cause: fmt.Errorf("%w: %w", sentinel.ErrDotenvRead, err),
		}
	}

	envMap, err := Parse(data)
	if err != nil {
		return &DotenvError{
			Path:  path,
			Msg:   fmt.Sprintf("cannot parse .env file %q: %v", path, err),
			Cause: fmt.Errorf("%w: %w", sentinel.ErrDotenvParse, err),
		}
	}

	for key, value := range envMap {
		if !overwrite {
			if _, exists := os.LookupEnv(key); exists {
				continue
			}
		}

		if err := os.Setenv(key, value); err != nil {
			return &DotenvError{
				Path:  path,
				Msg:   fmt.Sprintf("cannot set environment variable %q: %v", key, err),
				Cause: fmt.Errorf("%w: %w", sentinel.ErrDotenv, err),
			}
		}
	}

	return nil
}

func parseLine(line string, reader *bufio.Reader, lineNum *int) (string, string, bool, error) {
	// Strip optional "export " prefix
	if after, ok := strings.CutPrefix(line, "export "); ok {
		line = strings.TrimSpace(after)
	}

	keyPart, rawValPart, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false, fmt.Errorf("%w: missing '=' in statement", sentinel.ErrDotenvParse)
	}

	key := strings.TrimSpace(keyPart)
	if key == "" {
		return "", "", false, fmt.Errorf("%w: empty key name", sentinel.ErrDotenvParse)
	}

	// Validate key contains valid identifier characters
	for index, r := range key {
		if index == 0 && !unicode.IsLetter(r) && r != '_' {
			return "", "", false, fmt.Errorf("%w: invalid character %q at start of key", sentinel.ErrDotenvParse, r)
		}

		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' {
			return "", "", false, fmt.Errorf("%w: invalid character %q in key", sentinel.ErrDotenvParse, r)
		}
	}

	rawValue := strings.TrimSpace(rawValPart)

	// Parse value depending on quoting
	value, isLiteral, err := parseValue(rawValue, reader, lineNum)
	if err != nil {
		return "", "", false, err
	}

	return key, value, isLiteral, nil
}

func parseValue(rawValue string, reader *bufio.Reader, lineNum *int) (string, bool, error) {
	if rawValue == "" {
		return "", false, nil
	}

	firstChar := rawValue[0]

	if firstChar == '\'' {
		value, err := parseSingleQuoted(rawValue, reader, lineNum)

		return value, true, err
	}

	if firstChar == '"' {
		value, err := parseDoubleQuoted(rawValue, reader, lineNum)

		return value, false, err
	}

	// Unquoted value: strip trailing inline comments
	return parseUnquoted(rawValue), false, nil
}

func parseSingleQuoted(rawValue string, reader *bufio.Reader, lineNum *int) (string, error) {
	var builder strings.Builder

	cur := rawValue[1:]

	for {
		closeIdx := strings.IndexByte(cur, '\'')
		if closeIdx >= 0 {
			builder.WriteString(cur[:closeIdx])

			// Rest after closing quote must be comment or whitespace
			rest := strings.TrimSpace(cur[closeIdx+1:])
			if rest != "" && !strings.HasPrefix(rest, "#") {
				return "", fmt.Errorf("%w: unexpected characters after closing quote: %s", sentinel.ErrDotenvParse, rest)
			}

			return builder.String(), nil
		}

		builder.WriteString(strings.TrimRight(cur, "\r\n"))
		builder.WriteByte('\n')

		nextLine, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("%w: unclosed single quote", sentinel.ErrDotenvParse)
		}

		(*lineNum)++
		cur = nextLine
	}
}

func parseDoubleQuoted(rawValue string, reader *bufio.Reader, lineNum *int) (string, error) {
	var builder strings.Builder

	cur := rawValue[1:] // skip leading double quote

	for {
		cleanCur := strings.TrimRight(cur, "\r\n")

		closed, err := processDoubleQuotedLine(cleanCur, &builder)
		if err != nil {
			return "", err
		}

		if closed {
			return builder.String(), nil
		}

		builder.WriteByte('\n')

		nextLine, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("%w: unclosed double quote", sentinel.ErrDotenvParse)
		}

		(*lineNum)++
		cur = nextLine
	}
}

func processDoubleQuotedLine(cleanCur string, builder *strings.Builder) (bool, error) {
	escaped := false

	for index := range len(cleanCur) {
		char := cleanCur[index]

		if escaped {
			writeEscapedChar(char, builder)

			escaped = false

			continue
		}

		if char == '\\' {
			escaped = true

			continue
		}

		if char == '"' {
			rest := strings.TrimSpace(cleanCur[index+1:])
			if rest != "" && !strings.HasPrefix(rest, "#") {
				return false, fmt.Errorf("%w: unexpected characters after closing quote: %s", sentinel.ErrDotenvParse, rest)
			}

			return true, nil
		}

		builder.WriteByte(char)
	}

	return false, nil
}

func writeEscapedChar(char byte, builder *strings.Builder) {
	switch char {
	case 'n':
		builder.WriteByte('\n')
	case 'r':
		builder.WriteByte('\r')
	case 't':
		builder.WriteByte('\t')
	case '"':
		builder.WriteByte('"')
	case '\\':
		builder.WriteByte('\\')
	case '$':
		builder.WriteByte('$')
	default:
		builder.WriteByte('\\')
		builder.WriteByte(char)
	}
}

func parseUnquoted(rawValue string) string {
	// Strip trailing inline comments (separated by space or tab before #)
	for index := range rawValue {
		if rawValue[index] == '#' && index > 0 && (rawValue[index-1] == ' ' || rawValue[index-1] == '\t') {
			rawValue = rawValue[:index]

			break
		}
	}

	return strings.TrimSpace(rawValue)
}

func expandValue(value string, localMap map[string]string) string {
	return os.Expand(value, func(key string) string {
		if v, ok := localMap[key]; ok {
			return v
		}

		return os.Getenv(key)
	})
}
