# 31 — Exactly-one flag groups (`oneof`)

The `oneof:"group"` tag on optional flag containers declares an exactly-one
group: the caller must supply exactly one member, and supplying two is a usage
error. The usage line renders the group as `(--input-file=PATH | --input-url=URL)`.

```sh
go run ./_examples/31-oneof-groups --input-file data.txt
go run ./_examples/31-oneof-groups --input-url https://example.com
```

Missing both sources and supplying both sources both exit with status 2.
