package bufir

// vocab.go mirrors the enums buffers.v1 declares, one Go type per proto enum.
//
// They are re-declared rather than used from the generated stubs so that nothing
// downstream of the build imports bufferspbv1. A target renders a Layout; it has
// no business knowing which proto option the value arrived in, and options.go is
// the only file that translates between the two.

// Layout mirrors buffers.v1.Layout: whether a record is evolvable or packed.
type Layout uint8

const (
	// LayoutUnspecified means the build has not decided yet. It never survives
	// the build; inferLayout resolves it to one of the two below.
	LayoutUnspecified Layout = iota
	// LayoutTable is the evolvable, vtable-backed record. Always legal.
	LayoutTable
	// LayoutStruct is the packed, fixed-size record: no vtable, no defaults, no
	// evolution. Eligibility is decided by layout.go, which reports a message
	// that asks for it and does not qualify rather than downgrading it.
	LayoutStruct
)

// String renders the layout for a diagnostic.
func (l Layout) String() string {
	switch l {
	case LayoutTable:
		return "table"
	case LayoutStruct:
		return "struct"
	}
	return "unspecified"
}

// Behavior is one google.api.field_behavior value (AIP-203).
type Behavior uint8

const (
	// BehaviorUnspecified is the zero value, meaning the field declared none.
	BehaviorUnspecified Behavior = iota
	// BehaviorOptional marks a field the caller may omit.
	BehaviorOptional
	// BehaviorRequired marks a field the caller must supply. It is the behavior
	// most targets can only document rather than enforce.
	BehaviorRequired
	// BehaviorOutputOnly marks a field the server sets and the client may not.
	BehaviorOutputOnly
	// BehaviorInputOnly marks a field the client sets and the server never returns.
	BehaviorInputOnly
	// BehaviorImmutable marks a field settable at creation and not thereafter.
	BehaviorImmutable
	// BehaviorUnorderedList marks a repeated field whose order carries no meaning.
	BehaviorUnorderedList
	// BehaviorNonEmptyDefault marks a field whose default is not the zero value.
	BehaviorNonEmptyDefault
	// BehaviorIdentifier marks the field carrying a resource's name (AIP-122).
	// It is what Resource.NameField is resolved from.
	BehaviorIdentifier
)

// String renders the behavior as AIP spells it.
func (b Behavior) String() string {
	switch b {
	case BehaviorOptional:
		return "OPTIONAL"
	case BehaviorRequired:
		return "REQUIRED"
	case BehaviorOutputOnly:
		return "OUTPUT_ONLY"
	case BehaviorInputOnly:
		return "INPUT_ONLY"
	case BehaviorImmutable:
		return "IMMUTABLE"
	case BehaviorUnorderedList:
		return "UNORDERED_LIST"
	case BehaviorNonEmptyDefault:
		return "NON_EMPTY_DEFAULT"
	case BehaviorIdentifier:
		return "IDENTIFIER"
	}
	return "BEHAVIOR_UNSPECIFIED"
}

// Transport mirrors buffers.v1.Transport: how a method's messages move.
type Transport uint8

const (
	// TransportUnspecified lets the build classify from the method's own shape.
	TransportUnspecified Transport = iota
	// TransportCall is a request and response pair: a ROS service, a Cap'n Proto
	// method returning results.
	TransportCall
	// TransportTopic is a one-way publication, carrying the method's output.
	TransportTopic
	// TransportAction is a goal, feedback and result triple — a ROS action —
	// derivable only from an AIP-151 long-running method.
	TransportAction
)

// String renders the transport for a diagnostic.
func (t Transport) String() string {
	switch t {
	case TransportCall:
		return "call"
	case TransportTopic:
		return "topic"
	case TransportAction:
		return "action"
	}
	return "unspecified"
}
