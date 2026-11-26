# Plys TV

Android TV / Google TV client built with Kotlin, Compose for TV, Paging 3 and
Media3. Requires Android 8.0 (API 26) or newer and a running [Plys server](../README.md).
The API contract is documented in [OpenAPI](../docs/openapi.yaml).

## Build

Only Docker with Compose is required. From the repository root:

```sh
#docker compose --profile android build android-build
docker compose --profile android run --rm android-build :app:assembleDebug
```

Output: `android-tv/app/build/outputs/apk/debug/app-debug.apk`.

The image includes JDK 17, Gradle 8.11.1 and SDK 35. Build files use the host
UID/GID from `.env`. Gradle caches and the debug signing key persist in Docker
volumes; keep them to preserve signed updates and installed sessions. The build
image uses Linux/amd64; ARM hosts need Docker's amd64 emulation. This does not
restrict the architectures supported by the APK.

For native builds, use the same tool versions, SDK build-tools 35.0.0 and set
`ANDROID_HOME` or `sdk.dir` in `local.properties`. No Gradle wrapper is included.

### Signed APK

From the repository root, place the signing keystore at
`android-tv/release.keystore` and run:

```sh
./scripts/build-android-signed.sh
```

The command prompts for the key alias and both passwords without echoing the
passwords. For non-interactive use, set `PLYS_KEY_ALIAS`,
`PLYS_KEYSTORE_PASSWORD` and `PLYS_KEY_PASSWORD` in the environment. Set
`PLYS_KEYSTORE_FILE` to use a different local keystore path. Never commit the
keystore or credentials.

This builds the same non-debuggable **release variant** as the release
workflow, signed with the specified key, at
`android-tv/app/build/outputs/apk/release/app-release.apk`. To update an
existing installation, use the same keystore and key as the installed APK,
including APKs from earlier releases built with the debug variant. Android
does not allow installing a lower version code over a newer version.

## Tests

```sh
docker compose --profile android run --rm android-build :app:testDebugUnitTest :app:testReleaseUnitTest :app:lintDebug :app:lintRelease
# Optional headless TV emulator (Linux x86_64 with /dev/kvm):
docker compose --profile android-test build android-build android-emulator
docker compose --profile android-test run --rm android-test
docker compose --profile android-test stop android-emulator
```

`android-test` starts the emulator and waits for Android to boot. No ADB ports are
published; the profiles do not affect a normal `docker compose up`. Existing
volumes from earlier standalone builds are reused (Compose may warn that it did
not create them). Do not run builds and instrumented tests concurrently on the same checkout.

Instrumented tests use fixture data. Also check real-media playback, resume and
background/foreground transitions on the target device.

On the home screen, Continue watching and Next episodes share a row when all
their cards fit. Otherwise each section has its own horizontally scrollable row,
so the remote's up and down buttons navigate between them.

## Run on a TV

Enable ADB debugging on the device, then run from the repository root:

```sh
adb devices
adb -s DEVICE_SERIAL install -r android-tv/app/build/outputs/apk/debug/app-debug.apk
adb -s DEVICE_SERIAL shell am start -n fr.plys.tv/.MainActivity
```

Initialize the server and create an account through the web client first. Sign in
on the TV with the server's root URL. For the development API, use
`http://<LAN-IP>:8080` on a TV or `http://10.0.2.2:8080` on the Android emulator.
`localhost` refers to the device itself. Use HTTP only on a trusted local network;
HTTPS certificates must be trusted by Android.

## Updates

The app reads the latest GitHub release at launch and then once a day while it
runs (with the release's ETag, so an unchanged release costs nothing). A newer
version shows an entry above "Sign out" in the side rail: selecting it downloads
the APK and opens the system installer. Android asks once to allow Plys to
install unknown apps.

The side rail uses 40 dp buttons with 20 dp icons, occupying 52 dp when folded
and 228 dp when expanded. On shorter displays it scrolls with remote focus
instead of shrinking the last buttons, including when update instructions appear.
Icons stay anchored during expansion and collapse; rail focus uses colour and
a border without scaling, unlike other buttons.

Android only accepts an update signed with the installed app's key. Release APKs
are signed with the keystore pinned in the `release` environment of the
repository (`ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`,
`ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD`). Ordinary local debug builds keep
their own debug key; use the signed APK command above with the matching
keystore and credentials to build an update-compatible APK. An app installed
from a build signed with another key must be uninstalled and reinstalled by
hand once.

## Code Layout

- `app/src/main/java/fr/plys/tv/data/`: API client, models, sessions and pagination.
- `app/src/main/java/fr/plys/tv/ui/`: screens, view models, theme and playback.
- `app/src/main/res/`: translations, fonts and Android resources.
- `app/src/test/` and `app/src/androidTest/`: unit and instrumented tests.
