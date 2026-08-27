// buffers: one AIP-native schema, every serialization surface.
//
// The dependency set is small on purpose, and worth reading as a statement of
// what this plugin is allowed to know:
//
//	protokit       the neutral IR engine. This plugin uses its service IR (for
//	               RPC interfaces), its naming/header/templates/manifest helpers,
//	               and its factory Source/Target/Registry — but NOT its schema IR.
//	               See plugin/factory/bufir/doc.go for why a serialization plugin
//	               needs a message graph rather than a database tree.
//	genproto/api   google.api.resource / field_behavior / resource_reference —
//	               the AIP vocabulary this plugin reads directly.
//	cobra          the `buffers` CLI. The protoc plugin does not import it; only
//	               plugin/cmd/buffers does.
//	yaml.v3        buffers.yaml and buffers.lock.
//
// Notably absent: flatc, capnp and the Wire compiler. This plugin *emits schemas
// those toolchains consume* and shells out to them from the CLI; it does not link
// against, vendor, or reimplement any of them.
module github.com/the-protobuf-project/buffers

go 1.26.4

require (
	github.com/bufbuild/protocompile v0.14.1
	github.com/spf13/cobra v1.10.2
	github.com/the-protobuf-project/protokit v1.3.1
	google.golang.org/genproto/googleapis/api v0.0.0-20260825221802-da73d73af1c5
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sync v0.21.0 // indirect
)
