# Plys TV

Android TV / Google TV client built with Kotlin, Compose for TV, Paging 3 and
Media3. Requires Android 8.0 (API 26) or newer and a running [Plys server](../README.md).
The API contract is documented in [OpenAPI](../docs/openapi.yaml).

## Build

- JDK 17 and Gradle 8.11.1 available in `PATH` (no Gradle wrapper is included).
- Android SDK platform 35 and build-tools 35.0.0, with licenses accepted.
- Set `ANDROID_HOME`, or `sdk.dir` in an untracked `android-tv/local.properties`.

Open this directory in Android Studio, or run from the repository root:

```sh
gradle -p android-tv :app:assembleDebug
```

Output: `android-tv/app/build/outputs/apk/debug/app-debug.apk`.

The APK is debug-signed. Keep the same debug keystore between builds to update an
existing installation without uninstalling it and losing its session.

## Tests

```sh
gradle -p android-tv :app:testDebugUnitTest :app:lintDebug
# Requires a connected TV or emulator:
gradle -p android-tv :app:connectedDebugAndroidTest
```

Instrumented tests use fixture data. Also verify D-pad navigation, real-media
playback, resume and background/foreground transitions on the target device.

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

## Code Layout

- `app/src/main/java/fr/plys/tv/data/`: API client, models, sessions and pagination.
- `app/src/main/java/fr/plys/tv/ui/`: screens, view models, theme and playback.
- `app/src/main/res/`: translations, fonts and Android resources.
- `app/src/test/` and `app/src/androidTest/`: unit and instrumented tests.
