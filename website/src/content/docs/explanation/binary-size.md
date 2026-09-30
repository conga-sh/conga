---
title: About binary size and linking
description: Why unused subsystems cost nothing to link, what each opt-in battery adds, and how its measured sizes compare with cobra and urfave/cli.
---

Every CONGA feature is a link-time opt-in: the Go linker includes a subsystem only when the application enables it. An app that enables no completion, registers no config decoder, and loads no `.env` files carries none of those subsystems in its binary. This page explains how that works and what each opt-in costs; the full measured tables live in the [binary size benchmark](/reference/binary-size-benchmark/).

## The mechanism: the linker can only drop what it can prove unreachable

Go's linker eliminates dead code at function granularity, but only for symbols it can prove are never reached from an always-reachable path. Two wiring styles decide which code counts as reachable:

- A subsystem referenced only from an uncalled builder method is provably dead, allowing the linker to drop the whole symbol graph.
- A subsystem guarded by a runtime boolean sits behind a branch where the compiler assumes both arms are reachable, retaining the code even when the flag is `false`.

CONGA deliberately uses the first style. Completion, configuration, `.env` loading, and value provenance are reached only through nil function-hook fields installed by opt-in builder calls:

- `app.EnableCompletion()` installs the completion hooks — six shell drivers, the `__complete` handler, and predictor-driven value completion.
- `app.EnableWhy()` installs the `--why` value-provenance report renderer.
- `app.RegisterDecoder(...)` / `app.ConfigOptions(...)` install the config resolver.
- `app.Dotenv(...)` installs the `.env` loader and reload hooks.

An application that calls none of these leaves every hook nil; the linker proves each call site unreachable and eliminates the subsystem. A checked-in DCE guard (`task test:size`) asserts the payloads are absent, so the property cannot silently regress. See [How to enable shell completion](/how-to/enable-shell-completion/), [How to load .env files](/how-to/load-env-files/), and [How to manage config files](/how-to/manage-config-files/) for the opt-in surface.

Cobra and urfave/cli manage the same subsystems through runtime switches. `DisableDefaultCmd` and `EnableShellCompletion: false` control whether a completion command is registered; the completion code stays linked either way, which is why each framework's "off" build is byte-identical to its "on" build in the measured ladder.

## The metadata cost of retained functions

Every function the linker retains carries runtime metadata on top of its executable text — PC-to-line tables in `.gopclntab` and type descriptors in `.go.type` — whether or not the function ever runs. Function-granularity dead-code elimination is therefore the dominant size lever, not instruction-count trimming.

The completion rung makes this concrete. Enabling completion adds about 232 KB of `.text` across roughly 150 functions; the file grows by 409,600 bytes because the same functions pull ~151 KB of `.gopclntab` and a further ~25 KB of `.rodata` and other sections along with them. Two consequences follow: the least expensive function is one never linked, and a framework whose reachable surface is large carries that metadata cost for every linked function, whether or not it runs.

## The cost of each battery

Each opt-in is priced individually against the batteries-off floor, `CONGA core` (2,580,640 bytes): flags, native env binding, required validation, and the declarative schema:

| build | file bytes | what it enables |
|---|---:|---|
| CONGA core | 2,580,640 | env binding, required validation, declarative schema |
| `+` `EnableCompletion()` | 2,990,240 | six shell drivers, predictor-driven value completion |
| `+` `EnableWhy()` | 2,633,888 | `--why` value-provenance report |
| `+` `RegisterDecoder`/`Dotenv()` | 3,154,080 | config resolver, TOML decoder, `.env` loading |
| full batteries | 3,551,392 | all three opt-ins together |

Completion costs 409,600 bytes (+15.9%), config 573,440 (+22.2%), and `--why` 53,248 (+2.1%). All three together cost 970,752 (+37.6%), less than the 1,036,288 the isolates sum to, because `internal/spec` and `predict` are shared across them.

## How the sizes compare across frameworks

Each framework's builds differ in feature set and wiring style, so the comparison here is size-only; the [benchmark page](/reference/binary-size-benchmark/) records the exact builds and methodology. The measured ladder supports three separate observations rather than a single summary:

- At the core rung, binary sizes are close: `CONGA core` (2,580,640 bytes) is within 1.1% of cobra (2,551,968 bytes). Cobra's binary includes its completion code in both configurations, while `CONGA core` includes native env binding, required validation, and the declarative schema, linking no completion code until enabled.
- For completion alone, cobra's build is smaller: cobra's four-shell build is 438,272 bytes smaller than `CONGA core + EnableCompletion()` (six shells, predictor-driven values, and wire protocol). When completion is the sole requirement, cobra yields a smaller binary.
- For 12-factor parity, CONGA's full-battery build is smaller: adding environment variable and config file support pulls additional dependencies into general-purpose stacks. Viper introduces `net/http`, `crypto/tls`, and `encoding/json/v2`, expanding cobra from 2.6 MB to 6.1 MB, while altsrc expands urfave from 4.2 MB to 8.8 MB. The full-battery CONGA build lands at 3.6 MB (41.5% smaller than cobra + viper and 59.5% smaller than urfave + altsrc), providing six-shell completion, `.env` loading, TOML config, and `--why`.

The size delta for the 12-factor rung is standard-library growth, not framework code: cobra + viper adds ~1.44 MB of runtime and stdlib against ~257 KB of new third-party code. CONGA's batteries are deliberately narrower, which is why they cost a flat ~0.97 MB.

## Runtime toggles versus link-time opt-ins

The distinction the numbers illustrate is that "not shipped unless enabled" is a *build-time* property in CONGA and a *runtime* registration question in cobra and urfave/cli. In cobra and urfave/cli, disabling a feature changes what is registered rather than what is linked. This distinction lies behind the byte-identical "off" builds in the measured ladder.