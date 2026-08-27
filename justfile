# buffers dev tasks — run `just` (or `just --list`) to see recipes.
#
# Common flows:
#   just dev        # build+install both binaries, test, regen examples with them
#   just ci         # what CI verifies: lint, build, test (mutates nothing)
#   just regen      # rewrite every committed artifact: stubs, goldens, examples
#
# Requires: go, buf, protoc-gen-go (for `stubs`). The `langs` recipe additionally
# needs flatc and capnp on PATH; it is not part of `ci`, because a schema this
# repository emits is verified by golden comparison, and whether a third-party
# compiler accepts it is a separate claim tested by `just langs`.

# Dev binaries are built into ./bin and that dir is prepended to PATH for buf, so
# a brew/global protoc-gen-buffers never shadows the build under test.
bin := justfile_directory() / "bin"

# -buildvcs=false stamps the generated-file version banner as "dev" (matching the
# committed goldens/examples) instead of the git tag + "+dirty" working-tree
# version, so regeneration doesn't churn every banner line.
_flags := "-buildvcs=false"

# List recipes (default when you run bare `just`).
_default:
    @just --list

# Build both binaries into ./bin (version banner: "dev").
build:
    mkdir -p {{bin}}
    go build {{_flags}} -o {{bin}}/protoc-gen-buffers ./plugin/cmd/protoc-gen-buffers
    go build {{_flags}} -o {{bin}}/buffers ./plugin/cmd/buffers

# Install both binaries onto your Go bin (GOBIN) for use in other projects.
install:
    go install {{_flags}} ./plugin/cmd/protoc-gen-buffers
    go install {{_flags}} ./plugin/cmd/buffers

# Show every protoc-gen-buffers on PATH, in resolution order, with versions.
which:
    @for p in $(which -a protoc-gen-buffers 2>/dev/null); do printf '%s\t' "$p"; "$p" --version; done || echo "none on PATH (run: just install)"

# Regenerate the buffers.v1 option Go stubs into plugin/pb/bufferspbv1.
stubs:
    buf generate

# Format Go sources in place.
fmt:
    gofmt -w plugin

# Static checks: gofmt, vet, golangci-lint, buf lint, api-linter (mutates nothing).
#
# This is what .github/workflows/lint.yaml runs, in the same order, so a green
# `just lint` means a green Lint workflow.
[doc("House rules: every file under 200 lines, every declaration documented.")]
audit:
    ./scripts/audit.sh

[doc("Static checks: gofmt, vet, golangci-lint, buf lint, api-linter, house rules.")]
lint: aip audit
    @test -z "$(gofmt -l plugin)" || { echo "unformatted files (run: just fmt):"; gofmt -l plugin; exit 1; }
    go vet ./...
    golangci-lint run ./...
    buf lint

# Run the Google API linter over every proto this repository owns.
#
# No config file, no disabled rule. buffers generates schemas *from* AIP-shaped
# protos; a vocabulary that did not itself pass would be holding its users to a
# bar it ducks. Requires api-linter:
#   go install github.com/googleapis/api-linter/cmd/api-linter@latest
[doc("Run the Google API linter over every proto here (zero findings, no suppressions).")]
aip:
    ./scripts/aip.sh

# Run unit + golden tests.
[doc("Run unit, golden and toolchain tests.")]
test:
    go test ./...

# Rewrite the golden fixtures to whatever the current build produces. Review the
# diff — a change here is a change to every consumer's wire format.
[doc("Rewrite the golden fixtures to what the current build produces.")]
golden:
    go test ./... -update

# Render the example protos into examples/generated with the build under test.
[doc("Render the example protos with the build under test.")]
examples: build
    PATH="{{bin}}:$PATH" buf generate --template buf.gen.example.yaml

# Compile the emitted schemas with the real toolchains. Not part of `ci`: it
# needs flatc and capnp installed, and it tests *their* acceptance of the output
# rather than this repository's determinism.
[doc("Compile the emitted schema with the real flatc and capnp.")]
langs: examples
    PATH="{{bin}}:$PATH" buffers generate --config examples/buffers.yaml

# Everything CI runs, minus the platform build matrix.
#
# The reproducibility gates are included because they are the two most common
# ways to push a red build: regenerating the examples or the descriptor set
# without committing the result.
[doc("Everything CI checks, minus the platform build matrix.")]
ci: lint build test
    @buf build examples/proto -o examples/descriptors.binpb --as-file-descriptor-set
    @git diff --exit-code -- examples/descriptors.binpb \
        || { echo "examples/descriptors.binpb is stale — commit the rebuild"; exit 1; }
    @go run ./plugin/cmd/buffers generate --config examples/buffers.yaml >/dev/null
    @git diff --exit-code -- examples/generated/ \
        || { echo "examples/generated is stale — run 'just examples' and commit"; exit 1; }
    @go run ./plugin/cmd/buffers verify --config examples/buffers.yaml

# The full local loop.
dev: install test examples

# Rewrite every committed artifact.
regen: stubs golden examples
