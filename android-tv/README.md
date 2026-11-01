# Plys TV

Native Android TV / Google TV MVP, written in Kotlin with Jetpack Compose for TV
and Media3/ExoPlayer. Uses the existing [Plys API](../docs/openapi.yaml); no extra
server service or API migration is needed.

## Included

- TV launcher entry, landscape interface, D-pad/OK/Back navigation, visible focus.
- Server address and username/password sign-in using an existing Plys account.
- Refresh-token rotation, session restoration, encrypted refresh-token storage
  using AES-GCM and Android Keystore. Passwords and access tokens are not persisted;
  application backups are disabled.
- Accessible libraries, paginated posters (40 items/page), film details, seasons,
  episodes, videos and multi-part films.
- HLS playback, resume, play from the beginning, pause, 30-second skips, stop and
  a manual next episode/part button when playback ends.
- Progress reporting every 10 seconds during playback, at pause/stop/background
  and at the end. The client accounts for the server's keyframe start offset.
- English and French strings, selected using the TV's system language.
- Explicit connection, authentication, storage and playback errors, with retry.

This MVP uses the first readable file version, the server's default audio track,
and no subtitles. Version/audio/subtitle selectors, search, automatic next
episode playback, home recommendations, pairing codes, HDR/4K and Google Play
publication are outside this version. Video targets SDR at a maximum height of
1080p; codecs reported by Android are copied when the server can copy them,
otherwise the existing server transcodes. Unsupported HDR is tone mapped.

## Build a sideloadable APK

Requirements:

- JDK 17 and **Gradle 8.11.1** installed and available in `PATH`.
- Android SDK, with `platforms;android-35` and `build-tools;35.0.0`; accept its
  licenses using `sdkmanager --licenses`.
- `ANDROID_HOME` pointing to the SDK, or an untracked `local.properties` in this
  directory containing `sdk.dir=/absolute/path/to/Android/Sdk`.

The project uses Android Gradle Plugin 8.9.2 and Kotlin 2.1.20. Open this directory
in a compatible Android Studio (Meerkat or later) or build from the repository root:

```sh
gradle -p android-tv :app:assembleDebug :app:testDebugUnitTest :app:lintDebug
```

The debug APK is generated at:

```text
android-tv/app/build/outputs/apk/debug/app-debug.apk
```

It is automatically debug-signed and can be installed on Android TV **8.0
(API 26) or newer**. It is not a production release signing key.
For repeatable updates, build with the same local debug keystore; APKs signed
with a different key require uninstalling the previous app (which clears its session).

This checkout does not include a Gradle wrapper binary. After installing the
specified Gradle version, you can generate the standard wrapper with:

```sh
cd android-tv
gradle wrapper --gradle-version 8.11.1 --distribution-type bin
./gradlew :app:assembleDebug :app:testDebugUnitTest :app:lintDebug
```

## Install and connect

Enable developer options and ADB debugging on the TV. Use the TV's supported
USB/network debugging procedure (network debugging may require `adb pair` on
newer devices). Once connected:

```sh
adb devices
adb -s DEVICE_SERIAL install -r android-tv/app/build/outputs/apk/debug/app-debug.apk
adb -s DEVICE_SERIAL shell am start -n fr.plys.tv/.MainActivity
```

Alternatively, transfer the APK and install it using a TV file manager with
permission to install unknown applications. Open **Plys TV** in the TV launcher.

1. Initialize Plys, create your account, configure libraries and finish scanning
   using the web client first. Administration is not provided on the TV.
2. Enter the server's root address, for example `http://192.168.1.20:8096` or
   `https://plys.example.net`. A trailing `/api/v1` is also accepted. Reverse
   proxies must expose `/api/v1` at the origin root; subpath hosting is not supported.
3. Sign in, select a library, then a film or series/season/episode.
4. Select **Play** or **Resume**. In playback, OK/arrows reveal the controls;
   Back first hides active controls, then leaves the player. Pause/error/end
   screens can be exited directly with Back.

Use the server's LAN address, **not `localhost`**, which would mean the TV itself.
For an emulator reaching a server on its host, use `http://10.0.2.2:8096`.
HTTPS uses normal Android certificate validation; self-signed certificates are
not bypassed. HTTP is allowed for compatibility with Plys's default local
installation, but credentials and video traffic are unencrypted: use HTTP only
on a trusted home network, and prefer HTTPS.

Leaving playback or putting the app in the background reports progress and
deletes the stream session. Returning to the foreground creates a new stream at
the last position. If the network is unavailable, a visible error is shown;
server-side idle expiry eventually removes a session that could not be deleted.
Android may terminate the process without a final callback, so the last successful
periodic report remains the resume point.

## Validation

Unit tests cover URL validation, body-based login, first-run setup, serialized
refresh rotation, revocation, storage/network failures, pagination, playback
requests, progress reporting and source/HLS time conversion.

**Implementation status:** Kotlin compilation, unit tests, Android lint and APK
generation have not been run in the implementation environment: JDK/Gradle/SDK
were absent and their installation was denied. XML/resources and the diff were
checked statically. A build and a real TV/emulator playback check are still needed
before considering this MVP validated or distributing an APK.

After a successful build, check at least:

- Login, relaunch/session restoration, invalid credentials, unavailable server,
  session expiry and sign-out.
- D-pad focus, page changes beyond 40 items, Back navigation, empty and restricted
  libraries, series/seasons/episodes and multi-part films.
- A native-compatible file and a transcoded file; pause, skips beyond currently
  generated segments, stop, Home/background and foreground return.
- Resume in both the TV and web clients, watched state after completion and manual
  next episode playback. Check cleanup in server logs.
- French/English, a missing poster and playback errors.
