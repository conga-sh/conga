package system

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/scanx"
	"github.com/conga-sh/conga/internal/signals"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/style"
	"github.com/conga-sh/conga/predict"
)

//nolint:gochecknoglobals // static fallback locales list
var defaultLocales = []string{
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

func actionUsers(ctx predict.Context) predict.Predictor {
	items := scanUsers(ctx)

	return predict.Items(items...)
}

func parseGroupReader(r io.Reader) []predict.Item {
	seen := make(map[string]bool)

	var items []predict.Item

	_ = scanx.ScanLines(r, func(line string) bool { //nolint:errcheck // best-effort candidate scan
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
	})

	return items
}

func parsePasswdReader(r io.Reader) []predict.Item {
	seen := make(map[string]bool)

	var items []predict.Item

	_ = scanx.ScanLines(r, func(line string) bool { //nolint:errcheck // best-effort candidate scan
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
	})

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
		key, value, found := strings.Cut(entry, "=")
		if !found || key == "" || seen[key] {
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

func fallbackLocales() []string {
	return slices.Clone(defaultLocales)
}

func scanLocales() []predict.Item {
	locales := scanLocaleGen()
	if len(locales) == 0 {
		locales = scanI18nLocales()
	}

	if len(locales) == 0 {
		locales = fallbackLocales()
	}

	uniqueLocales := slicex.Filter(slicex.Unique(locales), func(loc string) bool {
		return loc != ""
	})

	return slicex.Map(uniqueLocales, func(loc string) predict.Item {
		return predict.Item{
			Value:   loc,
			Display: loc,
			Style:   style.Cyan,
			Tag:     "locales",
		}
	})
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
	pid := os.Getpid()
	exe := "conga"

	if selfExe, err := os.Executable(); err == nil {
		exe = filepath.Base(selfExe)
	}

	if runtime.GOOS == "linux" {
		if procs := scanLinuxProc(); len(procs) > 0 {
			if !slices.ContainsFunc(procs, func(p procEntry) bool { return p.Pid == pid }) {
				procs = append(procs, procEntry{Pid: pid, Exe: exe})
			}

			return procs
		}
	}

	return []procEntry{{Pid: pid, Exe: exe}}
}

func scanUsers(ctx predict.Context) []predict.Item {
	if runtime.GOOS != "windows" {
		if file, err := os.Open("/etc/passwd"); err == nil {
			items := parsePasswdReader(file)
			_ = file.Close() //nolint:errcheck // Read-only file close.

			return items
		}
	}

	var items []predict.Item

	name := cond.SetOrDefaultFunc(ctx.Getenv("USER"), func() string {
		return ctx.Getenv("USERNAME")
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
