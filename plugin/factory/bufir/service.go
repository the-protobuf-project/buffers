package bufir

// service.go holds the call surface: services, their methods, and the AIP-123
// resource a message may declare.
//
// Resource sits here rather than in message.go, though a message is what carries
// it, because what reads it is the RPC side: it is how a ROS .srv is shaped like
// the standard method it implements, and how the Wire target derives its
// tree-shaking roots.
type Service struct {
	// Node is the service's fully qualified proto name.
	Node NodeID
	// Name is the service's own name, unqualified.
	Name string
	// Package is the proto package declaring it.
	Package string
	// File is the file declaring it.
	File *File
	// Doc is the leading comment, as prose.
	Doc string

	// CapnpInterface reports whether a Cap'n Proto interface is emitted.
	CapnpInterface bool

	// CapnpID is the interface's Cap'n Proto ID, declared or derived.
	CapnpID uint64

	// ROSService reports whether .srv files are emitted for the call methods.
	ROSService bool

	// Targets restricts the service to the named targets; empty means all.
	Targets []string
	// Skip excludes the service from every target.
	Skip bool

	// Methods are its RPCs, in declaration order, which is also ordinal order.
	Methods []*Method
}

// Method is one RPC.
type Method struct {
	// Node is the method's fully qualified proto name.
	Node NodeID
	// Name is the method's own name, unqualified.
	Name string
	// Doc is the leading comment, as prose.
	Doc string
	// Parent is the service declaring it.
	Parent *Service

	// Input is the request message.
	Input *Message
	// Output is the response message.
	Output *Message

	// ClientStream reports whether the client streams its request. No target here
	// has an equivalent, so the build reports it rather than guessing.
	ClientStream bool
	// ServerStream reports whether the server streams its response, which is what
	// makes a method a publication rather than a call.
	ServerStream bool

	// Transport is resolved — never TransportUnspecified after a successful
	// build.
	Transport Transport

	// Topic is the ROS topic / eCAL channel name for a TransportTopic method.
	Topic string

	// ROSName is the .srv/.action base name.
	ROSName string

	// Ordinal is the Cap'n Proto method ordinal: 0-based within the interface.
	Ordinal int32
	// OrdinalSource records where Ordinal came from.
	OrdinalSource OrdinalSource

	// Pattern is the AIP method classification — "Get", "List", "Create",
	// "Update", "Delete", "BatchGet", … or "Custom". It is what lets a ROS .srv
	// be shaped like the standard method it implements.
	Pattern string

	// Targets restricts the method to the named targets; empty means all.
	Targets []string
	// Skip excludes the method, which still consumes its ordinal.
	Skip bool
}

// Resource is a message's AIP-123 resource descriptor.
type Resource struct {
	// Type is the AIP-123 resource type, e.g. "sensors.example.com/Sensor".
	Type string // "sensors.example.com/Sensor"
	// Patterns are the resource name templates it may be addressed by.
	Patterns []string // "robots/{robot}/sensors/{sensor}"
	// Singular is the resource's singular form.
	Singular string
	// Plural is the resource's plural form.
	Plural string

	// NameField is the field carrying the resource name, which AIP-122 says is
	// `name` and AIP-203 marks IDENTIFIER.
	NameField string
}

// allows reports whether a target allow-list admits the given target. An empty
// list admits everything, which is the common case; an empty target admits
