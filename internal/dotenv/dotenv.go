// Package dotenv loads environment variables from .env files.
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
//
// Precedence:
// Files are applied in order: a key defined in more than one file takes the value from the
// last file that defines it, while keys already present in the process environment outrank
// every file.
//
// Variable references resolve from the process environment first, then from keys defined
// earlier in the same file; a file that overrides a key already supplied by an earlier file
// in the same call expands to its own value.
func Load(paths ...string) error {
	return load(os.Setenv, paths...)
}

// LoadWith loads environment variables from .env files using setenv to set each
// variable. Passing the method value of testing.T.Setenv in tests ensures the
// variables loaded from .env files are restored automatically when the test
// finishes. When setenv is nil, LoadWith behaves like [Load] and sets the
// variables through os.Setenv.
//
// Resolution follows the same hierarchy, precedence, variable expansion, and
// 12-factor preservation rules as [Load]; the process environment remains the
// host view, so a setenv that does not mutate it (a collecting callback) also
// does not participate in later files' expansion.
func LoadWith(setenv func(key, value string), paths ...string) error {
	if setenv == nil {
		return Load(paths...)
	}

	return load(func(key, value string) error {
		setenv(key, value)

		return nil
	}, paths...)
}

// Parse parses raw .env file contents into a key-value map.
// Supports comments, export prefixes, unquoted, single-quoted, and double-quoted values with escape sequences.
// Variable references resolve from the process environment first, then from keys defined earlier in the file.
func Parse(data []byte) (map[string]string, error) {
	return parse(data, nil)
}

func parse(data []byte, loadedKeys map[string]struct{}) (map[string]string, error) {
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
				result[key] = expandValue(value, result, loadedKeys)
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return result, nil
}

func load(set func(key, value string) error, paths ...string) error {
	loadedKeys := make(map[string]struct{})

	if len(paths) == 0 {
		return loadDefaultHierarchy(set, loadedKeys)
	}

	for _, path := range paths {
		if err := loadSingleFile(path, true, loadedKeys, set); err != nil {
			return err
		}
	}

	return nil
}

func loadDefaultHierarchy(set func(key, value string) error, loadedKeys map[string]struct{}) error {
	candidateFiles := []string{".env", ".env.local"}

	for _, filename := range candidateFiles {
		if err := loadSingleFile(filename, false, loadedKeys, set); err != nil {
			return err
		}
	}

	return nil
}

func loadSingleFile(path string, mustExist bool, loadedKeys map[string]struct{}, set func(key, value string) error) error {
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

	envMap, err := parse(data, loadedKeys)
	if err != nil {
		return &DotenvError{
			Path:  path,
			Msg:   fmt.Sprintf("cannot parse .env file %q: %v", path, err),
			Cause: fmt.Errorf("%w: %w", sentinel.ErrDotenvParse, err),
		}
	}

	for key, value := range envMap {
		if _, exists := os.LookupEnv(key); exists {
			if _, loadedByFile := loadedKeys[key]; !loadedByFile {
				continue
			}
		}

		if err := set(key, value); err != nil {
			return &DotenvError{
				Path:  path,
				Msg:   fmt.Sprintf("cannot set environment variable %q: %v", key, err),
				Cause: fmt.Errorf("%w: %w", sentinel.ErrDotenv, err),
			}
		}

		loadedKeys[key] = struct{}{}
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

func expandValue(value string, localMap map[string]string, loadedKeys map[string]struct{}) string {
	return os.Expand(value, func(key string) string {
		localValue, locallyDefined := localMap[key]
		if _, overriddenEarlier := loadedKeys[key]; overriddenEarlier && locallyDefined {
			return localValue
		}

		if hostValue, hostDefined := os.LookupEnv(key); hostDefined {
			return hostValue
		}

		return localValue
	})
}
