---
title: About shell completion
draft: true
description: What happens when the shell asks a CONGA program for completions, how CONGA picks one predictor for each field, when predictors run, and why candidates leave validation to the run.
---

A CONGA program computes its own completions. `EnableCompletion` adds a hidden `__complete` command to the program ([Completion surface](/reference/builtins/#completion-surface)), and the script that `completion <shell>` prints contains none of the program's commands or flags. When the user asks for completions, the script runs `<cmd> __complete <shell>` with the words on the line, and passes the candidates the program prints to the shell.

A script could instead carry the command tree, written into it when it is generated, but it would then be correct only for the binary that generated it. The candidates come from the same binary that would run the command, so they change when the program does, with no new script. They can come from what the program reads at run time, such as a project file loaded in `Bootstrap` or an API a callback queries, which a script generated in advance can't hold. The six shells share one implementation of the parsing and the candidates, and differ in how they display them. The cost is that each request starts the program and builds its schema, so the program's startup time is time the user waits at the prompt.

## Completion follows the real run

The program reads the partial line against the schema a real run uses, so the candidates fit the command the line selects. The shell offers that command's subcommands and flags, with the flags its ancestors declare, and drops the other members of an `xor` or `oneof` group once the line names one.

Completion also runs the part of startup that loads data. CONGA loads `.env` files, binds the `bootstrap:"true"` fields from the command line, the environment, and their defaults, and calls `Bootstrap`, so a document the program loads there can supply candidates. When the program declares `conga.Features`, CONGA also reads the config files and the environment to decide which features are on, and leaves out the flags and commands gated behind features that are off. No other field is bound. [How to compute and shape completion candidates](/how-to/compute-completion-candidates/) completes a command's services from its project file this way. Then CONGA prints the candidates and exits before any command's `Run` method runs.

File and directory paths are candidates like any other, computed in the program and the same in every shell. The shell's own file-completion settings, such as zsh's `file-patterns` style, have no effect on them, as [File and directory candidates](/reference/builtins/#file-and-directory-candidates) describes.

The shell reads the program's standard output as the completion result, so errors wait for the real run. A failed `Bootstrap` or lookup yields an empty list, and the user reads the error when they run the command. A callback that printed would corrupt the result.

## One predictor per field

A predictor is the value that produces a field's candidates. CONGA gives each flag and positional argument one predictor at most, and that predictor alone decides what the shell offers for it. With one predictor, the answer to "where did these candidates come from" is a single place, even when several declarations could supply candidates. A field that needs candidates from several sources gets one predictor that combines them, built with `predict.Batch` and named in a `predictor` tag, so the merge shows in the program's code.

CONGA looks first at declarations made for completion, and then at tags made for something else. A `predictor` tag names the predictor for one field, so it comes first, and `predictor:"none"` turns completion off for that field. A predictor registered for the field's type with `app.RegisterType` comes next, which lets a program replace the completion of a type it doesn't own. The type's own methods follow (`Predictable`, `TagAwarePredictable`, and the path filters), written by the type's author for every field of that type. When none of these applies, CONGA derives candidates from the field's other tags and its type. These are the `enum` values, then `true` and `false` when the user types `=` after a bool flag. A field with none of these offers nothing. For a slice, the element type decides, and CONGA leaves out values the line already holds. [Which predictor a field uses](/reference/predict/#which-predictor-a-field-uses) lists the steps.

This order puts an `enum` behind the field's type, so a type with a long list keeps the whole list on a field whose `enum` narrows it. `system.Signal` completes every signal, and a `stop` command that accepts three of them still offers them all:

```go
type StopCmd struct {
	Signal conga.OptionalFlag[system.Signal] `default:"TERM" enum:"TERM,INT,HUP" help:"Signal to send"`
}
```

The shell offers 90 candidates for `--signal`, starting with `SIGHUP`, `HUP`, and `1`, and the run accepts three of them. The `enum` compares the text the user typed, so `SIGINT` and `2`, two spellings of `INT` that the shell offers, fail too:

```sh error={2}
$ cloudctl stop --signal SIGINT
cloudctl: error: invalid value "SIGINT" (must be one of: TERM, INT, HUP) for flag --signal
```

## When predictors run

Predictors run at two times, and the time decides what a predictor can see.

A type's `Predict` method runs at schema build. CONGA builds the schema at the start of every run, so with completion enabled, `Predict` runs each time anyone runs the program, ordinary commands included. It runs before CONGA parses anything, so it sees no flag values, and slow work in it delays every invocation, each completion request included. With completion off, CONGA never calls it.

A `predict.Callback` runs when the shell asks, after CONGA has parsed the partial line. Its `predict.Context` holds the words and flag values typed so far, so one value's candidates can follow another's. The images callback in [Add shell completion to a CLI](/tutorials/complete-a-cli/) offers the images of the service already on the line. The context holds the command line only, and values from the environment, a config file, or a `default` tag stay out of it. The user waits at the prompt while a callback runs, so CONGA gives it 250 milliseconds (`predict.DefaultTimeout`), or the deadline the predictor sets with `Timeout`, and then offers nothing. A type whose candidates come from live data returns a `Callback` from `Predict`, which moves the work from every run to the moment the shell asks, as the tutorial's `Service` type does.

## Candidates and validation

A predictor offers values, and the user can type any other. The run accepts what the user typed unless the type's parser, a tag such as `enum` or `range`, or a `Validator` checks it. Of these, only `enum` also completes, so a field with an `enum` tag and no other predictor offers exactly the values it accepts.

Keeping the two apart suits candidates from live sources. A callback may read a cache that is a day old, fetch a list that has changed since, or run out of time and offer nothing. A run that rejected values missing from those candidates would fail for reasons the user can't see. The command checks its input against the source it trusts, at run time, as `deploy` in [How to compute and shape completion candidates](/how-to/compute-completion-candidates/) checks tags against its project file in `Run`.

## How map flags complete

A map flag takes `key=value` pairs, so its completion has two parts, the key and then its value. A `predict.KeyValue` named in a `predictor` tag describes both, as a map from each key to its values. The shell offers the keys with their `=` (`team=` and `tier=`), then the values of the key on the line (`web` and `db` after `tier=`). [Complete a value in parts](/how-to/compute-completion-candidates/#complete-a-value-in-parts) builds one, and `predict.KeyValueDescribed` adds a description to each key.

The program supplies the keys because the map's type names only what kind they are. A `map[string]string` of labels accepts any key the user invents, so a map flag with no predictor shows the usage hint `<key>=<value>`.

`conga.Features` declares its keys. They are the fields of the struct it holds, so CONGA builds the key-value predictor from them, and feature names complete from the declaration alone.

## Where to go next

- [Add shell completion to a CLI](/tutorials/complete-a-cli/): turn completion on for `cloudctl` and complete its values from a tag, a type, and a callback.
- [How to get completion from tags and types](/how-to/complete-from-tags-and-types/): the values CONGA completes from `enum` tags, paths, and domain types.
- [How to compute and shape completion candidates](/how-to/compute-completion-candidates/): callbacks, combinators, values in parts, and deadlines.
- [Package predict](/reference/predict/): every constructor, combinator, and interface, and the order CONGA picks a field's predictor in.
