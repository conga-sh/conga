# 05 — Domain types

`conga.File` capability tags (`existing`, `stdin`, `stdout`, `symlink`, `glob`),
`conga.Dir`, `conga.ByteSize`, `conga.FileContent`, and custom domain types
implementing `encoding.TextUnmarshaler` + `conga.SelfValidator` (`PortRange`,
`HeaderMap`). Domain types earn filesystem completion for free.

## Run it

```sh
go run ./_examples/05-domain-types --help
printf 'key=value\n' > valid.txt
go run ./_examples/05-domain-types -i valid.txt -m 2GiB -p 8000-8080 -H Accept=text/html
```

The complete scenarios, fixtures included, live in
[`testdata/domain.txt`](testdata/domain.txt).

## What to look for

- `existing`/`symlink`/file-vs-directory validation errors at exit code 2.
- `File` + `stdin:"true"` streams lazily via `Reader()`; `FileContent` reads
  eagerly at decode time.
- `glob:"*.yaml,*.yml"` filters completion candidates by name pattern.

## Test

```sh
go test ./_examples/05-domain-types
```

Related: [19 — Shell completion](../19-shell-completion/),
[99 — Kitchen sink](../99-kitchen-sink/).
