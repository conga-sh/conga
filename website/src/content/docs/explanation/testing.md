---
title: About testing CONGA programs
description: How to test a CONGA program end to end, by running the built binary or by running the app inside the test, what each can see, and where unit tests of a command's functions fit.
---

An end-to-end test of a CLI starts where a user does, with a command line, and checks what the user gets back, namely output, error messages, and an exit status. Go programs usually test end to end by building the binary and running it as a separate process. CONGA also lets a test run the whole app inside the test process, with no binary. Each sees things the other misses, and both sit beside unit tests of the functions a command calls.

## Running the built binary

A test can build the program and run it as a separate process, with `os/exec` or a script runner such as [`testscript`](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript). The process gets a real command line, real environment variables, a real working directory, and real standard streams, and it ends with a real exit status. The test sees exactly what a user sees.

CONGA's own example programs are tested this way. Each one has scripts under `_examples/NN-*/testdata/` with lines such as `exit-status 2 subcommands-basic`, which runs the program and checks that it exits with status 2.

The cost is speed, and the distance between a failure and its cause. Each case starts a process, the scripts sit outside Go, and a failure points at output rather than at a line of code. In return, these tests see the exit status the process really returns, and everything the program writes to its standard output and standard error, whichever code writes it.

## Running the app inside the test process

A test can instead build the same `App` that `main` builds, give it a command line with `Args` and streams with `Stdin`, `Stdout`, and `Stderr`, and call `RunContext`. CONGA parses the command line, reads the environment and config file, and runs the commands, all inside the test process. The test gets back the error `RunContext` returns, and whatever the commands wrote to the streams. There is no binary to build, and a failure is an ordinary Go test failure.

### Why it depends on the session's streams

The streams a test sets belong to that `App`, and reach a command only through `session.Stdout()`, `session.Stderr()`, and `session.Stdin()`. In a normal run they are the process's own streams, so a command that writes through the session behaves the same for a user as one that calls `fmt.Println`. In a test they are whatever the test passed, so the same code writes into a buffer.

The arrangement breaks as soon as a command, or a package it calls, writes to `os.Stdout` or reads `os.Stdin` directly, as `fmt.Println` does. The command still works at the terminal, and the test never sees that output or supplies that input. A package that accepts an `io.Reader` or `io.Writer` can be handed the session's streams, while one that only uses the process's streams leaves those lines to the process-level tests. CONGA's own output (help, version, and the error messages `Exit` prints) already goes to the app's streams.

### What it can't see

An in-process run returns an error, not an exit status. The status a user would see comes from `app.Exit`, which maps the error to 0, 1, or 2 and calls `os.Exit`. A test can call `Exit` too, after giving `ExitFunc` a function that records the status in place of exiting, to check the status and the message on standard error. [How to test a command](/how-to/test-a-command/) shows it. A command that calls `os.Exit` itself ends the test binary.

The run also shares the test's process. The environment and the working directory belong to the whole process, so a test isolates itself with `t.Setenv` and `t.Chdir`, and such tests can't run in parallel. Tests that set neither can call `t.Parallel()`, because the command line and streams belong to each `App`.

An `App` runs once. A second `Run` or `RunContext` on the same `App` panics with `conga: App already executed`, so each test builds its own, usually through a function that `main` shares, as `newApp` does in [Test a CLI](/tutorials/test-a-cli/).

## Where unit tests fit

Unit tests call ordinary Go functions with ordinary values, as in any Go program. They don't involve CONGA at all, they are the fastest tests a program has, and they are the right place for the logic a command runs, such as building a deployment plan, formatting a report, or deciding whether a value is valid.

A command struct itself is a poor target for this. Its fields are containers such as `conga.OptionalFlag[int]`, and only CONGA fills them, while it binds the command line, environment, and config files. The containers have no exported setters. A test can't build a `DeployCmd` with `--replicas 3` and call its `Run` method. Keeping `Run` short, so that it reads the containers and hands plain values to functions that do the work, leaves most of a program testable this way.

What unit tests can't see is the command line, which is what the end-to-end tests are for. That covers flag names, defaults, required flags, the config file, the environment, and the messages a user gets when they get something wrong.

## Choosing between them

Most programs need more than one. Put the logic in functions and test it there, where tests are cheapest. Use in-process runs for the command line itself, checking that a flag exists and has the right default, that the config file and environment reach the right flags, that a prompt reads its answer, and that a mistake is a usage error. Keep a few process-level tests for what only a real process shows, which is the status the binary returns, including from a command that calls `os.Exit` itself, and output that doesn't go through the session.

How much an in-process run checks depends on the commands. If every command reads and writes through the session, it checks nearly everything a user sees, at the speed of a unit test. If some commands print directly, those lines are visible only to the process-level tests.
