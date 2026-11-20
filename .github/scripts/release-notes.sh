#!/usr/bin/env bash
# Prints the release notes for VERSION in Markdown: the commits since the previous version tag,
# grouped by the Area of their `Area - Summary` message (`Area #12 - Summary` links issue #12).
#   .github/scripts/release-notes.sh 0.8.0 [revision]
# Merge commits and `Release - x.y.z` commits are left out. The previous tag is the highest
# x.y.z tag below VERSION that is an ancestor of the revision (default HEAD).
set -euo pipefail

version="${1:?usage: $0 VERSION [revision]}"
revision="${2:-HEAD}"

previous=""
while read -r tag; do
  if [[ "$tag" != "$version" && "$(printf '%s\n%s\n' "$tag" "$version" | sort -V | tail -1)" == "$version" ]] \
    && git merge-base --is-ancestor "$tag" "$revision"; then
    previous="$tag"
    break
  fi
done < <(git tag --list | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -rV)

range="$revision"
[[ -n "$previous" ]] && range="$previous..$revision"

git log --no-merges --reverse --format='%s' "$range" | awk '
  /^Release - / { next }
  {
    area = "Other"; summary = $0
    if (match($0, / - /)) {
      area = substr($0, 1, RSTART - 1)
      summary = substr($0, RSTART + 3)
    }
    issue = ""
    if (match(area, / #[0-9]+$/)) {
      issue = substr(area, RSTART + 1)
      area = substr(area, 1, RSTART - 1)
    }
    if (!(area in lines)) order[++n] = area
    lines[area] = lines[area] "- " summary (issue != "" ? " (" issue ")" : "") "\n"
  }
  END {
    if (n == 0) print "No changes since the previous release."
    for (i = 1; i <= n; i++) {
      if (order[i] == "Other") { other = 1; continue }
      printf "### %s\n\n%s\n", order[i], lines[order[i]]
    }
    if (other) printf "### Other\n\n%s\n", lines["Other"]
  }
'

if [[ -n "$previous" ]]; then
  echo "**Changes since ${previous}**: https://github.com/Glideh/plys/compare/${previous}...${version}"
fi
