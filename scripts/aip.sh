#!/usr/bin/env bash
# Run the Google API linter over every proto this repository owns.
#
# There is no configuration file and no disabled rule. buffers generates
# serialization schemas *from* AIP-shaped protos, and a vocabulary that does not
# itself pass the linter would be telling its users to meet a bar it ducks. The
# examples are held to the same standard for the same reason: they are what a
# reader copies.
#
# api-linter reads a FileDescriptorSet rather than resolving imports itself, so
# buf builds one per module first.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

status=0

lint_module() {
  local module="$1"
  # Two statements, not one: bash expands the whole assignment list before
  # binding any of it, so a `local a=$1 b=$a` reads an unset $a under `set -u`.
  local set="$tmp/${module//\//_}.binpb"
  buf build "$root/$module" -o "$set" --as-file-descriptor-set

  # The files to lint are this module's own, relative to its module root — not
  # its dependencies', which are someone else's to fix.
  local files=()
  while IFS= read -r f; do files+=("$f"); done < <(
    cd "$root/$module" && find . -name '*.proto' | sed 's|^\./||' | sort
  )

  echo "==> $module (${#files[@]} files)"
  if ! api-linter --descriptor-set-in="$set" --set-exit-status "${files[@]}"; then
    status=1
  fi
}

lint_module protobuf
lint_module examples/proto

if [ "$status" -eq 0 ]; then
  echo "==> clean: no AIP findings"
fi
exit "$status"
