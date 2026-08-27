package bufir

// node.go holds the identity every other type in the graph is keyed by, and the
// target allow-list every one of them can carry.
//
// Both are here rather than beside the types that use them because they are used
// by all of them: a NodeID keys messages, fields, enums, values and methods
// alike, and a target filter applies to each in turn.

// NodeID is a node's fully qualified proto name — "sensors.v1.Sensor.rate_hz".
//
// It is the key for everything keyed by node, including the ordinal ledger, and
// it is always derived from the descriptor. Never from a rendered name: those are
// outputs, and a message renamed for ROS must not orphan its own ordinals.
type NodeID string

// allows reports whether a target allow-list admits the given target. An empty
// list admits everything, which is the common case; an empty target admits
// everything too, which is what a caller filtering on skip alone passes.
func allows(list []string, target string) bool {
	if len(list) == 0 || target == "" {
		return true
	}
	for _, t := range list {
		if t == target {
			return true
		}
	}
	return false
}
