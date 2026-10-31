# syntax=docker/dockerfile:1
ARG GO_VERSION=1.26

# GPU drivers: Intel iHD (VAAPI) and oneVPL runtime (QSV) are amd64-only and live in non-free; Mesa covers AMD.
ARG MEDIA_PACKAGES="ffmpeg vainfo mesa-va-drivers"
ARG INTEL_PACKAGES="intel-media-va-driver-non-free libvpl2 libmfx-gen1.2"

FROM golang:${GO_VERSION}-trixie AS dev
ARG MEDIA_PACKAGES
ARG INTEL_PACKAGES
RUN sed -i 's/^Components: main$/Components: main non-free non-free-firmware/' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends ${MEDIA_PACKAGES} \
        $(if [ "$(dpkg --print-architecture)" = amd64 ]; then echo ${INTEL_PACKAGES}; fi) \
    && rm -rf /var/lib/apt/lists/*
RUN go install github.com/air-verse/air@v1.61.7
# Named volumes inherit these permissions so any host UID can write the Go caches and the image cache.
RUN mkdir -p /go-cache /cache && chmod 777 /go-cache /cache
ENV GOPATH=/go-cache/gopath \
    GOCACHE=/go-cache/build \
    GOFLAGS=-buildvcs=false \
    HOME=/tmp
WORKDIR /src
EXPOSE 8080

FROM golang:${GO_VERSION}-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM debian:trixie-slim AS prod
ARG MEDIA_PACKAGES
ARG INTEL_PACKAGES
RUN sed -i 's/^Components: main$/Components: main non-free non-free-firmware/' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ${MEDIA_PACKAGES} \
        $(if [ "$(dpkg --print-architecture)" = amd64 ]; then echo ${INTEL_PACKAGES}; fi) \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/server /usr/local/bin/server
RUN mkdir -p /cache && chown 10001:10001 /cache
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
