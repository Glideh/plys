#!/usr/bin/env bash
# Builds the production images and publishes them to Docker Hub.
#   ./release.sh 0.1.0     build + push glide/plys-api and glide/plys-web (tags 0.1.0, 0.1, latest; plus 1 from 1.0.0 on)
#   ./release.sh --local   build :dev into the local Docker only
# Requires `docker login` first. NAMESPACE overrides the Docker Hub account.
set -euo pipefail
cd "$(dirname "$0")"

export NAMESPACE="${NAMESPACE:-glide}"
arg="${1:-}"

if [[ "$arg" == "--local" ]]; then
  export VERSION=dev
  docker buildx bake -f docker-bake.hcl --load
  echo "Built ${NAMESPACE}/plys-api:dev and ${NAMESPACE}/plys-web:dev"
  exit 0
fi

if [[ ! "$arg" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Usage: $0 <x.y.z> | --local" >&2
  exit 1
fi
export VERSION="$arg"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "The working tree has uncommitted changes: commit them first so the image matches a revision." >&2
  exit 1
fi
export REVISION="$(git rev-parse HEAD)"

echo "About to publish ${NAMESPACE}/plys-api and ${NAMESPACE}/plys-web ${VERSION} (also tagged latest) from ${REVISION:0:8}."
read -r -p "Continue? [y/N] " answer
[[ "$answer" == "y" || "$answer" == "Y" ]] || exit 1

docker buildx bake -f docker-bake.hcl --push
echo "Published. Tag the revision too: git tag v${VERSION} && git push origin v${VERSION}"
