// Package config reads buffers.yaml, the CLI's configuration.
//
// The protoc plugin has no configuration file — everything it needs arrives as a
// `opt:` in buf.gen.yaml, because that is where a buf user expects to configure a
// plugin and a second file would be a second place to look. The CLI does have
// one, because it does strictly more: it compiles protos itself, renders several
// targets in one pass, and then drives external toolchains, and expressing that
// as command-line flags would mean a shell line nobody can review.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Version is the only config schema version this build understands.
const Version = "v1"

// FileName is the conventional config file name.
const FileName = "buffers.yaml"

// Config is a whole buffers.yaml.
type Config struct {
	// Version pins the schema. Required, so that a later incompatible change can
	// be refused rather than half-read.
	Version string `yaml:"version"`

	// Proto describes where the .proto sources are.
	Proto Proto `yaml:"proto"`

	// Out is the root directory every target writes beneath, relative to the
	// config file.
	//
	// Empty is legal and means the config file's own directory, so a bare
	// `buffers.yaml` beside the protos writes <target>/ next to itself. It is not
	// required for the same reason protoc does not require -o: the useful default
	// is "here", and a config that names a source tree has already said where
	// "here" is.
	Out string `yaml:"out"`

	// Lock is the ordinal ledger's path, relative to the config file. Empty
	// disables it, which gives up slot stability across runs; see
	// plugin/factory/bufir/lock.go for what that costs.
	Lock string `yaml:"lock"`

	// Strict is the per-rule severity spec, spelled as in the plugin's
	// `strict=` option.
	Strict string `yaml:"strict"`

	// GoModule is the Go module generated Go code will live in, for any target
	// that emits some. Entry.GoModule overrides it per target.
	//
	// Both Go-emitting paths need it and neither can derive it. capnpc-go's
	// $Go.import must name where the generated file lands; flatc writes
	// cross-package imports as bare namespace paths that resolve against no module
	// without --go-module-name. In both cases the generator knows the package tree
	// and not the module root it hangs from.
	GoModule string `yaml:"go_module"`

	// Generate is the list of target renderings to perform.
	Generate []Entry `yaml:"generate"`
}

// Proto describes the input tree.
type Proto struct {
	// Paths are directories containing the .proto files to generate from. Each is
	// also an import root, so a proto importing "sensors/v1/x.proto" resolves
	// against them.
	Paths []string `yaml:"paths"`

	// Imports are additional import roots that are compiled but not generated
	// from — the equivalent of protoc's -I for a dependency tree.
	Imports []string `yaml:"imports"`

	// DescriptorSet is a prebuilt FileDescriptorSet, as produced by
	//
	//	buf build -o set.binpb --as-file-descriptor-set
	//
	// When set, paths and imports are not read. This is the option to use whenever
	// the protos depend on anything resolved by buf — googleapis, a BSR module —
	// because buf has already done the resolution and nothing here has to
	// reimplement it.
	DescriptorSet string `yaml:"descriptor_set"`

	// Generate narrows which files are generate-flagged, by proto path. Empty
	// means every file under paths, or every non-dependency file in a descriptor
	// set.
	Generate []string `yaml:"generate"`
}

// Entry is one target rendering.
type Entry struct {
	// Target names the backend: flatbuffers, capnp, ros, wire.
	Target string `yaml:"target"`

	// Out is this target's directory, relative to Config.Out. Defaults to the
	// target name, which keeps two targets from writing the same path.
	Out string `yaml:"out"`

	// Languages are the languages to compile the emitted schema into, by running
	// the target's toolchain. Empty emits schema and stops, which is what a run
	// that only wants the IDL asks for.
	Languages []string `yaml:"languages"`

	// LangOut is where compiled language output goes, relative to Config.Out.
	// Defaults to "<target>-<language>".
	LangOut string `yaml:"lang_out"`

	// GoModule overrides Config.GoModule for this entry.
	//
	// It usually has to be set when more than one target emits Go, because each
	// target's output lands in its own directory and therefore has its own import
	// path. One module root cannot describe both.
	GoModule string `yaml:"go_module"`
}

// Load reads and validates a config file.
//
// The returned paths are all resolved relative to the config file's directory, so
// a caller never has to know where it was invoked from — running `buffers
// generate -c sub/buffers.yaml` from a repository root behaves the same as
// running it from `sub/`.
func Load(path string) (*Config, error) {
	if path == "" {
		path = FileName
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(path); err != nil {
		return nil, err
	}
	cfg.resolve(filepath.Dir(path))
	return &cfg, nil
}
