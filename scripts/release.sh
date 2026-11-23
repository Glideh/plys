#!/usr/bin/env bash
# Builds the production images and publishes them to Docker Hub.
# Releases are normally published by the Release GitHub workflow (.github/workflows/release.yml), on master,
# once a develop commit with a new VERSION has passed the Tests and been promoted; this script is the manual fallback.
#   ./scripts/release.sh           build + push glide/plys-api and glide/plys-web at the version in VERSION (set it with scripts/bump.sh)
#   ./scripts/release.sh --local   build :dev into the local Docker only
# Requires `docker login` first. NAMESPACE overrides the Docker Hub account.
set -euo pipefail
cd "$(dirname "$0")/.."

export NAMESPACE="${NAMESPACE:-glide}"
arg="${1:-}"

if [[ "$arg" == "--local" ]]; then
  export VERSION=dev
  docker buildx bake -f docker/bake.hcl --load
  echo "Built ${NAMESPACE}/plys-api:dev and ${NAMESPACE}/plys-web:dev"
  exit 0
fi

if [[ -n "$arg" ]]; then
  echo "Usage: $0 [--local]. The version comes from VERSION; change it with scripts/bump.sh." >&2
  exit 1
fi
export VERSION="$(tr -d '[:space:]' < VERSION)"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "VERSION must contain x.y.z, got '$VERSION'" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "The working tree has uncommitted changes: commit them first so the image matches a revision." >&2
  exit 1
fi

# The README embeds the two end-user files from deploy/compose as its first two YAML blocks.
readme_block() { awk -v n="$1" '/^```yaml$/{c++; f=(c==n); next} /^```$/{f=0} f' README.md; }
for pair in "1:deploy/compose/compose.minimal.yaml" "2:deploy/compose/compose.prod.yaml"; do
  if [[ "$(readme_block "${pair%%:*}")" != "$(cat "${pair#*:}")" ]]; then
    echo "The compose file in README.md differs from ${pair#*:}: update the README." >&2
    exit 1
  fi
done
export REVISION="$(git rev-parse HEAD)"

echo "About to publish ${NAMESPACE}/plys-api and ${NAMESPACE}/plys-web ${VERSION} (also tagged latest) from ${REVISION:0:8}."
read -r -p "Continue? [y/N] " answer
[[ "$answer" == "y" || "$answer" == "Y" ]] || exit 1

docker buildx bake -f docker/bake.hcl --push
echo "Published. Tag the revision too: git tag ${VERSION} && git push origin ${VERSION}"
