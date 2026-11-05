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
ENV ANDROID_SYSTEM_IMAGE="system-images;android-34;android-tv;x86" \
    ANDROID_AVD=plys-tv
RUN sdkmanager "emulator" "${ANDROID_SYSTEM_IMAGE}"
COPY --chmod=755 emulator.sh /usr/local/bin/android-emulator
CMD ["emulator"]