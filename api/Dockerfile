# syntax=docker/dockerfile:1
ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-trixie AS dev
RUN go install github.com/air-verse/air@v1.61.7
# Named volume inherits these permissions so any host UID can write the Go caches.
RUN mkdir -p /go-cache && chmod 777 /go-cache
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
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/server /usr/local/bin/server
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
