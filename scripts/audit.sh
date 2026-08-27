#!/usr/bin/env bash
# Reports the two house rules this repository holds itself to:
#
#   1. every Go file is under 200 lines, comments included
#   2. every declaration carries a doc comment
#
# Both are checked here rather than by a linter because neither is a rule
# golangci-lint enforces: its `lll` is per-line, and `revive`'s exported-comment
# rule ignores unexported declarations, which are most of this codebase.
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

limit="${1:-200}"
status=0

echo "==> files over $limit lines"
over="$(find plugin -name '*.go' -not -path '*/pb/*' -exec wc -l {} \; \
  | awk -v n="$limit" '$1 > n {printf "  %5d  %s\n", $1, $2}' | sort -rn)"
if [ -n "$over" ]; then
  echo "$over"
  status=1
else
  echo "  none"
fi

echo
echo "==> declarations without a doc comment"
undocumented="$(go run ./scripts/docaudit 2>/dev/null)"
if [ -n "$undocumented" ]; then
  echo "$undocumented"
  status=1
else
  echo "  none"
fi

exit "$status"
