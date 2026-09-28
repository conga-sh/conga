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
	StringFlag  conga.OptionalFlag[string]   `default:"default"    help:"String value"  kong:"default='default'" safe:"true"`
	BoolFlag    conga.OptionalFlag[bool]     `default:"false"      help:"Boolean value" safe:"true"`
	IntFlag     conga.OptionalFlag[int]      `default:"42"         help:"Integer value" safe:"true"`
	Int8Flag    conga.OptionalFlag[int8]     `default:"8"          help:"Int8 value"    safe:"true"`
	Int16Flag   conga.OptionalFlag[int16]    `default:"16"         help:"Int16 value"   safe:"true"`
	Int32Flag   conga.OptionalFlag[int32]    `default:"32"         help:"Int32 value"   safe:"true"`
	Int64Flag   conga.OptionalFlag[int64]    `default:"64"         help:"Int64 value"   safe:"true"`
	UintFlag    conga.OptionalFlag[uint]     `default:"100"        help:"Unsigned int"  safe:"true"`
	Uint8Flag   conga.OptionalFlag[uint8]    `default:"8"          help:"Uint8 value"   safe:"true"`
	Uint16Flag  conga.OptionalFlag[uint16]   `default:"16"         help:"Uint16 value"  safe:"true"`
	Uint32Flag  conga.OptionalFlag[uint32]   `default:"32"         help:"Uint32 value"  safe:"true"`
	Uint64Flag  conga.OptionalFlag[uint64]   `default:"64"         help:"Uint64 value"  safe:"true"`
	Float32Flag conga.OptionalFlag[float32]  `default:"3.14"       help:"Float32 value" safe:"true"`
	Float64Flag conga.OptionalFlag[float64]  `default:"2.718"      help:"Float64 value" safe:"true"`
	Tags        conga.OptionalFlag[[]string] `help:"String slice"  safe:"true"          sep:","`
	Ports       conga.OptionalFlag[[]int]    `help:"Integer slice" safe:"true"          sep:","`
}

func (c *PrimitivesCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	return nil
}

// FilesystemCmd demonstrates the filesystem domain types.
type FilesystemCmd struct {
	InputFile conga.OptionalFlag[conga.File] `existing:"true"                          help:"Path to existing input file"            safe:"true"    short:"i"`
	OutputDir conga.OptionalFlag[conga.Dir]  `existing:"true"                          help:"Path to existing directory"             safe:"true"    short:"o"`
	LogFile   conga.OptionalFlag[conga.File] `help:"Log file path (created on demand)" safe:"true"`
	DataDir   conga.OptionalFlag[conga.Dir]  `help:"Data directory path"               safe:"true"`
	Editor    conga.OptionalFlag[conga.File] `executable:"true"                        help:"Editor executable (resolved via $PATH)" safe:"true"    short:"e"`
	Hook      conga.OptionalFlag[conga.File] `help:"Hook script (symlink completion)"  safe:"true"                                   symlink:"true"`
}

func (c *FilesystemCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	return nil
}

// ValueCmd demonstrates the remaining built-in value types.
type ValueCmd struct {
	Timeout  conga.OptionalFlag[time.Duration]     `default:"30s"                         help:"Timeout duration"      safe:"true"`
	Endpoint conga.OptionalFlag[url.URL]           `default:"https://example.com"         help:"API endpoint"          safe:"true"`
	MaxSize  conga.OptionalFlag[conga.ByteSize]    `default:"10MB"                        help:"Maximum file size"     safe:"true"`
	Level    conga.OptionalFlag[string]            `default:"info"                        enum:"debug,info,warn,error" help:"Log level" safe:"true"`
	ShardMap conga.OptionalFlag[map[string]string] `help:"Shard assignments (native map)" safe:"true"`
}

func (c *ValueCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	return nil
}
