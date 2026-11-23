#!/usr/bin/env bash
# Bumps the version in VERSION and in every file that repeats it, following semver.
#   ./scripts/bump.sh [patch|minor|major]   default: minor
# Does not commit. Warns when README.md and the end-user compose files differ.
set -euo pipefail
cd "$(dirname "$0")/.."

part="${1:-minor}"
case "$part" in
  patch|minor|major) ;;
  *) echo "Usage: $0 [patch|minor|major]" >&2; exit 1 ;;
esac

current="$(tr -d '[:space:]' < VERSION)"
if [[ ! "$current" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "VERSION must contain x.y.z, got '$current'" >&2
  exit 1
fi
major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}" patch="${BASH_REMATCH[3]}"
case "$part" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
new="$major.$minor.$patch"

# The first match is the top-level version; web/package-lock.json repeats it on lines 3 and 9.
sed -i "0,/\"version\": \"[^\"]*\"/s//\"version\": \"$new\"/" web/package.json
sed -i "1,10s/\"version\": \"$current\"/\"version\": \"$new\"/" web/package-lock.json
sed -i "0,/^  version: .*/s//  version: $new/" docs/openapi.yaml

# Release examples in the docs; the escaped pattern matches the dots literally.
esc="${current//./\\.}"
sed -i "s/\(release such as \)$esc\(, or latest\)/\1$new\2/" .env.example
sed -i "s/\(e\.g\. \`\)$esc\(\` to stay\)/\1$new\2/" README.md

gradle=android-tv/app/build.gradle.kts
code="$(sed -n 's/^ *versionCode = \([0-9]*\)$/\1/p' "$gradle")"
sed -i "s/versionName = \"[^\"]*\"/versionName = \"$new\"/; s/versionCode = $code\$/versionCode = $((code + 1))/" "$gradle"

echo "$new" > VERSION

readme_block() { awk -v n="$1" '/^```yaml$/{c++; f=(c==n); next} /^```$/{f=0} f' README.md; }
for pair in "1:deploy/compose/compose.minimal.yaml" "2:deploy/compose/compose.prod.yaml"; do
  if [[ "$(readme_block "${pair%%:*}")" != "$(cat "${pair#*:}")" ]]; then
    echo "WARNING: README.md differs from ${pair#*:}. scripts/release.sh will refuse to publish until they match." >&2
  fi
done

echo "Version $current -> $new"
echo "Next: review with git diff, commit, then ./scripts/release.sh"
