package bufir

// build.go is the entry point: descriptors in, graph out. It owns the pass
// ordering, which is the part of the walk that is not obvious — each pass needs
// something the one before it produced.

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

// Options configures a build.
type Options struct {
	// Strict is the per-rule severity spec, spelled like protokit.Options.Strict.
	// See ParseStrict.
	Strict string

	// LockPath is where the ordinal ledger is read from. Empty starts from an
	// empty ledger, which is what a test or a first run wants.
	LockPath string
}

// builder carries the state one build accumulates.
type builder struct {
	// strict resolves a diagnostic's rule to a severity.
	strict Strictness
	// lock is the ledger this build read, consulted for every slot.
	lock *Lock
	// recorder accumulates what this build actually assigned.
	recorder *Recorder
	// schema is the graph being assembled.
	schema *Schema

	// files indexes every file seen, generated or imported, so a field can find
	// the file its referenced type lives in when computing imports.
	files map[string]*File

	// owner maps a type's full name to the proto path declaring it.
	owner map[NodeID]string

	// pending accumulates diagnostics; diags() sorts them into a stable order.
	pending []Diagnostic
}

// Build walks the plugin's descriptors into a Schema.
//
// Every file contributes its types to the indexes, including files that are only
// imported: a generated message may well have a field whose type is declared in a
// dependency, and a target that cannot resolve it cannot emit an include line for
// it. Only generate-flagged files become Schema.Files, though — emitting a schema
// for someone else's module would write their types into this module's output
// directory.
func Build(p *protogen.Plugin, opts Options) (*Schema, error) {
	strict, err := ParseStrict(opts.Strict)
	if err != nil {
		return nil, err
	}
	lock, err := LoadLock(opts.LockPath)
	if err != nil {
		return nil, err
	}

	b := &builder{
		strict:   strict,
		lock:     lock,
		recorder: NewRecorder(),
		schema: &Schema{
			Messages: map[NodeID]*Message{},
			Enums:    map[NodeID]*Enum{},
		},
		files: map[string]*File{},
		owner: map[NodeID]string{},
	}

	// Pass 1: every file's types, so that later passes can resolve a reference to
	// anything the compilation unit knows about.
	for _, f := range p.Files {
		file := b.file(f)
		file.Generate = f.Generate
		b.files[file.Path] = file
		for _, m := range f.Messages {
			b.message(m, file, nil)
		}
		for _, e := range f.Enums {
			b.enum(e, file, nil)
		}
	}

	// Pass 2: slots. Ordinals are assigned over the whole index rather than the
	// generated files alone, because an imported message's ordinals are what a
	// generated message's reference to it will be read through.
	b.assignSlots()

	// Pass 3: layout. It runs after fields exist because a message is only
	// packable if every message it holds is packable, which needs the graph.
	b.resolveLayouts()

	// Pass 4: services, and the imports every generated file turns out to need.
	for _, f := range p.Files {
		file := b.files[f.Desc.Path()]
		for _, s := range f.Services {
			file.Services = append(file.Services, b.service(s, file))
		}
	}
	for _, f := range p.Files {
		if !f.Generate {
			continue
		}
		file := b.files[f.Desc.Path()]
		b.resolveImports(file)
		b.schema.Files = append(b.schema.Files, file)
	}

	b.checkROSPackages()
	b.schema.Lock = b.recorder.Lock()
	b.schema.Diags = b.diags()

	if errs, _ := b.strict.Partition(b.schema.Diags); len(errs) > 0 {
		return b.schema, fmt.Errorf("%d schema problem(s):\n  - %s", len(errs), joinDiags(errs))
	}
	return b.schema, nil
}

// Strictness returns the severity policy a build was run under, so a caller can
// print the warnings it chose not to fail on.
func (s *Schema) Strictness(spec string) (Strictness, error) { return ParseStrict(spec) }
