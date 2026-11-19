// Package enumtarget covers the value-type tag checks shared with schema build
// (spec.ValidateValueTarget): enum on path types and path predictor tags, and
// enum, range or len on maps and non-string map keys. Its filterable types do
// not import predict, so the interfaces are matched by method signature.
package enumtarget

import (
	"context"
	"io/fs"
	"os"

	"github.com/conga-sh/conga"
)

// FileFilter implements predict.FileFilterable with a value receiver.
type FileFilter string

func (FileFilter) MatchFile(path string, entry os.DirEntry) bool { return true }

func (f *FileFilter) UnmarshalText(text []byte) error { *f = FileFilter(text); return nil }

// DirFilter implements predict.DirFilterable with a pointer receiver.
type DirFilter string

func (*DirFilter) MatchDir(path string, entry fs.DirEntry) bool { return true }

func (f *DirFilter) UnmarshalText(text []byte) error { *f = DirFilter(text); return nil }

// PathFilter implements predict.PathFilterable.
type PathFilter string

func (PathFilter) MatchPath(path string, entry os.DirEntry) bool { return true }

func (f *PathFilter) UnmarshalText(text []byte) error { *f = PathFilter(text); return nil }

type PathTypes struct {
	File           conga.OptionalFlag[conga.File]        `enum:"a"`                                 // want `struct PathTypes field File: schema error: invalid struct tag: tag "enum" is not supported on conga\.File; use tag "glob" to filter completion candidates, or a string field with tag "enum" to accept a fixed set of names`
	FileContent    conga.OptionalFlag[conga.FileContent] `enum:"a"`                                 // want `tag "enum" is not supported on conga\.FileContent; use a string field with tag "enum" to accept a fixed set of names`
	Files          conga.OptionalFlag[[]conga.File]      `enum:"a"`                                 // want `tag "enum" is not supported on conga\.File; use tag "glob"`
	FilePointer    conga.OptionalFlag[*conga.File]       `enum:"a"`                                 // want `tag "enum" is not supported on conga\.File; use tag "glob"`
	FilePointers   conga.OptionalFlag[[]*conga.File]     `enum:"a"`                                 // want `tag "enum" is not supported on conga\.File; use tag "glob"`
	FileNone       conga.OptionalFlag[conga.File]        `enum:"a" predictor:"none"`                // want `tag "enum" is not supported on conga\.File; use tag "glob"`
	Tool           conga.OptionalFlag[conga.File]        `enum:"a" executable:"true"`               // want `tag "enum" is not supported on conga\.File; use a string field`
	ToolFalse      conga.OptionalFlag[conga.File]        `enum:"a" executable:"false"`              // want `tag "enum" is not supported on conga\.File; use a string field`
	Link           conga.OptionalFlag[conga.File]        `enum:"a" symlink:"true"`                  // want `tag "enum" is not supported on conga\.File; use a string field`
	FileFilter     conga.OptionalFlag[FileFilter]        `enum:"a"`                                 // want `tag "enum" is not supported on enumtarget\.FileFilter; use a string field`
	DirFilter      conga.OptionalFlag[DirFilter]         `enum:"a"`                                 // want `tag "enum" is not supported on enumtarget\.DirFilter; use a string field`
	PathFilters    conga.OptionalFlag[[]PathFilter]      `enum:"a"`                                 // want `tag "enum" is not supported on enumtarget\.PathFilter; use a string field`
	ExecutableGlob conga.OptionalFlag[conga.File]        `enum:"a" executable:"true" glob:"*.yaml"` // want `tag "executable" cannot be combined with tag "glob"`
	PointerTagged  conga.OptionalFlag[*conga.File]       `enum:"a" existing:"true"`                 // want `tag "enum" is not supported on conga\.File; use tag "glob"`
	ExistingBogus  conga.OptionalFlag[conga.File]        `enum:"a" existing:"bogus"`                // want `tag "existing" expects "true" or "false", got "bogus"`
	Dir            conga.RequiredArg[conga.Dir]          `enum:"a"`                                 // want `tag "enum" is not supported on conga\.Dir; use a string field`
}

func (c *PathTypes) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[PathTypes]()

type PathPredictors struct {
	File       conga.OptionalFlag[string] `enum:"a" predictor:"file"`        // want `struct PathPredictors field File: schema error: invalid struct tag: tag "enum" cannot be combined with predictor:"file"; remove tag "enum" \(predictor:"file:<patterns>" filters completion candidates\), or remove the predictor tag to complete the enum values`
	Patterns   conga.OptionalFlag[string] `enum:"a" predictor:"file:*.yaml"` // want `cannot be combined with predictor:"file"; remove tag "enum" \(`
	Spaced     conga.OptionalFlag[string] `enum:"a" predictor:" file "`      // want `cannot be combined with predictor:"file"; remove tag "enum" \(`
	Dir        conga.OptionalFlag[string] `enum:"a" predictor:"dir"`         // want `tag "enum" cannot be combined with predictor:"dir"; remove tag "enum", or remove the predictor tag to complete the enum values`
	Exec       conga.OptionalFlag[string] `enum:"a" predictor:"exec"`        // want `cannot be combined with predictor:"exec"; remove tag "enum", or`
	Executable conga.OptionalFlag[string] `enum:"a" predictor:"executable"`  // want `cannot be combined with predictor:"exec"; remove tag "enum", or`
	Symlink    conga.OptionalFlag[string] `enum:"a" predictor:"symlink"`     // want `cannot be combined with predictor:"symlink"; remove tag "enum", or`
	Symlinks   conga.OptionalFlag[string] `enum:"a" predictor:"symlinks"`    // want `cannot be combined with predictor:"symlink"; remove tag "enum", or`
	Malformed  conga.OptionalFlag[string] `enum:"a" predictor:"file:"`       // want `invalid predictor syntax "file:": file filter pattern cannot be empty`
}

func (c *PathPredictors) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[PathPredictors]()

type Maps struct {
	Enum         conga.OptionalFlag[map[string]string] `enum:"a"`      // want `struct Maps field Enum: schema error: invalid struct tag: tag "enum" is not supported on map types\. Use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`
	Range        conga.OptionalFlag[map[string]int]    `range:"1..10"` // want `tag "range" is not supported on map types`
	Len          conga.OptionalFlag[map[string]string] `len:"1..3"`    // want `tag "len" is not supported on map types`
	IntKey       conga.OptionalFlag[map[int]string]    // want `struct Maps field IntKey: schema error: map keys must be of type string, got int`
	EnumFile     conga.OptionalFlag[map[string]string] `enum:"a" predictor:"file"` // want `tag "enum" is not supported on map types`
	RequiredEnum conga.RequiredFlag[map[string]string] `default:"a=b" enum:"a"`    // want `tag "enum" is not supported on map types`
}

func (c *Maps) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[Maps]()

// Key is a named int map key.
type Key int

type NamedIntKey struct {
	Values conga.OptionalFlag[map[Key]string] // want `map keys must be of type string, got enumtarget\.Key`
}

func (c *NamedIntKey) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[NamedIntKey]()
