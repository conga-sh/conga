---
title: How to lint CONGA schemas
description: Catch leaf Runner violations, struct tag typos, and ancestor flag shadowing in CI with conga-lint.
---

`conga-lint` catches schema errors, tag typos, and invalid hierarchies before binaries are built or run. This guide covers installation, running the analyzer over your packages, and enabling builder-order validation. [conga-lint](/reference/conga-lint/) catalogs every check and flag.

## Install and run the analyzer

Declare the analyzer as a tool dependency of your Go module, then invoke it over your packages:

```sh
go get -tool github.com/conga-sh/conga/cmd/conga-lint
go tool conga-lint ./...
```

Go's `./...` pattern skips underscore-prefixed directories such as `_examples/`; pass those paths explicitly to lint them.

## Validate builder call ordering

The optional `-builder-order` flag checks builder method chains against CONGA's canonical ordering conventions:

```sh
# Report builder ordering violations
go tool conga-lint -builder-order ./...

# Automatically fix violations in place
go tool conga-lint -builder-order -fix ./...

# Preview suggested fixes as a diff
go tool conga-lint -builder-order -fix -diff ./...
```

Custom ordering rules can be supplied via `-builder-order-spec`, which accepts comma- or newline-separated method names or `@path` to read one per line; [conga-lint](/reference/conga-lint/#flags) documents every flag.

## Where to go next

- [conga-lint](/reference/conga-lint/) — checks, discovery contract, and flags.
- [Struct tag reference](/reference/struct-tags/) — reference catalog for all supported struct tags and syntax rules.
- [Errors and exit codes](/reference/errors/) — base error categories and exit code mappings.
- [How to declare subcommands](/how-to/declare-subcommands/) — command hierarchy routing and flag shadowing rules.
