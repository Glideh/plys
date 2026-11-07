FROM eclipse-temurin:17-jdk-jammy AS jdk

FROM ghcr.io/cirruslabs/android-sdk:35@sha256:c724009e305b4607157287624033ab97f319af44c244bfc9f73b6293f3bb01b9 AS build
ARG GRADLE_VERSION=8.11.1
ARG GRADLE_SHA256=f397b287023acdba1e9f6fc5ea72d22dd63669d59ed4a289a29b1a76eee151c6
COPY --from=jdk /opt/java/openjdk /opt/java/openjdk-17
ENV JAVA_HOME=/opt/java/openjdk-17
ENV PATH="/opt/gradle/bin:${JAVA_HOME}/bin:${PATH}" \
    GRADLE_USER_HOME=/root/.gradle \
    ANDROID_USER_HOME=/root/.android \
    JAVA_TOOL_OPTIONS=-Duser.home=/root
RUN curl --fail --location --silent --show-error \
        "https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip" -o /tmp/gradle.zip \
    && echo "${GRADLE_SHA256}  /tmp/gradle.zip" | sha256sum --check \
    && unzip -q /tmp/gradle.zip -d /opt \
    && ln -s "/opt/gradle-${GRADLE_VERSION}" /opt/gradle \
    && rm /tmp/gradle.zip \
    && sdkmanager "platforms;android-35" "build-tools;35.0.0" \
    && chmod 755 /root
COPY --chmod=755 entrypoint.sh /usr/local/bin/android-tools
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/android-tools"]
CMD [":app:assembleDebug"]

FROM build AS emulator
# Pinned: emulator 37.x hangs its vCPUs before the guest kernel starts on some KVM hosts
# (Linux 7.0, Haswell-E). sdkmanager only offers the latest build, so the archive replaces it; its
# package.xml is kept, with the pinned revision, because avdmanager refuses to create an AVD without it.
ARG EMULATOR_BUILD=14214601
ARG EMULATOR_REVISION=36.2.12
ARG EMULATOR_SHA256=55e7ce272dd27413855c81b4629c107a1553d07a2f77fa55a6049fdca22f4221
ENV ANDROID_SYSTEM_IMAGE="system-images;android-34;android-tv;x86" \
    ANDROID_AVD=plys-tv
RUN sdkmanager "emulator" "${ANDROID_SYSTEM_IMAGE}" \
    && curl --fail --location --silent --show-error \
        "https://dl.google.com/android/repository/emulator-linux_x64-${EMULATOR_BUILD}.zip" -o /tmp/emulator.zip \
    && echo "${EMULATOR_SHA256}  /tmp/emulator.zip" | sha256sum --check \
    && mv "${ANDROID_HOME}/emulator/package.xml" /tmp/package.xml \
    && rm -rf "${ANDROID_HOME}/emulator" \
    && unzip -q /tmp/emulator.zip -d "${ANDROID_HOME}" \
    && major="$(echo "${EMULATOR_REVISION}" | cut -d. -f1)" \
    && minor="$(echo "${EMULATOR_REVISION}" | cut -d. -f2)" \
    && micro="$(echo "${EMULATOR_REVISION}" | cut -d. -f3)" \
    && sed -e "s|<major>[0-9]*</major>|<major>${major}</major>|" \
        -e "s|<minor>[0-9]*</minor>|<minor>${minor}</minor>|" \
        -e "s|<micro>[0-9]*</micro>|<micro>${micro}</micro>|" \
        /tmp/package.xml > "${ANDROID_HOME}/emulator/package.xml" \
    && rm /tmp/emulator.zip /tmp/package.xml \
    && "${ANDROID_HOME}/emulator/emulator" -version | head -1 \
    && grep -o '<revision>.*</revision>' "${ANDROID_HOME}/emulator/package.xml"
COPY --chmod=755 emulator.sh /usr/local/bin/android-emulator
CMD ["emulator"]