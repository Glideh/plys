#!/usr/bin/env bash
# Checks the repository can be released and prints the version: VERSION holds x.y.z and the README
# embeds the two deploy Compose files verbatim (its first and second YAML blocks).
set -euo pipefail
cd "$(dirname "$0")/../.."

version="$(tr -d '[:space:]' < VERSION)"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "VERSION must contain x.y.z, got '$version'" >&2
  exit 1
fi

readme_block() { awk -v n="$1" '/^```yaml$/{c++; f=(c==n); next} /^```$/{f=0} f' README.md; }
for pair in "1:deploy/compose/compose.minimal.yaml" "2:deploy/compose/compose.prod.yaml"; do
  if [[ "$(readme_block "${pair%%:*}")" != "$(cat "${pair#*:}")" ]]; then
    echo "The compose file in README.md differs from ${pair#*:}: update the README." >&2
    exit 1
  fi
done

echo "$version"
