#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" == 0 ]]; then
    target_uid="${LOCAL_UID:-1000}"
    target_gid="${LOCAL_GID:-1000}"
    if [[ ! "$target_uid" =~ ^[0-9]+$ || ! "$target_gid" =~ ^[0-9]+$ || "$target_uid" == 0 || "$target_gid" == 0 ]]; then
        printf 'LOCAL_UID and LOCAL_GID must be non-root numeric IDs.\n' >&2
        exit 1
    fi
    mkdir -p "$GRADLE_USER_HOME" "$ANDROID_USER_HOME"
    for directory in "$GRADLE_USER_HOME" "$ANDROID_USER_HOME" /workspace/.gradle /workspace/.kotlin /workspace/app/build; do
        if [[ -d "$directory" && "$(stat -c '%u:%g' "$directory")" != "$target_uid:$target_gid" ]]; then
            chown -R "$target_uid:$target_gid" "$directory"
        fi
    done
    groups=(--clear-groups)
    if [[ -e /dev/kvm ]]; then
        groups=(--groups "$(stat -c '%g' /dev/kvm)")
    fi
    exec setpriv --reuid "$target_uid" --regid "$target_gid" "${groups[@]}" "$0" "$@"
fi

case "${1:-}" in
    emulator)
        shift
        exec /usr/local/bin/android-emulator "$@"
        ;;
    adb)
        exec "$@"
        ;;
    *)
        exec gradle "$@"
        ;;
esac