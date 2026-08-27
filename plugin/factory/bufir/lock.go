package bufir

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// lockVersion is the ledger's schema version. It exists so a future change to the
// file's shape can be detected and refused rather than misread; nothing yet
// migrates between versions.
const lockVersion = 1

// LockFileName is the ledger's conventional name, relative to the output
// directory.
const LockFileName = "buffers.lock"

// Lock is the ordinal ledger.
//
// The exported fields are slices rather than maps so that the marshalled file has
// one stable order and a diff shows what changed rather than a reshuffle. Lookups
// go through the unexported indexes, built on load.
type Lock struct {
	// Version pins the ledger's schema, so a future change of shape can be
	// refused rather than misread.
	Version int `yaml:"version"`
	// Messages holds each message's recorded field ordinals.
	Messages []MessageSlots `yaml:"messages,omitempty"`
	// Enums holds each enum's recorded value ordinals.
	Enums []EnumSlots `yaml:"enums,omitempty"`
	// Services holds each service's recorded method ordinals.
	Services []ServiceSlots `yaml:"services,omitempty"`

	// msgIdx indexes Messages for lookup: node to number to ordinal.
	msgIdx map[NodeID]map[int32]int32
	// enumIdx indexes Enums the same way.
	enumIdx map[NodeID]map[int32]int32
	// svcIdx indexes Services by method name, since methods have no numbers.
	svcIdx map[NodeID]map[string]int32
}

// MessageSlots is one message's recorded field ordinals.
type MessageSlots struct {
	// Node is the message's fully qualified proto name.
	Node NodeID `yaml:"message"`
	// Fields are its recorded slots, ordered by proto field number.
	Fields []SlotEntry `yaml:"fields"`
}

// EnumSlots is one enum's recorded value ordinals.
type EnumSlots struct {
	// Node is the enum's fully qualified proto name.
	Node NodeID `yaml:"enum"`
	// Values are its recorded slots, ordered by proto value.
	Values []SlotEntry `yaml:"values"`
}

// ServiceSlots is one service's recorded method ordinals.
type ServiceSlots struct {
	// Node is the service's fully qualified proto name.
	Node NodeID `yaml:"service"`
	// Methods are its recorded slots, in declaration order — which is also
	// ordinal order, and which a diff should show as written.
	Methods []MethodEntry `yaml:"methods"`
}

// SlotEntry is one recorded field or enum value.
//
// Name is carried but never read back: the key is Number. It is there so a human
// reading a diff sees which field moved without cross-referencing the .proto,
// which is most of what makes this file reviewable.
type SlotEntry struct {
	// Number is the proto field number or enum value. It is the key: a rename is
	// not a wire change, so keying by name would invent a breaking change out of
	// a compatible one.
	Number int32 `yaml:"number"`
	// Ordinal is the target slot assigned to it.
	Ordinal int32 `yaml:"ordinal"`
	// Name is carried for a human reading a diff and never read back, so that a
	// moved field is visible without cross-referencing the .proto.
	Name string `yaml:"name,omitempty"`
}

// MethodEntry is one recorded method. Keyed by name, since proto methods have no
// numbers.
type MethodEntry struct {
	// Name is the method's own name, which is the key — a proto method has no
	// number, so a rename here does read as a delete plus an add.
	Name string `yaml:"name"`
	// Ordinal is the method's position in the interface.
	Ordinal int32 `yaml:"ordinal"`
}

// NewLock returns an empty ledger — what a first run starts from.
func NewLock() *Lock {
	return &Lock{
		Version: lockVersion,
		msgIdx:  map[NodeID]map[int32]int32{},
		enumIdx: map[NodeID]map[int32]int32{},
		svcIdx:  map[NodeID]map[string]int32{},
	}
}

// LoadLock reads a ledger from disk.
//
// A missing file is not an error: it is a first run, and returns an empty ledger
// rather than nil so every caller has somewhere to record into. An unreadable or
// malformed one *is* an error — a corrupt ledger that silently became an empty
// one would reassign every ordinal in the repository from scratch, which is
// precisely the failure this file exists to prevent.
func LoadLock(path string) (*Lock, error) {
	if path == "" {
		return NewLock(), nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewLock(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseLock(data, path)
}

// ParseLock decodes a ledger. Decoding is strict, so an unknown key is an error
// rather than a silently dropped record.
func ParseLock(data []byte, source string) (*Lock, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	lock := NewLock()
	if err := dec.Decode(lock); err != nil {
		return nil, fmt.Errorf("parse %s: %w", source, err)
	}
	if lock.Version != lockVersion {
		return nil, fmt.Errorf("parse %s: ledger version %d is not supported by this build (expected %d)",
			source, lock.Version, lockVersion)
	}
	lock.reindex()
	return lock, nil
}

// reindex rebuilds the lookup maps from the marshalled slices.
func (l *Lock) reindex() {
	l.msgIdx = make(map[NodeID]map[int32]int32, len(l.Messages))
	for _, m := range l.Messages {
		slots := make(map[int32]int32, len(m.Fields))
		for _, f := range m.Fields {
			slots[f.Number] = f.Ordinal
		}
		l.msgIdx[m.Node] = slots
	}
	l.enumIdx = make(map[NodeID]map[int32]int32, len(l.Enums))
	for _, e := range l.Enums {
		slots := make(map[int32]int32, len(e.Values))
		for _, v := range e.Values {
			slots[v.Number] = v.Ordinal
		}
		l.enumIdx[e.Node] = slots
	}
	l.svcIdx = make(map[NodeID]map[string]int32, len(l.Services))
	for _, s := range l.Services {
		slots := make(map[string]int32, len(s.Methods))
		for _, m := range s.Methods {
			slots[m.Name] = m.Ordinal
		}
		l.svcIdx[s.Node] = slots
	}
}

// FieldSlots returns the recorded field ordinals for a message, or nil when the
// ledger has never seen it.
func (l *Lock) FieldSlots(node NodeID) map[int32]int32 { return l.msgIdx[node] }

// ValueSlots returns the recorded value ordinals for an enum.
func (l *Lock) ValueSlots(node NodeID) map[int32]int32 { return l.enumIdx[node] }

// MethodSlots returns the recorded method ordinals for a service.
func (l *Lock) MethodSlots(node NodeID) map[string]int32 { return l.svcIdx[node] }
