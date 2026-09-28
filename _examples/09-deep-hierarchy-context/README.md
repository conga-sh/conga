# 09 — Deep hierarchy context

Deep command nesting (`cluster node pod list`) with hierarchy navigation:
`session.Root[T]()`, root-down `Command[T].Get()` traversal, `session.Path()`,
`session.Name()`, and per-command `envprefix` tags.

## Run it

```sh
go run ./_examples/09-deep-hierarchy-context cluster node pod list -s pending
CLOUDCTL_GLOBAL_REGION=eu-west-1 go run ./_examples/09-deep-hierarchy-context
```

## What to look for

- The leaf reads ancestor flags through typed `Get()` traversal.
- `envprefix:"CLOUDCTL_"` binds root flags; `envprefix:"POD_"` overrides it for
  the pod subtree.
- `session.Path()` and `session.Name()` describe the active invocation.

## Test

```sh
go test ./_examples/09-deep-hierarchy-context
```

Related: [08 — Subcommands basic](../08-subcommands-basic/),
[13 — Twelve-factor precedence](../13-twelve-factor-precedence/).
