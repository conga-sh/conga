// Package system provides domain types and autocompletion predictors for system
// users, groups, running processes, PIDs, POSIX signals, locales, and
// environment variables, plus optional standard library bindings for
// *user.User, *user.Group, and syscall.Signal fields.
//
// Every exported domain type implements encoding.TextUnmarshaler and Predict:
// fields declared with these types coerce automatically and complete from
// package tables or live system state. Invalid values classify under
// conga.ErrCoerce with the flag and reason named in the message. Group.Lookup
// and User.Lookup report a blank receiver as an error naming the failed lookup.
//
// Mount [RegisterUser], [RegisterGroup], or [RegisterSignal] to register the
// corresponding standard library types and enable completion on fields typed
// with them:
//
//	type CLI struct {
//		User   conga.OptionalFlag[system.User]    `help:"Run as user"`
//		Signal conga.OptionalFlag[syscall.Signal] `help:"Termination signal"`
//	}
//
//	app := conga.New[CLI]().Use(system.RegisterSignal)
//
// Signals accept POSIX names (SIGHUP, SIGINT, TERM) or numeric values (15).
// Users and groups resolve via os/user lookups.
//
// See https://docs.conga.sh/reference/domain-types/ for the domain type
// reference.
package system

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/signals"
	"github.com/conga-sh/conga/predict"
)

var (
	errEmptyGroup = errors.New("group lookup: empty group name")
	errEmptyUser  = errors.New("user lookup: empty username")
	errInvalidPID = errors.New("invalid PID")
)

// EnvVar represents an environment variable name with automatic environment completion.
type EnvVar string

// Predict returns environment variable autocompletion candidates.
func (e *EnvVar) Predict() predict.Predictor {
	return EnvVars()
}

// Help returns the default value syntax description for environment variable
// flags. The engine renders this text in the flag's help column when the field
// declares no help tag; a field with a help tag shows that text instead. Shells
// also show it as the description of the flag's completion candidates.
func (e *EnvVar) Help() string {
	return "Environment variable name (e.g. PATH)"
}

// String returns the environment variable name.
func (e *EnvVar) String() string {
	if e == nil {
		return ""
	}

	return string(*e)
}

// Value returns the value of the environment variable via os.Getenv.
func (e *EnvVar) Value() string {
	if e == nil {
		return ""
	}

	return os.Getenv(string(*e))
}

// Lookup queries the environment variable via os.LookupEnv.
func (e *EnvVar) Lookup() (string, bool) {
	if e == nil {
		return "", false
	}

	return os.LookupEnv(string(*e))
}

// UnmarshalText sets the environment variable name.
func (e *EnvVar) UnmarshalText(text []byte) error {
	*e = EnvVar(strings.TrimSpace(string(text)))

	return nil
}

// Group represents a system user group with automatic group completion.
type Group string

// Predict returns system group autocompletion candidates.
func (g *Group) Predict() predict.Predictor {
	return Groups()
}

// Help returns the default value syntax description for system group flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (g *Group) Help() string {
	return "System group name"
}

// String returns the group name.
func (g *Group) String() string {
	if g == nil {
		return ""
	}

	return string(*g)
}

// Lookup queries system group details using os/user.LookupGroup.
func (g *Group) Lookup() (*user.Group, error) {
	if g == nil || *g == "" {
		return nil, errEmptyGroup
	}

	grp, err := user.LookupGroup(string(*g))
	if err != nil {
		return nil, fmt.Errorf("group lookup: %w", err)
	}

	return grp, nil
}

// UnmarshalText sets the group name.
func (g *Group) UnmarshalText(text []byte) error {
	*g = Group(strings.TrimSpace(string(text)))

	return nil
}

// Locale represents an internationalization locale identifier with automatic locale completion.
type Locale string

// Predict returns locale autocompletion candidates.
func (l *Locale) Predict() predict.Predictor {
	return Locales()
}

// Help returns the default value syntax description for locale flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (l *Locale) Help() string {
	return "Locale identifier (e.g. en_US.UTF-8)"
}

// String returns the locale identifier.
func (l *Locale) String() string {
	if l == nil {
		return ""
	}

	return string(*l)
}

// UnmarshalText sets the locale identifier.
func (l *Locale) UnmarshalText(text []byte) error {
	*l = Locale(strings.TrimSpace(string(text)))

	return nil
}

// PID represents a positive process identifier with active process
// autocompletion. Its Predict hook pairs with ProcessIDs().
type PID int

// Predict returns active process ID autocompletion candidates.
func (p *PID) Predict() predict.Predictor {
	return ProcessIDs()
}

// Help returns the default value syntax description for PID flags. The engine
// renders this text in the flag's help column when the field declares no help
// tag; a field with a help tag shows that text instead. Shells also show it as
// the description of the flag's completion candidates.
func (p *PID) Help() string {
	return "Process ID (a positive integer)"
}

// Int returns the PID as an integer.
func (p *PID) Int() int {
	if p == nil {
		return 0
	}

	return int(*p)
}

// String returns the PID formatted as a string.
func (p *PID) String() string {
	if p == nil {
		return "0"
	}

	return strconv.Itoa(int(*p))
}

// UnmarshalText validates and parses a positive PID.
func (p *PID) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		*p = 0

		return nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("%w: PID %q (must be an integer): %w", errInvalidPID, raw, err)
	}

	if value <= 0 {
		return fmt.Errorf("%w: PID %d (must be a positive integer > 0)", errInvalidPID, value)
	}

	*p = PID(value)

	return nil
}

// Process represents a system process executable name with active process
// completion. The stored value is the executable name; its Predict hook pairs
// with ProcessNames().
type Process string

// Predict returns active process autocompletion candidates.
func (p *Process) Predict() predict.Predictor {
	return ProcessNames()
}

// Help returns the default value syntax description for process name flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (p *Process) Help() string {
	return "Process executable name (e.g. postgres)"
}

// String returns the process executable name.
func (p *Process) String() string {
	if p == nil {
		return ""
	}

	return string(*p)
}

// UnmarshalText sets the process executable name.
func (p *Process) UnmarshalText(text []byte) error {
	*p = Process(strings.TrimSpace(string(text)))

	return nil
}

// Signal represents a POSIX or system signal by name or numeric code.
type Signal string

// Predict returns kill signal autocompletion candidates.
func (s *Signal) Predict() predict.Predictor {
	return Signals()
}

// Help returns the default value syntax description for signal flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (s *Signal) Help() string {
	return "Signal name or number (e.g. SIGTERM, 9)"
}

// String returns the signal representation.
func (s *Signal) String() string {
	if s == nil {
		return ""
	}

	return string(*s)
}

// Signal converts the signal string into a syscall.Signal. It is a total
// conversion of a value already validated by UnmarshalText: a nil receiver,
// an empty value, and unrecognized text all yield 0. Callers that must reject
// unknown input validate it with UnmarshalText or the parser installed by
// RegisterSignal before converting; predictor callbacks have no error channel and ignore
// unknown text by construction.
func (s *Signal) Signal() syscall.Signal {
	if s == nil {
		return 0
	}

	sig, _ := signals.Lookup(string(*s))

	return sig
}

// UnmarshalText validates that the text represents a recognized signal.
func (s *Signal) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		*s = ""

		return nil
	}

	if _, ok := signals.Lookup(raw); !ok {
		return fmt.Errorf("%w %q", signals.ErrInvalid, raw)
	}

	*s = Signal(raw)

	return nil
}

// User represents a system username with automatic user autocompletion.
type User string

// Predict returns system username autocompletion candidates.
func (u *User) Predict() predict.Predictor {
	return Users()
}

// Help returns the default value syntax description for username flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (u *User) Help() string {
	return "System username"
}

// String returns the username.
func (u *User) String() string {
	if u == nil {
		return ""
	}

	return string(*u)
}

// Lookup queries system user details using os/user.Lookup.
func (u *User) Lookup() (*user.User, error) {
	if u == nil || *u == "" {
		return nil, errEmptyUser
	}

	usr, err := user.Lookup(string(*u))
	if err != nil {
		return nil, fmt.Errorf("user lookup: %w", err)
	}

	return usr, nil
}

// UnmarshalText sets the username.
func (u *User) UnmarshalText(text []byte) error {
	*u = User(strings.TrimSpace(string(text)))

	return nil
}

// RegisterGroup mounts *user.Group support onto the app builder: the parser
// resolves group names with os/user.LookupGroup and completion offers the
// host's groups. Apply with app.Use.
func RegisterGroup[T any](app *conga.App[T]) {
	app.RegisterType(user.LookupGroup, conga.WithPredictor(Groups()))
}

// RegisterSignal mounts syscall.Signal support onto the app builder: the parser
// accepts POSIX signal names (SIGHUP, TERM) and numeric values (15), and
// completion offers the platform's signal table. Apply with app.Use.
func RegisterSignal[T any](app *conga.App[T]) {
	app.RegisterType(parseSignal, conga.WithPredictor(Signals()))
}

// RegisterUser mounts *user.User support onto the app builder: the parser
// resolves usernames with os/user.Lookup and completion offers the host's
// users. Apply with app.Use.
func RegisterUser[T any](app *conga.App[T]) {
	app.RegisterType(user.Lookup, conga.WithPredictor(Users()))
}

// EnvVars configures completion of environment variable names currently defined
// in the execution environment or in the prediction context.
func EnvVars() predict.Predictor {
	return predict.Callback(actionEnvVars)
}

// Groups configures completion of system user groups.
func Groups() predict.Predictor {
	return predict.Callback(actionGroups)
}

// Locales configures completion of system and standard internationalization locale identifiers.
func Locales() predict.Predictor {
	return predict.Callback(actionLocales)
}

// ProcessIDs configures completion of active process IDs with their executable names as descriptions.
// Full process scanning is supported on Linux via /proc; on other platforms, it returns the current process.
// It pairs with the PID type.
func ProcessIDs() predict.Predictor {
	return predict.Callback(actionProcessIDs)
}

// ProcessNames configures completion of active running process executable names.
// Full process scanning is supported on Linux via /proc; on other platforms, it returns the current process.
// It pairs with the Process type, whose stored value is an executable name.
func ProcessNames() predict.Predictor {
	return predict.Callback(actionProcessNames)
}

// Signals configures completion of standard POSIX / system signals.
func Signals() predict.Predictor {
	return predict.Callback(actionSignals)
}

// Users configures completion of system user accounts.
func Users() predict.Predictor {
	return predict.Callback(actionUsers)
}

// parseSignal parses a signal by name (e.g. "SIGTERM", "TERM", "hup") or numeric string ("15").
func parseSignal(raw string) (syscall.Signal, error) {
	sig, ok := signals.Lookup(raw)
	if !ok {
		return 0, fmt.Errorf("%w %q", signals.ErrInvalid, raw)
	}

	return sig, nil
}
