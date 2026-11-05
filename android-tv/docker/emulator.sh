#!/usr/bin/env bash
set -euo pipefail

if [[ ! -r /dev/kvm || ! -w /dev/kvm ]]; then
    printf 'Android TV emulation requires Linux with access to /dev/kvm.\n' >&2
    exit 1
fi
if [[ ! -f "$ANDROID_USER_HOME/avd/$ANDROID_AVD.ini" ]]; then
    avdmanager create avd --name "$ANDROID_AVD" --package "$ANDROID_SYSTEM_IMAGE" --device tv_1080p </dev/null
fi
adb start-server
exec emulator -avd "$ANDROID_AVD" -port 5554 -no-window -no-audio -no-boot-anim \
    -no-snapshot -no-metrics -gpu swiftshader_indirect -accel on "$@"