// Package bufir is the message-graph IR every buffers target renders from:
// descriptors in, a target-agnostic graph of files, messages, fields, enums and
// services out, with each field pinned to a stable target slot.
//
// # Why not protokit's schema IR
//
// protokit ships two frontends and this package is neither, which deserves an
// explanation since reusing one of them was the first thing tried.
//
// The schema IR folds messages into databases, schemas, tables and columns. That
// fold is exactly right for a generator that stores things and exactly wrong
// here, in three ways that are not fixable by reading it more carefully. It keeps
// only the messages that are resources or reachable from one, so a plain value
// object — a Vector3 — has no table and therefore no representation, while a
// .fbs that omits it does not compile. It collapses the four 64-bit widths into
// one neutral type, which a database is right to do and a serialization schema is
// not: Cap'n Proto has a distinct Int64 and UInt64 and picking the wrong one
// silently reinterprets every negative value. And it has nowhere to put a oneof,
// because a table has no union.
//
// The service IR is closer — it carries proto-granular Kinds, oneofs, and the AIP
// resource index, and this package's Kind is deliberately spelled the same way so
// the two read alike. But service.Build only materializes messages reachable from
// a service method, and a .proto file of pure messages with no service at all is
// the single most common input a serialization plugin gets. A schema that
// disappears when you delete the service is not a schema.
//
// So the graph is built here. What protokit still supplies is everything above
// the IR: naming, the reproducible banner, the template helper, the manifest, the
// golden harness, and the factory's Source/Target/Registry.
//
// Method classification is the one place this package duplicates something
// protokit does better, and the reason is worth recording. service.Build
// classifies AIP-131..136 methods and then *checks the shape the name implies* —
// a GetBook carrying a body is rejected — which is more than the prefix match in
// build.go's classifyMethod. But it reaches that classification by building the
// whole route table, and a route table with two overlapping google.api.http
// templates fails the build. That is exactly right for a gateway and an unrelated
// reason for a .capnp file to fail to generate. The common robotics input here is
// a .proto with no HTTP annotations at all, and it has to produce a schema. The
// classification is used only to shape a ROS .srv and to order an interface's
// methods, so the cheaper answer is the proportionate one.
//
// # What makes it a schema rather than a dump
//
// The one property this package exists to protect is that a field's slot in the
// emitted schema never moves. A proto field number is not that slot: proto
// numbers are 1-based and sparse, Cap'n Proto ordinals are 0-based and
// contiguous, and FlatBuffers ids are 0-based, contiguous, and consumed two at a
// time by a union. Every target therefore needs a mapping, and a mapping that is
// recomputed from scratch on each run is a mapping that silently changes when
// someone deletes a field.
//
// ordinal.go derives that mapping, lock.go records it, and the two disagreeing is
// a diagnostic rather than a coin flip. See ordinal.go for the derivation rules
// and why a deleted field must be `reserved`.
package bufir
