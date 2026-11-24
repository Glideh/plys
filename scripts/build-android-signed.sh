#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ $# -ne 0 ]]; then
  echo "Usage: $0" >&2
  exit 1
fi

keystore="${PLYS_KEYSTORE_FILE:-android-tv/release.keystore}"
if [[ ! -f "$keystore" || ! -r "$keystore" ]]; then
  echo "Keystore not found or not readable: $keystore" >&2
  exit 1
fi
keystore="$(realpath "$keystore")"
if [[ "$keystore" == *:* ]]; then
  echo "Keystore path cannot contain a colon: $keystore" >&2
  exit 1
fi

prompt_if_missing() {
  local name="$1" value="$2" prompt="$3" secret="${4:-false}"
  if [[ -z "$value" ]]; then
    if [[ ! -t 0 ]]; then
      echo "$name must be set when running non-interactively." >&2
      exit 1
    fi
    if [[ "$secret" == true ]]; then
      read -r -s -p "$prompt: " value
      echo
    else
      read -r -p "$prompt: " value
    fi
  fi
  if [[ -z "$value" ]]; then
    echo "$name cannot be empty." >&2
    exit 1
  fi
  REPLY="$value"
}

prompt_if_missing PLYS_KEY_ALIAS "${PLYS_KEY_ALIAS:-}" "Key alias"
export PLYS_KEY_ALIAS="$REPLY"
prompt_if_missing PLYS_KEYSTORE_PASSWORD "${PLYS_KEYSTORE_PASSWORD:-}" "Keystore password" true
export PLYS_KEYSTORE_PASSWORD="$REPLY"
prompt_if_missing PLYS_KEY_PASSWORD "${PLYS_KEY_PASSWORD:-}" "Key password" true
export PLYS_KEY_PASSWORD="$REPLY"

export PLYS_KEYSTORE=/run/plys-release.keystore
docker compose --profile android run --rm \
  -v "$keystore:$PLYS_KEYSTORE:ro" \
  -e PLYS_KEYSTORE -e PLYS_KEYSTORE_PASSWORD -e PLYS_KEY_ALIAS -e PLYS_KEY_PASSWORD \
  android-build :app:assembleRelease

echo "Signed APK: android-tv/app/build/outputs/apk/release/app-release.apk"
