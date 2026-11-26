# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
    -ldflags="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$BUILD_DATE" \
    -o /cp450-exporter .
RUN printf 'version=%s\ncommit=%s\nbuild_date=%s\n' \
    "$VERSION" "$COMMIT" "$BUILD_DATE" > /version.txt

FROM alpine:latest

RUN apk --no-cache add ca-certificates \
    && addgroup -S exporter \
    && adduser -S -G exporter exporter

COPY --from=builder /cp450-exporter /usr/local/bin/cp450-exporter
COPY --from=builder /version.txt /version.txt

USER exporter
EXPOSE 9817

ENTRYPOINT ["/usr/local/bin/cp450-exporter"]
