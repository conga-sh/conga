package main

import (
	"context"
	"net/url"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// TypesCmd groups the exhaustive flag-type matrix.
type TypesCmd struct {
	Primitives conga.Command[PrimitivesCmd] `summary:"Every primitive integer, unsigned, float, and slice width"`
	Filesystem conga.Command[FilesystemCmd] `summary:"Filesystem domain types"`
	Values     conga.Command[ValueCmd]      `summary:"Duration, URL, byte size, and enum value types"`
}

// PrimitivesCmd demonstrates every primitive width, bound from config
// sections. The foreign kong tag is permitted schema-wide by app.AllowTags.
type PrimitivesCmd struct {
	StringFlag  conga.OptionalFlag[string] `default:"default" kong:"default='default'" safe:"true"`
	BoolFlag    conga.OptionalFlag[bool]
	IntFlag     conga.OptionalFlag[int]      `default:"42"    safe:"true"`
	Int8Flag    conga.OptionalFlag[int8]     `default:"8"     safe:"true"`
	Int16Flag   conga.OptionalFlag[int16]    `default:"16"    safe:"true"`
	Int32Flag   conga.OptionalFlag[int32]    `default:"32"    safe:"true"`
	Int64Flag   conga.OptionalFlag[int64]    `default:"64"    safe:"true"`
	UintFlag    conga.OptionalFlag[uint]     `default:"100"   safe:"true"`
	Uint8Flag   conga.OptionalFlag[uint8]    `default:"8"     safe:"true"`
	Uint16Flag  conga.OptionalFlag[uint16]   `default:"16"    safe:"true"`
	Uint32Flag  conga.OptionalFlag[uint32]   `default:"32"    safe:"true"`
	Uint64Flag  conga.OptionalFlag[uint64]   `default:"64"    safe:"true"`
	Float32Flag conga.OptionalFlag[float32]  `default:"3.14"  safe:"true"`
	Float64Flag conga.OptionalFlag[float64]  `default:"2.718" safe:"true"`
	Tags        conga.OptionalFlag[[]string] `safe:"true"`
	Ports       conga.OptionalFlag[[]int]    `safe:"true"`
}

func (c *PrimitivesCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	return nil
}

// FilesystemCmd demonstrates the filesystem domain types.
type FilesystemCmd struct {
	InputFile conga.OptionalFlag[conga.File] `existing:"true"   safe:"true"    short:"i"`
	OutputDir conga.OptionalFlag[conga.Dir]  `existing:"true"   safe:"true"    short:"o"`
	LogFile   conga.OptionalFlag[conga.File] `safe:"true"`
	DataDir   conga.OptionalFlag[conga.Dir]  `safe:"true"`
	Editor    conga.OptionalFlag[conga.File] `executable:"true" safe:"true"    short:"e"`
	Hook      conga.OptionalFlag[conga.File] `safe:"true"       symlink:"true"`
}

func (c *FilesystemCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	return nil
}

// ValueCmd demonstrates the remaining built-in value types.
type ValueCmd struct {
	Timeout  conga.OptionalFlag[time.Duration]     `default:"30s"                         safe:"true"`
	Endpoint conga.OptionalFlag[url.URL]           `default:"https://example.com"         help:"API endpoint"          safe:"true"`
	MaxSize  conga.OptionalFlag[conga.ByteSize]    `default:"10MB"                        safe:"true"`
	Level    conga.OptionalFlag[string]            `default:"info"                        enum:"debug,info,warn,error" safe:"true"`
	ShardMap conga.OptionalFlag[map[string]string] `help:"Shard assignments (native map)" safe:"true"`
}

func (c *ValueCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	return nil
}
