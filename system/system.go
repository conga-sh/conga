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
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/scanx"
	"github.com/conga-sh/conga/internal/signals"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/style"
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

type procEntry struct {
	Pid int
	Exe string
}

func actionEnvVars(ctx predict.Context) predict.Predictor {
	items := scanEnvVars(ctx)

	return predict.Items(items...)
}

func actionGroups(_ predict.Context) predict.Predictor {
	items := scanGroups()

	return predict.Items(items...)
}

func actionLocales(_ predict.Context) predict.Predictor {
	items := scanLocales()

	return predict.Items(items...)
}

func actionProcessIDs(_ predict.Context) predict.Predictor {
	items := slicex.Map(scanRunningProcesses(), func(proc procEntry) predict.Item {
		pidStr := strconv.Itoa(proc.Pid)

		return predict.Item{
			Value:       pidStr,
			Display:     pidStr,
			Description: proc.Exe,
			Style:       style.Magenta,
			Tag:         "pids",
		}
	})

	return predict.Items(items...)
}

func actionProcessNames(_ predict.Context) predict.Predictor {
	procs := scanRunningProcesses()

	counts := make(map[string]int)
	order := make([]string, 0, len(procs))

	for _, proc := range procs {
		name := proc.Exe
		if name == "" {
			continue
		}

		if counts[name] == 0 {
			order = append(order, name)
		}

		counts[name]++
	}

	slices.Sort(order)

	items := make([]predict.Item, 0, len(order))
	for _, name := range order {
		desc := "1 instance"
		if count := counts[name]; count > 1 {
			desc = fmt.Sprintf("%d instances", count)
		}

		items = append(items, predict.Item{
			Value:       name,
			Display:     name,
			Description: desc,
			Style:       style.Green,
			Tag:         "processes",
		})
	}

	return predict.Items(items...)
}

func actionSignals(ctx predict.Context) predict.Predictor {
	knownSignals := signals.Known()
	items := make([]predict.Item, 0, len(knownSignals)*3)
	seen := make(map[string]bool)

	toComplete := ctx.ToComplete
	upperToComplete := strings.ToUpper(toComplete)

	for _, sig := range knownSignals {
		candidates := []struct {
			value string
			desc  string
		}{
			{value: sig.Name, desc: sig.Description},
			{value: sig.ShortName, desc: sig.Description},
			{value: strconv.Itoa(int(sig.Signal)), desc: sig.Name + " (" + sig.Description + ")"},
		}

		for _, cand := range candidates {
			if seen[cand.value] {
				continue
			}

			if toComplete != "" && !strings.HasPrefix(strings.ToUpper(cand.value), upperToComplete) {
				continue
			}

			seen[cand.value] = true
			items = append(items, predict.Item{
				Value:       cand.value,
				Display:     cand.value,
				Description: cand.desc,
				Style:       sig.Style,
				Tag:         "signals",
			})
		}
	}

	return predict.Items(items...)
}

func actionUsers(_ predict.Context) predict.Predictor {
	items := scanUsers()

	return predict.Items(items...)
}

func fallbackLocales() []string {
	return []string{
		"C", "C.UTF-8", "POSIX",
		"en_US.UTF-8", "en_GB.UTF-8", "en_CA.UTF-8", "en_AU.UTF-8",
		"de_DE.UTF-8", "de_AT.UTF-8", "de_CH.UTF-8",
		"fr_FR.UTF-8", "fr_CA.UTF-8", "fr_BE.UTF-8", "fr_CH.UTF-8",
		"es_ES.UTF-8", "es_MX.UTF-8", "es_AR.UTF-8", "es_CL.UTF-8",
		"it_IT.UTF-8", "it_CH.UTF-8",
		"pt_PT.UTF-8", "pt_BR.UTF-8",
		"nl_NL.UTF-8", "nl_BE.UTF-8",
		"ru_RU.UTF-8", "zh_CN.UTF-8", "zh_TW.UTF-8", "zh_HK.UTF-8",
		"ja_JP.UTF-8", "ko_KR.UTF-8", "pl_PL.UTF-8", "sv_SE.UTF-8",
		"da_DK.UTF-8", "fi_FI.UTF-8", "nb_NO.UTF-8", "nn_NO.UTF-8",
		"cs_CZ.UTF-8", "el_GR.UTF-8", "he_IL.UTF-8", "hi_IN.UTF-8",
		"hu_HU.UTF-8", "id_ID.UTF-8", "tr_TR.UTF-8", "uk_UA.UTF-8",
		"vi_VN.UTF-8",
	}
}

func parseGroupReader(r io.Reader) []predict.Item {
	seen := make(map[string]bool)

	var items []predict.Item

	if err := scanx.ScanLines(r, func(line string) bool {
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			return true
		}

		name := strings.TrimSpace(parts[0])
		if name == "" || seen[name] {
			return true
		}

		seen[name] = true
		gid := strings.TrimSpace(parts[2])

		desc := "GID: " + gid
		if len(parts) >= 4 && strings.TrimSpace(parts[3]) != "" {
			desc += fmt.Sprintf(" (%s)", strings.TrimSpace(parts[3]))
		}

		items = append(items, predict.Item{
			Value:       name,
			Display:     name,
			Description: desc,
			Style:       style.Cyan,
			Tag:         "user groups",
		})

		return true
	}); err != nil {
		return items
	}

	return items
}

func parsePasswdReader(r io.Reader) []predict.Item {
	seen := make(map[string]bool)

	var items []predict.Item

	if err := scanx.ScanLines(r, func(line string) bool {
		parts := strings.Split(line, ":")
		if len(parts) < 6 {
			return true
		}

		name := strings.TrimSpace(parts[0])
		if name == "" || seen[name] {
			return true
		}

		seen[name] = true
		gecos := strings.TrimSpace(parts[4])

		if index := strings.Index(gecos, ","); index != -1 {
			gecos = strings.TrimSpace(gecos[:index])
		}

		desc := gecos
		if desc == "" {
			desc = strings.TrimSpace(parts[5]) // Home directory
		}

		items = append(items, predict.Item{
			Value:       name,
			Display:     name,
			Description: desc,
			Style:       style.Blue,
			Tag:         "users",
		})

		return true
	}); err != nil {
		return items
	}

	return items
}

func scanEnvVars(ctx predict.Context) []predict.Item {
	var envList []string

	if len(ctx.Env) > 0 {
		for k, v := range ctx.Env {
			envList = append(envList, k+"="+v)
		}
	} else {
		envList = os.Environ()
	}

	slices.Sort(envList)

	hasDollarPrefix := strings.HasPrefix(ctx.ToComplete, "$")

	seen := make(map[string]bool)

	var items []predict.Item

	for _, entry := range envList {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		if key == "" || seen[key] {
			continue
		}

		seen[key] = true

		valueRunes := []rune(value)
		if len(valueRunes) > 40 {
			value = string(valueRunes[:37]) + "..."
		}

		valueCandidate := key
		if hasDollarPrefix {
			valueCandidate = "$" + key
		}

		items = append(items, predict.Item{
			Value:       valueCandidate,
			Display:     valueCandidate,
			Description: value,
			Style:       style.Yellow,
			Tag:         "environment variables",
		})
	}

	return items
}

func scanGroups() []predict.Item {
	if runtime.GOOS != "windows" {
		if file, err := os.Open("/etc/group"); err == nil {
			items := parseGroupReader(file)
			_ = file.Close() //nolint:errcheck // Read-only file close.

			return items
		}
	}

	var items []predict.Item

	if curr, err := user.Current(); err == nil && curr.Gid != "" {
		groupName := curr.Username
		if grp, err := user.LookupGroupId(curr.Gid); err == nil && grp.Name != "" {
			groupName = grp.Name
		}

		items = append(items, predict.Item{
			Value:       groupName,
			Display:     groupName,
			Description: "Current user group",
			Style:       style.Cyan,
			Tag:         "user groups",
		})
	}

	return items
}

func scanI18nLocales() []string {
	entries, err := os.ReadDir("/usr/share/i18n/locales")
	if err != nil {
		return nil
	}

	var locales []string

	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			locales = append(locales, entry.Name())
		}
	}

	return locales
}

func scanLocaleGen() []string {
	file, err := os.Open("/etc/locale.gen")
	if err != nil {
		return nil
	}

	defer func() {
		_ = file.Close() //nolint:errcheck // Read-only file close.
	}()

	var locales []string

	if err := scanx.ScanLines(file, func(line string) bool {
		parts := strings.Fields(line)
		if len(parts) > 0 {
			locales = append(locales, parts[0])
		}

		return true
	}); err != nil {
		return nil
	}

	return locales
}

func scanLocales() []predict.Item {
	locales := scanLocaleGen()
	if len(locales) == 0 {
		locales = scanI18nLocales()
	}

	if len(locales) == 0 {
		locales = fallbackLocales()
	}

	seen := make(map[string]bool)

	var items []predict.Item

	for _, loc := range locales {
		if loc == "" || seen[loc] {
			continue
		}

		seen[loc] = true

		items = append(items, predict.Item{
			Value:   loc,
			Display: loc,
			Style:   style.Cyan,
			Tag:     "locales",
		})
	}

	return items
}

func scanLinuxProc() []procEntry {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var procs []procEntry

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		commPath := filepath.Clean(fmt.Sprintf("/proc/%s/comm", entry.Name()))

		commBytes, err := os.ReadFile(commPath)
		if err != nil {
			continue
		}

		exe := strings.TrimSpace(string(commBytes))
		if exe != "" {
			procs = append(procs, procEntry{Pid: pid, Exe: exe})
		}
	}

	return procs
}

func scanRunningProcesses() []procEntry {
	if runtime.GOOS == "linux" {
		if procs := scanLinuxProc(); len(procs) > 0 {
			return procs
		}
	}

	pid := os.Getpid()
	exe := "conga"

	if selfExe, err := os.Executable(); err == nil {
		exe = filepath.Base(selfExe)
	}

	return []procEntry{{Pid: pid, Exe: exe}}
}

func scanUsers() []predict.Item {
	if runtime.GOOS != "windows" {
		if file, err := os.Open("/etc/passwd"); err == nil {
			items := parsePasswdReader(file)
			_ = file.Close() //nolint:errcheck // Read-only file close.

			return items
		}
	}

	var items []predict.Item

	name := cond.SetOrDefaultFunc(os.Getenv("USER"), func() string {
		return os.Getenv("USERNAME")
	})

	if curr, err := user.Current(); err == nil && curr.Username != "" {
		name = curr.Username
	}

	if name != "" {
		items = append(items, predict.Item{
			Value:       name,
			Display:     name,
			Description: "Current user",
			Style:       style.Blue,
			Tag:         "users",
		})
	}

	return items
}

// parseSignal parses a signal by name (e.g. "SIGTERM", "TERM", "hup") or numeric string ("15").
func parseSignal(raw string) (syscall.Signal, error) {
	sig, ok := signals.Lookup(raw)
	if !ok {
		return 0, fmt.Errorf("%w %q", signals.ErrInvalid, raw)
	}

	return sig, nil
}
