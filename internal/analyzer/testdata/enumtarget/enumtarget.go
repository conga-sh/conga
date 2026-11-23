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
	File           conga.OptionalFlag[conga.File]        `enum:"a"`                                 // want `struct PathTypes field File: tag "enum" is not supported on conga\.File`
	FileContent    conga.OptionalFlag[conga.FileContent] `enum:"a"`                                 // want `tag "enum" is not supported on conga\.FileContent`
	Files          conga.OptionalFlag[[]conga.File]      `enum:"a"`                                 // want `tag "enum" is not supported on conga\.File`
	FilePointer    conga.OptionalFlag[*conga.File]       `enum:"a"`                                 // want `tag "enum" is not supported on conga\.File`
	FilePointers   conga.OptionalFlag[[]*conga.File]     `enum:"a"`                                 // want `tag "enum" is not supported on conga\.File`
	FileNone       conga.OptionalFlag[conga.File]        `enum:"a" predictor:"none"`                // want `tag "enum" is not supported on conga\.File`
	Tool           conga.OptionalFlag[conga.File]        `enum:"a" executable:"true"`               // want `tag "enum" is not supported on conga\.File`
	ToolFalse      conga.OptionalFlag[conga.File]        `enum:"a" executable:"false"`              // want `tag "enum" is not supported on conga\.File`
	Link           conga.OptionalFlag[conga.File]        `enum:"a" symlink:"true"`                  // want `tag "enum" is not supported on conga\.File`
	FileFilter     conga.OptionalFlag[FileFilter]        `enum:"a"`                                 // want `tag "enum" is not supported on enumtarget\.FileFilter`
	DirFilter      conga.OptionalFlag[DirFilter]         `enum:"a"`                                 // want `tag "enum" is not supported on enumtarget\.DirFilter`
	PathFilters    conga.OptionalFlag[[]PathFilter]      `enum:"a"`                                 // want `tag "enum" is not supported on enumtarget\.PathFilter`
	ExecutableGlob conga.OptionalFlag[conga.File]        `enum:"a" executable:"true" glob:"*.yaml"` // want `tag "executable" cannot be combined with tag "glob"`
	PointerTagged  conga.OptionalFlag[*conga.File]       `enum:"a" existing:"true"`                 // want `tag "enum" is not supported on conga\.File`
	ExistingBogus  conga.OptionalFlag[conga.File]        `enum:"a" existing:"bogus"`                // want `tag "existing" expects "true" or "false", got "bogus"`
	Dir            conga.RequiredArg[conga.Dir]          `enum:"a"`                                 // want `tag "enum" is not supported on conga\.Dir`
}

func (c *PathTypes) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[PathTypes]()

type PathPredictors struct {
	File       conga.OptionalFlag[string] `enum:"a" predictor:"file"`        // want `struct PathPredictors field File: tag "enum" cannot be combined with predictor:"file"`
	Patterns   conga.OptionalFlag[string] `enum:"a" predictor:"file:*.yaml"` // want `cannot be combined with predictor:"file"`
	Spaced     conga.OptionalFlag[string] `enum:"a" predictor:" file "`      // want `cannot be combined with predictor:"file"`
	Dir        conga.OptionalFlag[string] `enum:"a" predictor:"dir"`         // want `tag "enum" cannot be combined with predictor:"dir"`
	Exec       conga.OptionalFlag[string] `enum:"a" predictor:"exec"`        // want `cannot be combined with predictor:"exec"`
	Executable conga.OptionalFlag[string] `enum:"a" predictor:"executable"`  // want `cannot be combined with predictor:"exec"`
	Symlink    conga.OptionalFlag[string] `enum:"a" predictor:"symlink"`     // want `cannot be combined with predictor:"symlink"`
	Symlinks   conga.OptionalFlag[string] `enum:"a" predictor:"symlinks"`    // want `cannot be combined with predictor:"symlink"`
	Malformed  conga.OptionalFlag[string] `enum:"a" predictor:"file:"`       // want `invalid predictor syntax "file:": file filter pattern cannot be empty`
}

func (c *PathPredictors) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[PathPredictors]()

type Maps struct {
	Enum         conga.OptionalFlag[map[string]string] `enum:"a"`      // want `struct Maps field Enum: tag "enum" is not supported on map types`
	Range        conga.OptionalFlag[map[string]int]    `range:"1..10"` // want `tag "range" is not supported on map types`
	Len          conga.OptionalFlag[map[string]string] `len:"1..3"`    // want `tag "len" is not supported on map types`
	IntKey       conga.OptionalFlag[map[int]string]    // want `struct Maps field IntKey: map keys must be of type string, got int`
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
