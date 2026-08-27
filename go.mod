// buffers: one AIP-native schema, every serialization surface.
//
// The dependency set is small on purpose, and worth reading as a statement of
// what this plugin is allowed to know:
//
//	protokit       the neutral IR engine, and the only direct dependency that
//	               shapes anything. This plugin renders from its buffers IR
//	               (protokit/buffers: the message graph, the ordinal derivation
//	               and the ledger), and uses its naming and header helpers and its
//	               factory Source/Target/Registry. Not its schema IR, and not its
//	               service IR — see that package's doc.go for why a serialization
//	               schema needs a message graph rather than a database tree or a
//	               route table.
//
//	               The buffers.v1 vocabulary reaches that IR through
//	               plugin/factory/vocab rather than through an import: protokit
//	               imports no annotation module, and google.api.* is now read on
//	               its side of the seam, which is why genproto is indirect here.
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
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sync v0.21.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260825221802-da73d73af1c5 // indirect
)
