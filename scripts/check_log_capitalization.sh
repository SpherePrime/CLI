#!/bin/bash
# Fails when a Prime log message starts with a lowercase letter.
#
# vendordeps/ is excluded on purpose: it holds third-party code we do not own,
# and x/exp/slog's own doc examples log lowercase strings, which would make
# this check unsatisfiable without editing a dependency.
set -uo pipefail

matches=$(grep -rEn \
  'slog\.(Error|Info|Warn|Debug|Fatal|Print|Println|Printf)\(["\"][a-z]' \
  --include="*.go" \
  --exclude-dir=vendordeps \
  --exclude-dir=.git \
  --exclude-dir=node_modules \
  . 2>/dev/null)

if [ -n "$matches" ]; then
  echo "❌ Log messages must start with a capital letter. Found lowercase logs above:"
  echo "$matches"
  exit 1
fi